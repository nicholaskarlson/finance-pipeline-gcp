package gcsutil

import (
	"os"
	"path/filepath"
	"reflect"
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
