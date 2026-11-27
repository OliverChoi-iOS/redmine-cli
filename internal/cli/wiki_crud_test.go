package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"redmine-cli/internal/output"
)

func TestWikiCreateRejectsExistingPage(t *testing.T) {
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected write: %s", r.Method)
		}
		_, _ = io.WriteString(w, `{"wiki_page":{"title":"Guide","version":3}}`)
	}, "wiki", "create", "Guide", "--project", "mobile", "--text", "h1. Guide")
	if code != output.ExitConflict || env.Error == nil || env.Error.Code != "conflict" {
		t.Fatalf("code=%d env=%+v", code, env)
	}
}

func TestWikiCreateNewPage(t *testing.T) {
	gets, puts := 0, 0
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/redmine/projects/mobile/wiki/Guide.json" {
			t.Errorf("unexpected URL: %s", r.URL)
		}
		if r.Method == http.MethodGet {
			gets++
			if gets == 1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = io.WriteString(w, `{"wiki_page":{"title":"Guide","text":"h1. Guide","version":1,"parent":{"title":"Manual"}}}`)
			return
		}
		puts++
		var payload struct {
			Page map[string]any `json:"wiki_page"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if _, ok := payload.Page["version"]; ok || payload.Page["parent_title"] != "Manual" || payload.Page["text"] != "h1. Guide" {
			t.Errorf("payload = %#v", payload.Page)
		}
		w.WriteHeader(http.StatusCreated)
	}, "wiki", "create", "Guide", "--project", "mobile", "--text", "h1. Guide", "--parent-title", "Manual")
	data, _ := env.Data.(map[string]any)
	if code != 0 || !env.OK || gets != 2 || puts != 1 || data["created"] != true || data["after"] == nil {
		t.Fatalf("code=%d env=%+v gets=%d puts=%d", code, env, gets, puts)
	}
}

func TestWikiUpdateRequiresExistingPage(t *testing.T) {
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected write: %s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}, "wiki", "update", "Guide", "--project", "mobile", "--text", "new")
	if code != output.ExitNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("code=%d env=%+v", code, env)
	}
}

func TestWikiUpdateSendsCurrentVersion(t *testing.T) {
	puts := 0
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"wiki_page":{"title":"Guide","text":"old","version":6}}`)
			return
		}
		puts++
		var payload struct {
			Page map[string]any `json:"wiki_page"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Page["version"] != float64(6) || payload.Page["comments"] != "fix" {
			t.Errorf("payload = %#v", payload.Page)
		}
		w.WriteHeader(http.StatusNoContent)
	}, "wiki", "update", "Guide", "--project", "mobile", "--text", "new", "--comments", "fix")
	data, _ := env.Data.(map[string]any)
	if code != 0 || !env.OK || puts != 1 || data["created"] != false || data["before"] == nil {
		t.Fatalf("code=%d env=%+v puts=%d", code, env, puts)
	}
}

func TestWikiDelete(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		wantCode    string
		wantDeletes int
		wantGets    int
	}{
		{"confirmed", []string{"--confirm", "Guide"}, "", 1, 1},
		{"matching version", []string{"--confirm", "Guide", "--page-version", "4"}, "", 1, 1},
		{"missing confirm", nil, "validation_failed", 0, 0},
		{"wrong confirm", []string{"--confirm", "Other"}, "validation_failed", 0, 0},
		{"stale version", []string{"--confirm", "Guide", "--page-version", "3"}, "conflict", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gets, deletes := 0, 0
			args := append([]string{"wiki", "delete", "Guide", "--project", "mobile"}, tc.args...)
			code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/redmine/projects/mobile/wiki/Guide.json" {
					t.Errorf("unexpected URL: %s", r.URL)
				}
				switch r.Method {
				case http.MethodGet:
					gets++
					if r.URL.Query().Get("include") != "attachments" {
						t.Error("missing attachments include")
					}
					_, _ = io.WriteString(w, `{"wiki_page":{"title":"Guide","text":"old","version":4}}`)
				case http.MethodDelete:
					deletes++
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected method: %s", r.Method)
				}
			}, args...)
			if gets != tc.wantGets || deletes != tc.wantDeletes {
				t.Fatalf("gets=%d deletes=%d", gets, deletes)
			}
			if tc.wantCode == "" {
				data, _ := env.Data.(map[string]any)
				if code != 0 || !env.OK || data["deleted"] != true || data["before"] == nil {
					t.Fatalf("code=%d env=%+v", code, env)
				}
			} else if code == 0 || env.Error == nil || env.Error.Code != tc.wantCode {
				t.Fatalf("code=%d env=%+v", code, env)
			}
		})
	}
}

func TestWikiDeleteMissingPage(t *testing.T) {
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected write: %s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}, "wiki", "delete", "Guide", "--project", "mobile", "--confirm", "Guide")
	if code != output.ExitNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("code=%d env=%+v", code, env)
	}
}

func TestWikiCrudDryRunsSkipNetwork(t *testing.T) {
	for _, tc := range []struct {
		args []string
		risk string
	}{
		{[]string{"wiki", "create", "Guide", "--project", "mobile", "--text", "new", "--dry-run"}, "low"},
		{[]string{"wiki", "update", "Guide", "--project", "mobile", "--text", "new", "--page-version", "2", "--dry-run"}, "medium"},
		{[]string{"wiki", "delete", "Guide", "--project", "mobile", "--dry-run"}, "high"},
	} {
		t.Run(tc.args[1], func(t *testing.T) {
			code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("unexpected network request")
				w.WriteHeader(500)
			}, tc.args...)
			if code != 0 || env.DryRun == nil || !*env.DryRun {
				t.Fatalf("code=%d env=%+v", code, env)
			}
			op, _ := env.Meta["operation"].(map[string]any)
			if op["risk"] != tc.risk {
				t.Fatalf("risk = %v", op["risk"])
			}
		})
	}
}

func TestWikiCreateRejectsPageVersionFlag(t *testing.T) {
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected network request")
	}, "wiki", "create", "Guide", "--project", "mobile", "--text", "new", "--page-version", "2")
	if code == 0 || env.Error == nil || env.Error.Code != "validation_failed" {
		t.Fatalf("code=%d env=%+v", code, env)
	}
}
