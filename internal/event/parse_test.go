package event

import (
	"net/http"
	"testing"
)

func TestParseObjectRef_PlainBody(t *testing.T) {
	r := &http.Request{Header: http.Header{"Ce-Type": []string{"t"}}}
	body := []byte(`{"bucket":"b1","name":"in/demo/right.csv"}`)
	ref := ParseObjectRef(r, body)
	if ref.Type != "t" {
		t.Fatalf("type: expected %q got %q", "t", ref.Type)
	}
	if ref.Bucket != "b1" {
		t.Fatalf("bucket: expected %q got %q", "b1", ref.Bucket)
	}
	if ref.Name != "in/demo/right.csv" {
		t.Fatalf("name: expected %q got %q", "in/demo/right.csv", ref.Name)
	}
}

func TestParseObjectRef_EnvelopeData_UnescapesName(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	body := []byte(`{"data":{"bucket":"b2","name":"in%2Fdemo%2Fright.csv"}}`)
	ref := ParseObjectRef(r, body)
	if ref.Bucket != "b2" {
		t.Fatalf("bucket: expected %q got %q", "b2", ref.Bucket)
	}
	if ref.Name != "in/demo/right.csv" {
		t.Fatalf("name: expected %q got %q", "in/demo/right.csv", ref.Name)
	}
}

func TestParseObjectRef_BadJSON_ReturnsTypeOnly(t *testing.T) {
	r := &http.Request{Header: http.Header{"Ce-Type": []string{"t"}}}
	ref := ParseObjectRef(r, []byte("not-json"))
	if ref.Type != "t" {
		t.Fatalf("type: expected %q got %q", "t", ref.Type)
	}
	if ref.Bucket != "" || ref.Name != "" {
		t.Fatalf("expected empty bucket/name, got bucket=%q name=%q", ref.Bucket, ref.Name)
	}
}
