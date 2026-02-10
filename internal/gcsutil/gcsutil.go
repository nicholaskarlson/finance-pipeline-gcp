package gcsutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const metadataTokenURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

type tokenResp struct {
	AccessToken string `json:"access_token"`
	// ExpiresIn   int    `json:"expires_in"` // not needed for MVP
	// TokenType   string `json:"token_type"`
}

// AccessToken returns a bearer token for calling Google APIs.
// Priority:
//  1. env GCP_ACCESS_TOKEN (for local testing)
//  2. Cloud Run / GCE metadata server token
func AccessToken(ctx context.Context) (string, error) {
	if v := strings.TrimSpace(os.Getenv("GCP_ACCESS_TOKEN")); v != "" {
		return v, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataTokenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("metadata token request failed: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("metadata token status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var tr tokenResp
	if err := json.Unmarshal(b, &tr); err != nil {
		return "", fmt.Errorf("metadata token parse: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("metadata token missing access_token")
	}
	return tr.AccessToken, nil
}

func DownloadToFile(ctx context.Context, token, bucket, object, dst string) error {
	u := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o/%s?alt=media",
		url.PathEscape(bucket),
		url.PathEscape(object),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
		return fmt.Errorf("gcs download status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("download copy: %w", err)
	}
	return nil
}

func UploadFile(ctx context.Context, token, bucket, object, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	u := fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		url.PathEscape(bucket),
		url.QueryEscape(object),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, f)
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
		return fmt.Errorf("gcs upload status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	return nil
}

func UploadDir(ctx context.Context, token, bucket, prefix, dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		obj := filepath.ToSlash(filepath.Join(prefix, rel))
		return UploadFile(ctx, token, bucket, obj, path)
	})
}
