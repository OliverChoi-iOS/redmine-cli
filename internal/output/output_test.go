package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteJSONFailure(t *testing.T) {
	env := Failure("validation_failed", "Bad input", "Try again", map[string]any{"field": "subject"})

	var buf bytes.Buffer
	if err := WriteJSON(&buf, env, false); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	var decoded Envelope
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output was not JSON: %v", err)
	}
	if decoded.OK {
		t.Fatalf("decoded.OK = true, want false")
	}
	if decoded.Error == nil || decoded.Error.Code != "validation_failed" {
		t.Fatalf("decoded error = %#v", decoded.Error)
	}
}

func TestExitCodeForError(t *testing.T) {
	tests := map[string]int{
		"auth_missing":      ExitConfigAuth,
		"network_error":     ExitNetwork,
		"not_found":         ExitNotFound,
		"permission_denied": ExitPermission,
		"workflow_rejected": ExitConflict,
		"unknown_name":      ExitUsage,
		"internal_error":    ExitGeneral,
	}

	for code, want := range tests {
		if got := ExitCodeForError(code); got != want {
			t.Fatalf("ExitCodeForError(%q) = %d, want %d", code, got, want)
		}
	}
}
