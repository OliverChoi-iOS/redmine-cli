package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"redmine-cli/internal/config"
	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	if stdout.String() != Version+"\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunHelpExplainsPersistentLoginAndOptionalOverrides(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	for _, want := range []string{"auth login", "REDMINE_URL and REDMINE_API_KEY are not required", "optional overrides", "--allow-insecure-http", "shell history"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help does not contain %q: %s", want, stdout.String())
		}
	}
}

func TestRunImplementedCommandRequiresAuth(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"issue", "list", "--config", configPath, "--cache-dir", cacheDir}, &stdout, &stderr)
	if code != output.ExitConfigAuth {
		t.Fatalf("exit code = %d", code)
	}

	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	if env.OK {
		t.Fatalf("env.OK = true, want false")
	}
	if env.Error == nil || env.Error.Code != "config_missing" {
		t.Fatalf("error = %#v", env.Error)
	}
	for _, want := range []string{"redmine auth login", "--url", "REDMINE_URL"} {
		if !strings.Contains(env.Error.Hint, want) {
			t.Fatalf("hint does not contain %q: %q", want, env.Error.Hint)
		}
	}
}

func TestRunImplementedCommandWithoutAPIKeySuggestsLoginAndOverrides(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"issue", "list", "--url", "https://redmine.example.com", "--config", configPath, "--cache-dir", cacheDir}, &stdout, &stderr)
	if code != output.ExitConfigAuth {
		t.Fatalf("exit code = %d", code)
	}

	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	if env.Error == nil || env.Error.Code != "auth_missing" {
		t.Fatalf("error = %#v", env.Error)
	}
	for _, want := range []string{"redmine auth login", "--api-key", "REDMINE_API_KEY"} {
		if !strings.Contains(env.Error.Hint, want) {
			t.Fatalf("hint does not contain %q: %q", want, env.Error.Hint)
		}
	}
}

func TestRunRejectsHTTPWithoutExplicitOptIn(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"issue", "list", "--url", "http://redmine.example.com", "--api-key", "key", "--config", configPath, "--cache-dir", cacheDir}, &stdout, &stderr)
	if code != output.ExitUsage {
		t.Fatalf("exit code = %d", code)
	}

	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	if env.Error == nil || env.Error.Code != "validation_failed" {
		t.Fatalf("error = %#v", env.Error)
	}
	if !strings.Contains(env.Error.Hint, "--allow-insecure-http") {
		t.Fatalf("hint = %q", env.Error.Hint)
	}
}

func TestRunUnknownCommandReturnsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"nope"}, &stdout, &stderr)
	if code != output.ExitUsage {
		t.Fatalf("exit code = %d", code)
	}

	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	if env.Error == nil || env.Error.Code != "validation_failed" {
		t.Fatalf("error = %#v", env.Error)
	}
}

func TestAuthLoginStoresAPIKeyFromStdin(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/current.json" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Redmine-API-Key"); got != "secret-key" {
			t.Fatalf("api key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":150,"login":"jkchoi","firstname":"종국","lastname":"최"}}`))
	}))
	defer server.Close()

	oldStdin := stdin
	stdin = strings.NewReader("secret-key\n")
	t.Cleanup(func() { stdin = oldStdin })

	var stdout, stderr bytes.Buffer
	code := Run([]string{"auth", "login", "--url", server.URL, "--stdin", "--allow-insecure-http", "--config", configPath, "--cache-dir", cacheDir, "--format", "pretty-json"}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "secret-key") {
		t.Fatalf("stdout leaked API key: %s", stdout.String())
	}

	cfg, loaded, err := config.LoadFile(configPath)
	if err != nil || !loaded {
		t.Fatalf("LoadFile = loaded %t err %v", loaded, err)
	}
	if cfg.URL != server.URL || cfg.APIKey != "secret-key" {
		t.Fatalf("stored config = %#v", cfg)
	}
}

func TestAuthLoginReadsURLAndAPIKeyFromStdin(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/current.json" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Redmine-API-Key"); got != "secret-key" {
			t.Fatalf("api key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":150,"login":"jkchoi"}}`))
	}))
	defer server.Close()

	oldStdin := stdin
	stdin = strings.NewReader(server.URL + "\nsecret-key\n")
	t.Cleanup(func() { stdin = oldStdin })

	var stdout, stderr bytes.Buffer
	code := Run([]string{"auth", "login", "--stdin", "--allow-insecure-http", "--config", configPath, "--cache-dir", cacheDir, "--format", "pretty-json"}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "secret-key") {
		t.Fatalf("stdout leaked API key: %s", stdout.String())
	}

	cfg, loaded, err := config.LoadFile(configPath)
	if err != nil || !loaded {
		t.Fatalf("LoadFile = loaded %t err %v", loaded, err)
	}
	if cfg.URL != server.URL || cfg.APIKey != "secret-key" {
		t.Fatalf("stored config = %#v", cfg)
	}
}

