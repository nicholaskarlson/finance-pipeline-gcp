package server

import (
	"testing"

	contract "github.com/nicholaskarlson/proof-first-event-contracts/contract"
)

func TestEventContract_URLUnescapeFeedsRunID(t *testing.T) {
	body := []byte(`{"bucket":"inbucket","name":"in%2Fdemo%2Fright.csv"}`)

	dec, obj, errText := contract.ParseEventarcAndDecide(contract.TypeFinalized, body, "inbucket")
	if errText != nil {
		t.Fatalf("unexpected error: %q", *errText)
	}
	if !dec.ShouldRun {
		t.Fatalf("expected run, got: %+v", dec)
	}

	name := obj.NameUnescaped
	if name == "" {
		name = obj.Name
	}

	runID, ok := parseRunID(name, "in/")
	if !ok || runID != "demo" {
		t.Fatalf("runID=%q ok=%v want demo/true (name=%q)", runID, ok, name)
	}
}
