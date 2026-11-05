package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"redmine-cli/internal/output"
)

func runContentCommand(t *testing.T, handler http.HandlerFunc, args ...string) (int, output.Envelope) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()
	t.Setenv("REDMINE_URL", server.URL+"/redmine")
	t.Setenv("REDMINE_API_KEY", "test-key")
	args = append(args, "--allow-insecure-http", "--config", filepath.Join(t.TempDir(), "config.toml"), "--cache-dir", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, stdout.String())
	}
	return code, env
}

func TestWikiPutPreservesParentAndChecksVersion(t *testing.T) {
	gets, puts := 0, 0
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/redmine/projects/mobile/wiki/설치 가이드.json" || strings.Contains(r.RequestURI, "%25EC") {
			t.Errorf("unexpected URL: %s", r.RequestURI)
		}
		if r.Method == http.MethodGet {
			gets++
			_, _ = io.WriteString(w, `{"wiki_page":{"title":"설치 가이드","text":"old","version":4,"parent":{"title":"Manual"}}}`)
			return
		}
		puts++
		var payload struct {
			Page map[string]any `json:"wiki_page"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.Page["version"] != float64(4) || payload.Page["parent_title"] != "Manual" || payload.Page["text"] != "h1. New" {
			t.Errorf("payload = %#v", payload.Page)
		}
		w.WriteHeader(http.StatusNoContent)
	}, "wiki", "put", "설치 가이드", "--project", "mobile", "--text", "h1. New")
	if code != 0 || !env.OK || gets != 2 || puts != 1 {
		t.Fatalf("code=%d env=%+v gets=%d puts=%d", code, env, gets, puts)
	}
}

func TestWikiPutCreatesPageAndKeepsSuccessAfterReadFailure(t *testing.T) {
	gets, puts := 0, 0
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
			w.WriteHeader(http.StatusNotFound)
			return
		}
		puts++
		w.WriteHeader(http.StatusCreated)
	}, "wiki", "put", "Guide", "--project", "mobile", "--text", "h1. Guide")
	if code != 0 || !env.OK || gets != 2 || puts != 1 || len(env.Warnings) < 2 || env.Data.(map[string]any)["created"] != true {
		t.Fatalf("code=%d env=%+v gets=%d puts=%d", code, env, gets, puts)
	}
}

func TestWikiPutConflict(t *testing.T) {
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"wiki_page":{"title":"Guide","version":5}}`)
			return
		}
		var payload struct {
			Page map[string]any `json:"wiki_page"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Page["version"] != float64(3) {
			t.Errorf("version = %v", payload.Page["version"])
		}
		w.WriteHeader(http.StatusConflict)
	}, "wiki", "put", "Guide", "--project", "mobile", "--text", "new", "--page-version", "3")
	if code != output.ExitConflict || env.Error == nil || env.Error.Code != "conflict" {
		t.Fatalf("code=%d env=%+v", code, env)
	}
}

func TestWikiPutDoesNotWriteAfterPermissionFailure(t *testing.T) {
	code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected write: %s", r.Method)
		}
		w.WriteHeader(http.StatusForbidden)
	}, "wiki", "put", "Guide", "--project", "mobile", "--text", "new")
	if code != output.ExitPermission || env.Error == nil || env.Error.Code != "permission_denied" {
		t.Fatalf("code=%d env=%+v", code, env)
	}
}

func TestWikiListAndGet(t *testing.T) {
	for _, command := range []string{"list", "get"} {
		t.Run(command, func(t *testing.T) {
			args := []string{"wiki", command, "--project", "mobile"}
			if command == "get" {
				args = append(args, "Guide", "--page-version", "2", "--include", "attachments")
			}
			code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
				if command == "list" {
					if r.URL.Path != "/redmine/projects/mobile/wiki/index.json" {
						t.Error(r.URL)
					}
					_, _ = io.WriteString(w, `{"wiki_pages":[{"title":"Guide","version":2}]}`)
				} else {
					if r.URL.Path != "/redmine/projects/mobile/wiki/Guide/2.json" || r.URL.Query().Get("include") != "attachments" {
						t.Error(r.URL)
					}
					_, _ = io.WriteString(w, `{"wiki_page":{"title":"Guide","text":"old","version":2,"attachments":[{"id":7}]}}`)
				}
			}, args...)
			if code != 0 || !env.OK {
				t.Fatalf("code=%d env=%+v", code, env)
			}
		})
	}
}

func TestContentCommandsValidateWithoutNetwork(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file+".empty", nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"wiki dry run", []string{"wiki", "put", "Guide", "--project", "mobile", "--text", "new", "--dry-run"}, ""},
		{"wiki text file", []string{"wiki", "put", "Guide", "--project", "mobile", "--text-file", file, "--dry-run"}, ""},
		{"conflicting text", []string{"wiki", "put", "Guide", "--project", "mobile", "--text", "new", "--text-file", file}, "validation_failed"},
		{"missing text file", []string{"wiki", "put", "Guide", "--project", "mobile", "--text-file", file + "missing"}, "file_not_found"},
		{"attachment dry run", []string{"issue", "attach", "123", "--file", file, "--dry-run"}, ""},
		{"missing text", []string{"wiki", "put", "Guide", "--project", "mobile"}, "validation_failed"},
		{"invalid version", []string{"wiki", "get", "Guide", "--project", "mobile", "--page-version", "0"}, "validation_failed"},
		{"invalid title", []string{"wiki", "get", "../Guide", "--project", "mobile"}, "validation_failed"},
		{"missing file", []string{"issue", "attach", "123", "--file", file + "missing"}, "file_not_found"},
		{"directory", []string{"issue", "attach", "123", "--file", filepath.Dir(file)}, "validation_failed"},
		{"empty file", []string{"issue", "attach", "123", "--file", file + ".empty"}, "validation_failed"},
		{"duplicate file", []string{"issue", "attach", "123", "--file", file, "--file", file}, "validation_failed"},
		{"invalid filename", []string{"issue", "attach", "123", "--file", file, "--filename", "../test"}, "validation_failed"},
		{"control in filename", []string{"issue", "attach", "123", "--file", file, "--filename", "test\t.txt"}, "validation_failed"},
		{"invalid MIME", []string{"issue", "attach", "123", "--file", file, "--content-type", "invalid"}, "validation_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("unexpected network request")
				w.WriteHeader(500)
			}, tc.args...)
			if tc.want == "" {
				if code != 0 || env.DryRun == nil || !*env.DryRun {
					t.Fatalf("code=%d env=%+v", code, env)
				}
			} else if code == 0 || env.Error == nil || env.Error.Code != tc.want {
				t.Fatalf("code=%d env=%+v", code, env)
			}
		})
	}
}

func TestIssueAttachUploadAndLink(t *testing.T) {
	for _, failure := range []string{"", "upload", "link", "after", "issue", "token"} {
		t.Run("failure="+failure, func(t *testing.T) {
			content := []byte{'x', 0, 255, '\n'}
			file := filepath.Join(t.TempDir(), "sample.bin")
			if err := os.WriteFile(file, content, 0600); err != nil {
				t.Fatal(err)
			}
			gets, uploads, links := 0, 0, 0
			code, env := runContentCommand(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Redmine-API-Key") != "test-key" {
					t.Error("missing authentication")
				}
				switch r.Method {
				case http.MethodGet:
					gets++
					if failure == "issue" {
						w.WriteHeader(404)
						return
					}
					if failure == "after" && gets == 2 {
						w.WriteHeader(403)
						return
					}
					if r.URL.Query().Get("include") != "attachments" {
						t.Error("missing attachments include")
					}
					_, _ = io.WriteString(w, `{"issue":{"id":123,"attachments":[{"id":7,"filename":"증거.bin"}]}}`)
				case http.MethodPost:
					uploads++
					if r.URL.Path != "/redmine/uploads.json" || r.URL.Query().Get("filename") != "증거.bin" {
						t.Error(r.URL)
					}
					body, _ := io.ReadAll(r.Body)
					if !bytes.Equal(body, content) || r.ContentLength != int64(len(content)) || r.Header.Get("Content-Type") != "application/octet-stream" {
						t.Errorf("incorrect upload: %v", body)
					}
					if failure == "upload" {
						w.WriteHeader(413)
						return
					}
					if failure == "token" {
						_, _ = io.WriteString(w, `{}`)
						return
					}
					w.WriteHeader(201)
					_, _ = io.WriteString(w, `{"upload":{"token":"upload-token"}}`)
				case http.MethodPut:
					links++
					var body struct {
						Issue struct {
							Uploads []map[string]any `json:"uploads"`
						} `json:"issue"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if len(body.Issue.Uploads) != 1 {
						t.Error(body)
						return
					}
					upload := body.Issue.Uploads[0]
					if upload["token"] != "upload-token" || upload["filename"] != "증거.bin" || upload["description"] != "evidence" {
						t.Error(upload)
					}
					if failure == "link" {
						w.WriteHeader(403)
						return
					}
					w.WriteHeader(204)
				}
			}, "issue", "attach", "123", "--file", file, "--filename", "증거.bin", "--description", "evidence")
			switch failure {
			case "", "after":
				if code != 0 || !env.OK || gets != 2 || uploads != 1 || links != 1 {
					t.Fatalf("code=%d env=%+v gets=%d uploads=%d links=%d", code, env, gets, uploads, links)
				}
			case "issue":
				if code == 0 || uploads != 0 || links != 0 {
					t.Fatalf("code=%d uploads=%d links=%d", code, uploads, links)
				}
			case "upload", "token":
				if code == 0 || uploads != 1 || links != 0 {
					t.Fatalf("code=%d uploads=%d links=%d", code, uploads, links)
				}
			case "link":
				if code == 0 || env.Error.Details["upload_completed"] != true || env.Error.Details["attachment_link_confirmed"] != false {
					t.Fatalf("code=%d env=%+v", code, env)
				}
			}
		})
	}
}