func TestAuthStatusUsesStoredAPIKey(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Redmine-API-Key"); got != "stored-key" {
			t.Fatalf("api key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":150,"login":"jkchoi"}}`))
	}))
	defer server.Close()

	cfg := config.Defaults()
	cfg.URL = server.URL
	cfg.APIKey = "stored-key"
	if err := config.WriteFile(configPath, cfg); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"auth", "status", "--allow-insecure-http", "--config", configPath, "--cache-dir", cacheDir, "--format", "pretty-json"}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "stored-key") {
		t.Fatalf("stdout leaked API key: %s", stdout.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	data := decoded["data"].(map[string]any)
	if data["authenticated"] != true || data["api_key_source"] != "config" {
		t.Fatalf("data = %#v", data)
	}
}

func TestAuthLogoutRemovesStoredAPIKey(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	cfg := config.Defaults()
	cfg.URL = "https://redmine.example.com"
	cfg.APIKey = "stored-key"
	if err := config.WriteFile(configPath, cfg); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"auth", "logout", "--config", configPath, "--cache-dir", cacheDir, "--format", "pretty-json"}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "stored-key") {
		t.Fatalf("stdout leaked API key: %s", stdout.String())
	}

	loadedCfg, loaded, err := config.LoadFile(configPath)
	if err != nil || !loaded {
		t.Fatalf("LoadFile = loaded %t err %v", loaded, err)
	}
	if loadedCfg.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty", loadedCfg.APIKey)
	}
}

func TestIssueListDefaultRequestAndPagination(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issues.json" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Redmine-API-Key"); got != "key" {
			t.Fatalf("api key = %q", got)
		}
		q := r.URL.Query()
		if got := q.Get("status_id"); got != "open" {
			t.Fatalf("status_id = %q", got)
		}
		if got := q.Get("sort"); got != "updated_on:desc" {
			t.Fatalf("sort = %q", got)
		}
		if got := q.Get("limit"); got != "50" {
			t.Fatalf("limit = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[{"id":123,"subject":"Fix bug","status":{"id":1,"name":"New"}}],"total_count":2,"offset":0,"limit":50}`))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"issue", "list", "--url", server.URL, "--api-key", "key", "--allow-insecure-http", "--config", configPath, "--cache-dir", cacheDir}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %#v", env.Error)
	}
	if env.Pagination == nil || !env.Pagination.HasMore || env.Pagination.TotalCount != 2 {
		t.Fatalf("pagination = %#v", env.Pagination)
	}
	for _, want := range []string{"--api-key can expose", "cleartext HTTP"} {
		if !strings.Contains(strings.Join(env.Warnings, "\n"), want) {
			t.Fatalf("warnings do not contain %q: %#v", want, env.Warnings)
		}
	}
}

func TestIssueCommentSendsNotes(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/issues/123.json" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		var body map[string]map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("body decode: %v", err)
		}
		if got := body["issue"]["notes"]; got != "h3. Done" {
			t.Fatalf("notes = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"issue", "comment", "123", "--message", "h3. Done", "--url", server.URL, "--api-key", "key", "--allow-insecure-http", "--config", configPath, "--cache-dir", cacheDir}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %#v", env.Error)
	}
}

