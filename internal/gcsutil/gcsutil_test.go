package gcsutil

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestCollectFilePaths_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()

	// Intentionally create in an order that shouldn't matter.
	mustWrite(t, filepath.Join(dir, "b.txt"), "b")
	mustWrite(t, filepath.Join(dir, "a.txt"), "a")
	mustWrite(t, filepath.Join(dir, "sub", "z.txt"), "z")
	mustWrite(t, filepath.Join(dir, "sub", "m.txt"), "m")

	got1, err := collectFilePaths(dir)
	if err != nil {
		t.Fatalf("collectFilePaths #1: %v", err)
	}
	got2, err := collectFilePaths(dir)
	if err != nil {
		t.Fatalf("collectFilePaths #2: %v", err)
	}
	if !reflect.DeepEqual(got1, got2) {
		t.Fatalf("order not stable\n#1=%v\n#2=%v", got1, got2)
	}

	rels := make([]string, 0, len(got1))
	for _, p := range got1 {
		r, err := filepath.Rel(dir, p)
		if err != nil {
			t.Fatalf("Rel: %v", err)
		}
		rels = append(rels, filepath.ToSlash(r))
	}

	want := []string{
		"a.txt",
		"b.txt",
		"sub/m.txt",
		"sub/z.txt",
	}
	if !reflect.DeepEqual(rels, want) {
		t.Fatalf("unexpected order\n got=%v\nwant=%v", rels, want)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestObjectExists(t *testing.T) {
	old := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = old }()

	// Ensure deterministic retry behavior in this unit test.
	os.Setenv("GCS_RETRIES", "1")
	defer os.Unsetenv("GCS_RETRIES")

	http.DefaultClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}
		if got := req.Header.Get("Authorization"); got != "Bearer tok" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("missing auth")),
				Header:     make(http.Header),
			}, nil
		}

		u := req.URL.String()
		switch {
		case strings.Contains(u, "/o/exist"):
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     make(http.Header),
			}, nil
		case strings.Contains(u, "/o/nope"):
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
				Header:     make(http.Header),
			}, nil
		default:
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader("bad request")),
				Header:     make(http.Header),
			}, nil
		}
	})

	ctx := context.Background()

	ok, err := ObjectExists(ctx, "tok", "bucket", "exist")
	if err != nil {
		t.Fatalf("ObjectExists exist: %v", err)
	}
	if !ok {
		t.Fatalf("expected exist=true")
	}

	ok, err = ObjectExists(ctx, "tok", "bucket", "nope")
	if err != nil {
		t.Fatalf("ObjectExists nope: %v", err)
	}
	if ok {
		t.Fatalf("expected exist=false")
	}
}
