package gcsutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const metadataTokenURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

type tokenResp struct {
	AccessToken string `json:"access_token"`
	// ExpiresIn   int    `json:"expires_in"` // not needed for MVP
	// TokenType   string `json:"token_type"`
}

// ---- Tunables (env) ----
//
// All are optional.
//
// Retries
//   GCS_RETRIES: total attempts (default 3)
//
// Timeouts (per-attempt)
//   GCS_TOKEN_TIMEOUT:    metadata token request timeout (default 10s)
//   GCS_DOWNLOAD_TIMEOUT: object download timeout (default 60s)
//   GCS_UPLOAD_TIMEOUT:   object upload timeout (default 60s)
//
// Backoff
//   GCS_RETRY_BACKOFF:     initial backoff between attempts (default 200ms)
//   GCS_RETRY_MAX_BACKOFF: maximum backoff (default 2s)

func envInt(name string, def int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil || i < 0 {
		return def
	}
	return i
}

func envDuration(name string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func retries() int { return envInt("GCS_RETRIES", 3) }

func tokenTimeout() time.Duration { return envDuration("GCS_TOKEN_TIMEOUT", 10*time.Second) }

func downloadTimeout() time.Duration { return envDuration("GCS_DOWNLOAD_TIMEOUT", 60*time.Second) }

func uploadTimeout() time.Duration { return envDuration("GCS_UPLOAD_TIMEOUT", 60*time.Second) }

func retryBackoff() time.Duration { return envDuration("GCS_RETRY_BACKOFF", 200*time.Millisecond) }

func retryMaxBackoff() time.Duration { return envDuration("GCS_RETRY_MAX_BACKOFF", 2*time.Second) }

func shouldRetryStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	if code >= 500 && code <= 599 {
		return true
	}
	return false
}

func isRetryableNetErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		if ne.Timeout() {
			return true
		}
		if ne.Temporary() {
			return true
		}
	}
	return false
}