func TestIssueCreateDryRunResolvesProject(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/issue_statuses.json":
			_, _ = w.Write([]byte(`{"issue_statuses":[{"id":1,"name":"New"}]}`))
		case "/trackers.json":
			_, _ = w.Write([]byte(`{"trackers":[{"id":2,"name":"Bug"}]}`))
		case "/enumerations/issue_priorities.json":
			_, _ = w.Write([]byte(`{"issue_priorities":[{"id":3,"name":"Normal"}]}`))
		case "/custom_fields.json":
			_, _ = w.Write([]byte(`{"custom_fields":[]}`))
		case "/projects.json":
			_, _ = w.Write([]byte(`{"projects":[{"id":42,"name":"Mobile","identifier":"mobile"}],"total_count":1,"offset":0,"limit":100}`))
		case "/users.json":
			_, _ = w.Write([]byte(`{"users":[],"total_count":0,"offset":0,"limit":100}`))
		case "/issues.json":
			t.Fatalf("dry-run must not create issue")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"issue", "create", "--subject", "Fix bug", "--project", "mobile", "--dry-run", "--url", server.URL, "--api-key", "key", "--allow-insecure-http", "--config", configPath, "--cache-dir", cacheDir}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	if decoded["ok"] != true || decoded["dry_run"] != true {
		t.Fatalf("decoded = %#v", decoded)
	}
	data := decoded["data"].(map[string]any)
	request := data["request"].(map[string]any)
	if got := request["project_id"]; got != float64(42) {
		t.Fatalf("project_id = %#v", got)
	}
}

func TestMetaSyncAllowsInsecureHTTPWithWarning(t *testing.T) {
	t.Setenv("REDMINE_URL", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := t.TempDir() + "/config.toml"
	cacheDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/issue_statuses.json":
			_, _ = w.Write([]byte(`{"issue_statuses":[{"id":1,"name":"New"}]}`))
		case "/trackers.json":
			_, _ = w.Write([]byte(`{"trackers":[{"id":2,"name":"Bug"}]}`))
		case "/enumerations/issue_priorities.json":
			_, _ = w.Write([]byte(`{"issue_priorities":[{"id":3,"name":"Normal"}]}`))
		case "/custom_fields.json":
			_, _ = w.Write([]byte(`{"custom_fields":[]}`))
		case "/projects.json":
			_, _ = w.Write([]byte(`{"projects":[],"total_count":0,"offset":0,"limit":100}`))
		case "/users.json":
			_, _ = w.Write([]byte(`{"users":[],"total_count":0,"offset":0,"limit":100}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"meta", "sync", "--url", server.URL, "--api-key", "key", "--allow-insecure-http", "--config", configPath, "--cache-dir", cacheDir}, &stdout, &stderr)
	if code != output.ExitOK {
		t.Fatalf("exit code = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	var env output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout was not JSON: %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %#v", env.Error)
	}
	if !strings.Contains(strings.Join(env.Warnings, "\n"), "cleartext HTTP") {
		t.Fatalf("warnings = %#v", env.Warnings)
	}
}

func TestWriteTableSanitizesTerminalControls(t *testing.T) {
	issue := redmine.Issue{
		ID:        123,
		Project:   redmine.Reference{Name: "Mobile\x1b[31m"},
		Status:    redmine.Reference{Name: "New"},
		Subject:   "fix\x1b[0m\tbad\nnext\x1b]52;c;AAAA\a",
		UpdatedOn: "2026-08-10",
	}

	var buf bytes.Buffer
	if err := writeTable(&buf, map[string]any{"issues": []redmine.Issue{issue}}); err != nil {
		t.Fatalf("writeTable: %v", err)
	}
	out := buf.String()
	for _, forbidden := range []string{"\x1b", "\a", "]52", "\tbad", "\nbad"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("table output contains %q: %q", forbidden, out)
		}
	}
	if !strings.Contains(out, "fix bad next") {
		t.Fatalf("table output = %q", out)
	}
}
