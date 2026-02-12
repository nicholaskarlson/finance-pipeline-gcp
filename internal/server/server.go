package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/nicholaskarlson/finance-pipeline-gcp/internal/gcsutil"
	"github.com/nicholaskarlson/finance-pipeline-gcp/internal/pipeline"
	contract "github.com/nicholaskarlson/proof-first-event-contracts/contract"
)

const maxEventBodyBytes int64 = 1 << 20 // 1MiB

func Run() error {
	inPrefix := ensureSlash(getenv("INPUT_PREFIX", "in/"))
	outPrefix := ensureSlash(getenv("OUTPUT_PREFIX", "out/"))

	inBucket := strings.TrimSpace(os.Getenv("INPUT_BUCKET"))
	if inBucket == "" {
		return fmt.Errorf("INPUT_BUCKET is required")
	}

	outBucket := strings.TrimSpace(os.Getenv("OUTPUT_BUCKET"))
	if outBucket == "" {
		return fmt.Errorf("OUTPUT_BUCKET is required")
	}

	port := getenv("PORT", "8080")
	addr := ":" + port

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxEventBodyBytes)
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				http.Error(w, "event payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "read event body failed", http.StatusBadRequest)
			return
		}

		dec, obj, errText := contract.ParseEventarcAndDecide(r.Header.Get("Ce-Type"), body, inBucket)
		if errText != nil {
			// Malformed / unexpected events should not cause retries.
			fmt.Fprintf(os.Stdout, "event_contract_error: %s", *errText)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if !dec.ShouldRun {
			// Deterministic ignores (delete/archive/metadata updates, wrong bucket, etc.).
			fmt.Fprintf(os.Stdout, "event_contract_ignore: %s\n", dec.Reason)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		name := obj.NameUnescaped
		if name == "" {
			name = obj.Name
		}

		// Trigger only on: in/<runID>/right.csv
		runID, ok := parseRunID(name, inPrefix)
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Temp workspace
		tmp, err := os.MkdirTemp("", "finance-pipeline-*")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(tmp)

		leftObj := inPrefix + runID + "/left.csv"
		rightObj := inPrefix + runID + "/right.csv"

		leftPath := filepathOS(tmp, "left.csv")
		rightPath := filepathOS(tmp, "right.csv")

		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Minute)
		defer cancel()

		token, err := gcsutil.AccessToken(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Idempotency: if this run has already completed (success or error marker exists),
		// ACK the event and return without re-running work.
		markerPrefix := outPrefix + runID
		if ok, err := gcsutil.ObjectExists(ctx, token, outBucket, markerPrefix+"/_SUCCESS.json"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} else if ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if ok, err := gcsutil.ObjectExists(ctx, token, outBucket, markerPrefix+"/_ERROR.json"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} else if ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Download inputs from INPUT_BUCKET (not from the event payload).
		if err := gcsutil.DownloadToFile(ctx, token, inBucket, leftObj, leftPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := gcsutil.DownloadToFile(ctx, token, inBucket, rightObj, rightPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Run pipeline into temp output
		outBase := filepathOS(tmp, "out")
		res, runErr := pipeline.Run(ctx, pipeline.Config{
			LeftPath:     leftPath,
			RightPath:    rightPath,
			OutBase:      outBase,
			RunID:        runID,
			ReconBin:     "recon",
			AuditpackBin: "auditpack",
		})

		// Write a completion marker into the run directory so downstream consumers
		// can avoid reading partial outputs.
		if err := writeCompletionMarker(res.RunDir, runID, runErr); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Always upload results (pack exists even on recon failure)
		uploadPrefix := outPrefix + runID
		if err := gcsutil.UploadDir(ctx, token, outBucket, uploadPrefix, res.RunDir); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// IMPORTANT: for "bad data" we ACK 2xx to prevent retries.
		if runErr != nil {
			fmt.Fprintf(os.Stdout, "processed run_id=%s with error: %v\n", runID, runErr)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		fmt.Fprintf(os.Stdout, "processed run_id=%s ok\n", runID)
		w.WriteHeader(http.StatusNoContent)
	})

	fmt.Printf("listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func validRunID(runID string) bool {
	if runID == "" {
		return false
	}
	if len(runID) > 64 {
		return false
	}
	for i := 0; i < len(runID); i++ {
		c := runID[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		isAllowed := isAlphaNum || c == '-' || c == '_'
		if !isAllowed {
			return false
		}
		if i == 0 && !isAlphaNum {
			return false
		}
	}
	return true
}

// parseRunID extracts a run id from an object name like:
//
//	in/<run_id>/right.csv
//
// run_id is intentionally restrictive to prevent path traversal / prefix escape.
func parseRunID(objectName, inputPrefix string) (string, bool) {
	inputPrefix = ensureSlash(inputPrefix)
	if !strings.HasPrefix(objectName, inputPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(objectName, inputPrefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", false
	}
	runID := parts[0]
	if parts[1] != "right.csv" {
		return "", false
	}
	if !validRunID(runID) {
		return "", false
	}
	return runID, true
}

func ensureSlash(p string) string {
	if p == "" {
		return ""
	}
	if !strings.HasSuffix(p, "/") {
		return p + "/"
	}
	return p
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func filepathOS(dir, name string) string {
	// tiny helper to build OS-native file paths
	return strings.ReplaceAll(path.Join(dir, name), "/", string(os.PathSeparator))
}