func sleepOrDone(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type retryableStatusError struct {
	status int
	body   string
}

func (e retryableStatusError) Error() string {
	if e.body == "" {
		return fmt.Sprintf("status=%d", e.status)
	}
	return fmt.Sprintf("status=%d body=%s", e.status, e.body)
}

func doWithRetry(ctx context.Context, attempts int, backoff, maxBackoff time.Duration, fn func(context.Context) error) error {
	if attempts <= 0 {
		attempts = 1
	}
	if backoff <= 0 {
		backoff = 200 * time.Millisecond
	}
	if maxBackoff <= 0 {
		maxBackoff = 2 * time.Second
	}

	var last error
	b := backoff
	for i := 0; i < attempts; i++ {
		if err := fn(ctx); err != nil {
			// Never retry on context cancellation/deadline.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			last = err

			var se retryableStatusError
			if errors.As(err, &se) {
				if !shouldRetryStatus(se.status) || i == attempts-1 {
					return err
				}
			} else if !isRetryableNetErr(err) || i == attempts-1 {
				return err
			}

			if err := sleepOrDone(ctx, b); err != nil {
				return err
			}
			b *= 2
			if b > maxBackoff {
				b = maxBackoff
			}
			continue
		}
		return nil
	}
	if last == nil {
		last = fmt.Errorf("retry loop exhausted")
	}
	return last
}

// AccessToken returns a bearer token for calling Google APIs.
// Priority:
//  1. env GCP_ACCESS_TOKEN (for local testing)
//  2. Cloud Run / GCE metadata server token
func AccessToken(ctx context.Context) (string, error) {
	if v := strings.TrimSpace(os.Getenv("GCP_ACCESS_TOKEN")); v != "" {
		return v, nil
	}

	attempts := retries()
	to := tokenTimeout()

	var out string
	err := doWithRetry(ctx, attempts, retryBackoff(), retryMaxBackoff(), func(parent context.Context) error {
		cctx, cancel := context.WithTimeout(parent, to)
		defer cancel()

		req, err := http.NewRequestWithContext(cctx, http.MethodGet, metadataTokenURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Metadata-Flavor", "Google")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("metadata token request: %w", err)
		}
		defer resp.Body.Close()

		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if resp.StatusCode/100 != 2 {
			return retryableStatusError{status: resp.StatusCode, body: strings.TrimSpace(string(b))}
		}

		var tr tokenResp
		if err := json.Unmarshal(b, &tr); err != nil {
			return fmt.Errorf("metadata token parse: %w", err)
		}
		if tr.AccessToken == "" {
			return fmt.Errorf("metadata token missing access_token")
		}
		out = tr.AccessToken
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("metadata token failed after %d attempt(s): %w", attempts, err)
	}
	return out, nil
}

func ObjectExists(ctx context.Context, token, bucket, object string) (bool, error) {
	u := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o/%s",
		url.PathEscape(bucket),
		url.PathEscape(object),
	)

	attempts := retries()
	to := downloadTimeout()

	exists := false
	err := doWithRetry(ctx, attempts, retryBackoff(), retryMaxBackoff(), func(parent context.Context) error {
		cctx, cancel := context.WithTimeout(parent, to)
		defer cancel()

		req, err := http.NewRequestWithContext(cctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Not found is a clean, non-retryable answer.
		if resp.StatusCode == http.StatusNotFound {
			exists = false
			return nil
		}

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
			// Retry transient failures.
			return retryableStatusError{status: resp.StatusCode, body: strings.TrimSpace(string(b))}
		}

		exists = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return exists, nil
}

func DownloadToFile(ctx context.Context, token, bucket, object, dst string) error {
	u := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o/%s?alt=media",
		url.PathEscape(bucket),
		url.PathEscape(object),
	)

	attempts := retries()
	to := downloadTimeout()

	return doWithRetry(ctx, attempts, retryBackoff(), retryMaxBackoff(), func(parent context.Context) error {
		cctx, cancel := context.WithTimeout(parent, to)
		defer cancel()

		req, err := http.NewRequestWithContext(cctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("gcs download request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			body := strings.TrimSpace(string(b))
			if shouldRetryStatus(resp.StatusCode) {
				return retryableStatusError{status: resp.StatusCode, body: body}
			}
			return fmt.Errorf("gcs download status=%d body=%s", resp.StatusCode, body)
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}

		// Write to temp + rename (atomic) to avoid partial files on retry / crash.
		tmp := dst + ".tmp"
		f, err := os.Create(tmp)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, resp.Body)
		closeErr := f.Close()
		if copyErr != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("download copy: %w", copyErr)
		}
		if closeErr != nil {
			_ = os.Remove(tmp)
			return closeErr
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return nil
	})
}

func UploadFile(ctx context.Context, token, bucket, object, src string) error {
	u := fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		url.PathEscape(bucket),
		url.QueryEscape(object),
	)

	attempts := retries()
	to := uploadTimeout()

	return doWithRetry(ctx, attempts, retryBackoff(), retryMaxBackoff(), func(parent context.Context) error {
		cctx, cancel := context.WithTimeout(parent, to)
		defer cancel()

		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()

		req, err := http.NewRequestWithContext(cctx, http.MethodPost, u, f)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("gcs upload request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			body := strings.TrimSpace(string(b))
			if shouldRetryStatus(resp.StatusCode) {
				return retryableStatusError{status: resp.StatusCode, body: body}
			}
			return fmt.Errorf("gcs upload status=%d body=%s", resp.StatusCode, body)
		}

		return nil
	})
}

// collectFilePaths returns absolute file paths under dir in a deterministic order (sorted by rel path).
func collectFilePaths(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		ri, _ := filepath.Rel(dir, files[i])
		rj, _ := filepath.Rel(dir, files[j])
		return filepath.ToSlash(ri) < filepath.ToSlash(rj)
	})
	return files, nil
}

func UploadDir(ctx context.Context, token, bucket, prefix, dir string) error {
	files, err := collectFilePaths(dir)
	if err != nil {
		return err
	}
	for _, p := range files {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		obj := filepath.ToSlash(filepath.Join(prefix, rel))
		if err := UploadFile(ctx, token, bucket, obj, p); err != nil {
			return err
		}
	}
	return nil
}
