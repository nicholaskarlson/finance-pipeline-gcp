package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type completionMarker struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func writeCompletionMarker(runDir, runID string, runErr error) error {
	if strings.TrimSpace(runDir) == "" {
		// Nothing to write; treat as internal error so the event can be retried.
		return fmt.Errorf("missing run dir for completion marker")
	}

	status := "success"
	var errSummary string
	if runErr != nil {
		status = "error"
		s := runErr.Error()
		// Keep only the first line (avoids embedding volatile paths/output).
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[:i]
		}
		errSummary = strings.TrimSpace(s)
	}

	m := completionMarker{
		RunID:  runID,
		Status: status,
		Error:  errSummary,
	}

	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	name := "_SUCCESS.json"
	if runErr != nil {
		name = "_ERROR.json"
	}

	p := filepathOS(runDir, name)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
