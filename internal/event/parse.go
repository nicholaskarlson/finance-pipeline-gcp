package event

import (
	"encoding/json"
	"net/http"
)

type ObjectRef struct {
	Bucket string
	Name   string
	Type   string
}

type objectEvent struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

type envelope struct {
	Data objectEvent `json:"data"`
}

func ParseObjectRef(r *http.Request, body []byte) ObjectRef {
	ref := ObjectRef{Type: r.Header.Get("Ce-Type")}

	// Common: body is just { "bucket": "...", "name": "..." }
	var oe objectEvent
	if err := json.Unmarshal(body, &oe); err == nil && oe.Bucket != "" && oe.Name != "" {
		ref.Bucket, ref.Name = oe.Bucket, oe.Name
		return ref
	}

	// Sometimes: { "data": { "bucket": "...", "name": "..." } }
	var env envelope
	if err := json.Unmarshal(body, &env); err == nil && env.Data.Bucket != "" && env.Data.Name != "" {
		ref.Bucket, ref.Name = env.Data.Bucket, env.Data.Name
		return ref
	}

	return ref
}
