package event

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseObjectRef_DirectShape_UnescapesName(t *testing.T) {
	r := httptestRequest("google.cloud.storage.object.v1.finalized")
	body := []byte(`{"bucket":"b","name":"in%2Fdemo%2Fright.csv"}`)

	ref := ParseObjectRef(r, body)
	if ref.Bucket != "b" {
		t.Fatalf("bucket=%q, want %q", ref.Bucket, "b")
	}
	if ref.Name != "in/demo/right.csv" {
		t.Fatalf("name=%q, want %q", ref.Name, "in/demo/right.csv")
	}
	if ref.Type != "google.cloud.storage.object.v1.finalized" {
		t.Fatalf("type=%q, want finalized", ref.Type)
	}
}

func TestParseObjectRef_EnvelopeShape_UnescapesName(t *testing.T) {
	r := httptestRequest("google.cloud.storage.object.v1.finalized")
	body := []byte(`{"data":{"bucket":"b","name":"drop%2Fleft.csv"}}`)

	ref := ParseObjectRef(r, body)
	if ref.Bucket != "b" {
		t.Fatalf("bucket=%q, want %q", ref.Bucket, "b")
	}
	if ref.Name != "drop/left.csv" {
		t.Fatalf("name=%q, want %q", ref.Name, "drop/left.csv")
	}
}

func httptestRequest(ceType string) *http.Request {
	r, _ := http.NewRequest("POST", "http://example", strings.NewReader("{}"))
	r.Header.Set("Ce-Type", ceType)
	return r
}
