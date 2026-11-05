package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
)

func timeCreateArgs() []string {
	return []string{"time", "create", "--issue", "123", "--hours", "1.5", "--activity-id", "9", "--spent-on", "2026-11-05"}
}

func TestTimeCreatePostsDecimalHours(t *testing.T) {
	requests := 0
	args := append(timeCreateArgs(), "--comments", "버그 수정")
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/redmine/time_entries.json" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		if r.Header.Get("X-Redmine-API-Key") != "test-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing authentication or JSON content type")
		}
		var body map[string]map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		want := map[string]any{"issue_id": float64(123), "hours": 1.5, "activity_id": float64(9), "spent_on": "2026-11-05", "comments": "버그 수정"}
		if !reflect.DeepEqual(body["time_entry"], want) {
			t.Errorf("payload = %#v, want %#v", body, want)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"time_entry":{"id":77,"project":{"id":1,"name":"Mobile"},"issue":{"id":123},"user":{"id":4,"name":"Tester"},"activity":{"id":9,"name":"개발"},"hours":1.5,"comments":"버그 수정","spent_on":"2026-11-05","created_on":"2026-11-05T00:00:00Z"}}`)
	}, args...)
	if code != output.ExitOK || !env.OK || requests != 1 {
		t.Fatalf("code=%d env=%+v requests=%d", code, env, requests)
	}
	data := env.Data.(map[string]any)
	if data["id"] != float64(77) || data["hours"] != 1.5 || data["comments"] != "버그 수정" || data["spent_on"] != "2026-11-05" {
		t.Fatalf("data = %#v", data)
	}
}

func TestTimeCreateDryRunAndValidation(t *testing.T) {
	commentsFile := filepath.Join(t.TempDir(), "comments.txt")
	comments := strings.Repeat("가", 255)
	if err := os.WriteFile(commentsFile, []byte(comments), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		flags []string
		want  string
	}{
		{"dry run", nil, ""},
		{"comments file", []string{"--comments-file", commentsFile}, ""},
		{"quarter hour", []string{"--hours", "0.25"}, ""},
		{"missing hours", []string{"--hours="}, "validation_failed"},
		{"zero hours", []string{"--hours", "0"}, "validation_failed"},
		{"negative hours", []string{"--hours", "-1"}, "validation_failed"},
		{"NaN hours", []string{"--hours", "NaN"}, "validation_failed"},
		{"infinite hours", []string{"--hours", "+Inf"}, "validation_failed"},
		{"overflow hours", []string{"--hours", "1e999"}, "validation_failed"},
		{"duration string", []string{"--hours", "1:30"}, "validation_failed"},
		{"missing issue", []string{"--issue="}, "validation_failed"},
		{"zero issue", []string{"--issue", "0"}, "validation_failed"},
		{"invalid issue", []string{"--issue", "abc"}, "validation_failed"},
		{"negative activity", []string{"--activity-id", "-1"}, "validation_failed"},
		{"missing activity", []string{"--activity-id="}, "validation_failed"},
		{"missing work date", []string{"--spent-on="}, "validation_failed"},
		{"invalid date", []string{"--spent-on", "2026-02-30"}, "validation_failed"},
		{"invalid format", []string{"--spent-on", "2026/11/05"}, "validation_failed"},
		{"long comments", []string{"--comments", strings.Repeat("가", 256)}, "validation_failed"},
		{"conflicting comments", []string{"--comments", "text", "--comments-file", commentsFile}, "validation_failed"},
		{"missing comments file", []string{"--comments-file", commentsFile + ".missing"}, "file_not_found"},
		{"extra argument", []string{"unexpected"}, "validation_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(timeCreateArgs(), "--dry-run")
			args = append(args, tc.flags...)
			code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("unexpected network request")
				w.WriteHeader(500)
			}, args...)
			if tc.want != "" {
				if code != output.ExitUsage || env.Error == nil || env.Error.Code != tc.want {
					t.Fatalf("code=%d env=%+v", code, env)
				}
				return
			}
			if code != output.ExitOK || env.DryRun == nil || !*env.DryRun {
				t.Fatalf("code=%d env=%+v", code, env)
			}
			data := env.Data.(map[string]any)
			request := data["request"].(map[string]any)
			if data["operation"] != "time.create" || request["issue_id"] != float64(123) || request["activity_id"] != float64(9) || request["spent_on"] != "2026-11-05" {
				t.Fatalf("dry run = %#v", data)
			}
			if tc.name == "comments file" && request["comments"] != comments {
				t.Fatalf("comments = %#v", request["comments"])
			}
			if tc.name == "quarter hour" && request["hours"] != 0.25 {
				t.Fatalf("hours = %#v", request["hours"])
			}
		})
	}
}

func TestTimeCreateErrorsDoNotRetry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		want      string
		uncertain bool
	}{
		{"auth", 401, "", "auth_failed", false},
		{"permission", 403, "", "permission_denied", false},
		{"not found", 404, "", "not_found", false},
		{"server validation", 422, `{"errors":["Activity is invalid"]}`, "validation_failed", false},
		{"server error", 500, "", "redmine_error", true},
		{"missing ID", 201, `{"time_entry":{}}`, "redmine_error", true},
		{"empty response", 201, "", "redmine_error", true},
		{"invalid JSON", 201, "invalid", "redmine_error", true},
		{"lost response", 0, "", "network_error", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if tc.status == 0 {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
					return
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}, timeCreateArgs()...)
			if code == 0 || env.Error == nil || env.Error.Code != tc.want || requests != 1 {
				t.Fatalf("code=%d env=%+v requests=%d", code, env, requests)
			}
			warned := false
			for _, warning := range env.Warnings {
				if strings.Contains(warning, "duplicate entries") {
					warned = true
				}
			}
			if warned != tc.uncertain {
				t.Fatalf("warnings = %#v, uncertain = %t", env.Warnings, tc.uncertain)
			}
		})
	}
}

func TestTimeActivities(t *testing.T) {
	for _, body := range []string{`{"time_entry_activities":[{"id":9,"name":"개발","is_default":true}]}`, `{"time_entry_activities":[]}`} {
		code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/redmine/enumerations/time_entry_activities.json" {
				t.Errorf("request = %s %s", r.Method, r.URL)
			}
			_, _ = io.WriteString(w, body)
		}, "time", "activities")
		if code != output.ExitOK || !env.OK {
			t.Fatalf("code=%d env=%+v", code, env)
		}
		activities, ok := env.Data.(map[string]any)["activities"].([]any)
		if !ok {
			t.Fatalf("activities = %#v", env.Data)
		}
		if strings.Contains(body, "개발") && (len(activities) != 1 || activities[0].(map[string]any)["is_default"] != true) {
			t.Fatalf("activities = %#v", activities)
		}
	}
}

func TestTimeTableSanitizesServerText(t *testing.T) {
	for _, data := range []any{
		map[string]any{"activities": []redmine.TimeEntryActivity{{ID: 9, Name: "\x1b[31m개발\x1b[0m\ntext", IsDefault: true}}},
		redmine.TimeEntry{ID: 77, Issue: &redmine.Reference{ID: 123}, Hours: 1.5, SpentOn: "2026-11-05", Activity: redmine.Reference{Name: "개발"}, Comments: "\x1b[31mtext\x1b[0m\nnext"},
	} {
		var buf bytes.Buffer
		if err := writeTable(&buf, data); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(buf.String(), "\x1b") || !strings.Contains(buf.String(), "개발") || strings.Count(buf.String(), "\n") != 2 {
			t.Fatalf("table = %q", buf.String())
		}
	}
}
