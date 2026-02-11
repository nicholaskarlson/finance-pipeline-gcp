package server

import "testing"

func TestParseRunID(t *testing.T) {
	tests := []struct {
		name       string
		objectName string
		prefix     string
		wantID     string
		wantOK     bool
	}{
		{
			name:       "ok",
			objectName: "in/demo/right.csv",
			prefix:     "in/",
			wantID:     "demo",
			wantOK:     true,
		},
		{
			name:       "ok prefix without slash",
			objectName: "in/demo/right.csv",
			prefix:     "in",
			wantID:     "demo",
			wantOK:     true,
		},
		{
			name:       "reject wrong filename",
			objectName: "in/demo/left.csv",
			prefix:     "in/",
			wantID:     "",
			wantOK:     false,
		},
		{
			name:       "reject nested path",
			objectName: "in/demo/sub/right.csv",
			prefix:     "in/",
			wantID:     "",
			wantOK:     false,
		},
		{
			name:       "reject traversal run id",
			objectName: "in/../right.csv",
			prefix:     "in/",
			wantID:     "",
			wantOK:     false,
		},
		{
			name:       "reject dot run id",
			objectName: "in/./right.csv",
			prefix:     "in/",
			wantID:     "",
			wantOK:     false,
		},
		{
			name:       "reject leading dash",
			objectName: "in/-demo/right.csv",
			prefix:     "in/",
			wantID:     "",
			wantOK:     false,
		},
		{
			name:       "reject other prefix",
			objectName: "other/demo/right.csv",
			prefix:     "in/",
			wantID:     "",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := parseRunID(tt.objectName, tt.prefix)
			if gotOK != tt.wantOK {
				t.Fatalf("ok=%v want %v (id=%q)", gotOK, tt.wantOK, gotID)
			}
			if gotID != tt.wantID {
				t.Fatalf("id=%q want %q", gotID, tt.wantID)
			}
		})
	}
}
