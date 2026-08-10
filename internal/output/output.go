package output

import (
	"encoding/json"
	"io"
)

const (
	ExitOK         = 0
	ExitGeneral    = 1
	ExitUsage      = 2
	ExitConfigAuth = 3
	ExitNetwork    = 4
	ExitNotFound   = 5
	ExitPermission = 6
	ExitConflict   = 7
)

type Envelope struct {
	OK         bool           `json:"ok"`
	Data       any            `json:"data,omitempty"`
	Error      *Error         `json:"error,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
	Pagination *Pagination    `json:"pagination,omitempty"`
	Warnings   []string       `json:"warnings,omitempty"`
	DryRun     *bool          `json:"dry_run,omitempty"`
}

type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Hint    string         `json:"hint,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

type Pagination struct {
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	TotalCount int    `json:"total_count,omitempty"`
	NextOffset int    `json:"next_offset,omitempty"`
	HasMore    bool   `json:"has_more,omitempty"`
	Returned   int    `json:"returned,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	Hint       string `json:"hint,omitempty"`
}

func Success(data any, meta map[string]any) Envelope {
	return Envelope{OK: true, Data: data, Meta: meta}
}

func Failure(code, message, hint string, details map[string]any) Envelope {
	return Envelope{
		OK: false,
		Error: &Error{
			Code:    code,
			Message: message,
			Hint:    hint,
			Details: details,
		},
	}
}

func WriteJSON(w io.Writer, env Envelope, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(env)
}

func ExitCodeForError(code string) int {
	switch code {
	case "config_missing", "auth_missing", "auth_failed", "cache_error":
		return ExitConfigAuth
	case "network_error", "timeout", "redmine_error", "rate_limited":
		return ExitNetwork
	case "not_found":
		return ExitNotFound
	case "permission_denied":
		return ExitPermission
	case "conflict", "workflow_rejected":
		return ExitConflict
	case "validation_failed", "ambiguous_name", "unknown_name", "unsupported_filter", "input_parse_error", "file_not_found":
		return ExitUsage
	default:
		return ExitGeneral
	}
}
