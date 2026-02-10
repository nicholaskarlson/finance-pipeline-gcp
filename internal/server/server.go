package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/nicholaskarlson/finance-pipeline-gcp/internal/event"
	"github.com/nicholaskarlson/finance-pipeline-gcp/internal/gcsutil"
	"github.com/nicholaskarlson/finance-pipeline-gcp/internal/pipeline"
)

func Run() error {
	inPrefix := ensureSlash(getenv("INPUT_PREFIX", "in/"))
	outPrefix := ensureSlash(getenv("OUTPUT_PREFIX", "out/"))
	outBucket := os.Getenv("OUTPUT_BUCKET")
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

		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		ref := event.ParseObjectRef(r, body)
		if ref.Bucket == "" || ref.Name == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Trigger only on: in/<runID>/right.csv
		runID, ok := parseRunID(ref.Name, inPrefix)
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

		leftObj := path.Join(inPrefix, runID, "left.csv")
		rightObj := path.Join(inPrefix, runID, "right.csv")

		leftPath := filepathOS(tmp, "left.csv")
		rightPath := filepathOS(tmp, "right.csv")

		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Minute)
		defer cancel()

		token, err := gcsutil.AccessToken(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Download inputs from the *input bucket* (ref.Bucket)
		if err := gcsutil.DownloadToFile(ctx, token, ref.Bucket, leftObj, leftPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := gcsutil.DownloadToFile(ctx, token, ref.Bucket, rightObj, rightPath); err != nil {
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

		// Always upload results (pack exists even on recon failure)
		uploadPrefix := path.Join(outPrefix, runID)
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

func parseRunID(objectName, inputPrefix string) (string, bool) {
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
	if runID == "" {
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
