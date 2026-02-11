package event

import (
	"net/http"

	contract "github.com/nicholaskarlson/proof-first-event-contracts/contract"
)

type ObjectRef struct {
	Bucket string
	Name   string
	Type   string
}

// ParseObjectRef parses an Eventarc-delivered event and returns the object ref.
// We keep the existing server-level guards (finalize-only + bucket check) unchanged.
// This function delegates parsing + URL-unescaping to proof-first-event-contracts.
func ParseObjectRef(r *http.Request, body []byte) ObjectRef {
	dec, obj, errText := contract.ParseEventarcAndDecide(r.Header.Get("Ce-Type"), body, "")
	if errText != nil {
		return ObjectRef{}
	}

	name := obj.NameUnescaped
	if name == "" {
		name = obj.Name
	}

	return ObjectRef{
		Bucket: obj.Bucket,
		Name:   name,
		Type:   dec.EventType,
	}
}
