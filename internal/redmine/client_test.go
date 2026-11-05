package redmine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIssueURL(t *testing.T) {
	client := New("https://redmine.example.com/", "key")
	if got := client.IssueURL(123); got != "https://redmine.example.com/issues/123" {
		t.Fatalf("IssueURL = %q", got)
	}
}

func TestNewRequestAddsAPIKey(t *testing.T) {
	client := New("https://redmine.example.com", "key")
	req, err := client.NewRequest("GET", "/issues.json")
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	if got := req.Header.Get("X-Redmine-API-Key"); got != "key" {
		t.Fatalf("X-Redmine-API-Key = %q", got)
	}
}

func TestNewRequestPreservesEscapedPathSegments(t *testing.T) {
	client := New("https://redmine.example.com/redmine%20app", "key")
	for _, title := range []string{"설치 가이드", "Guide#1?", "100% ready", "a/b"} {
		req, err := client.NewRequest(http.MethodGet, "/projects/mobile/wiki/"+url.PathEscape(title)+".json")
		if err != nil {
			t.Fatal(err)
		}
		want := "https://redmine.example.com/redmine%20app/projects/mobile/wiki/" + url.PathEscape(title) + ".json"
		if req.URL.String() != want {
			t.Errorf("URL = %s, want %s", req.URL, want)
		}
	}
}

func TestNewRequestRejectsHTTPByDefault(t *testing.T) {
	client := New("http://redmine.example.com", "key")
	_, err := client.NewRequest("GET", "/issues.json")
	if err == nil {
		t.Fatalf("NewRequest returned nil error")
	}
	if !strings.Contains(err.Error(), "https://") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewRequestAllowsHTTPWithOption(t *testing.T) {
	client := New("http://redmine.example.com", "key", WithAllowInsecureHTTP())
	req, err := client.NewRequest("GET", "/issues.json")
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	if req.URL.Scheme != "http" {
		t.Fatalf("scheme = %q", req.URL.Scheme)
	}
}

func TestDoMapsValidationErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["Subject cannot be blank"]}`))
	}))
	defer server.Close()

	client := New(server.URL, "key", WithAllowInsecureHTTP())
	err := client.Do(context.Background(), http.MethodPost, "/issues.json", nil, map[string]any{"issue": map[string]any{}}, nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if apiErr.Code != "validation_failed" {
		t.Fatalf("Code = %q", apiErr.Code)
	}
	if len(apiErr.RedmineErrors) != 1 {
		t.Fatalf("RedmineErrors = %#v", apiErr.RedmineErrors)
	}
}

func TestDoMapsWorkflowRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["Status is invalid"]}`))
	}))
	defer server.Close()

	client := New(server.URL, "key", WithAllowInsecureHTTP())
	err := client.UpdateIssue(context.Background(), 123, map[string]any{"status_id": 9})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if apiErr.Code != "workflow_rejected" {
		t.Fatalf("Code = %q", apiErr.Code)
	}
}

func TestDoDoesNotForwardAPIKeyAcrossOrigins(t *testing.T) {
	targetHit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
		if got := r.Header.Get("X-Redmine-API-Key"); got != "" {
			t.Fatalf("target received API key %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/users/current.json", http.StatusFound)
	}))
	defer source.Close()

	client := New(source.URL, "key", WithAllowInsecureHTTP())
	err := client.Do(context.Background(), http.MethodGet, "/users/current.json", nil, nil, nil)
	if err == nil {
		t.Fatalf("Do returned nil error")
	}
	if targetHit {
		t.Fatalf("cross-origin redirect was followed")
	}
}

func TestDoAllowsSameOriginRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/current.json":
			http.Redirect(w, r, "/redirected/current.json", http.StatusFound)
		case "/redirected/current.json":
			if got := r.Header.Get("X-Redmine-API-Key"); got != "key" {
				t.Fatalf("api key = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user":{"id":1,"login":"me"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(server.URL, "key", WithAllowInsecureHTTP())
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if user.ID != 1 {
		t.Fatalf("user = %#v", user)
	}
}

func TestDoLimitsResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"login":"` + strings.Repeat("x", 64) + `"}}`))
	}))
	defer server.Close()

	client := New(server.URL, "key", WithAllowInsecureHTTP(), WithMaxResponseBytes(16))
	err := client.Do(context.Background(), http.MethodGet, "/users/current.json", nil, nil, &struct{}{})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if apiErr.Code != "redmine_error" || apiErr.Message != "Redmine response is too large" {
		t.Fatalf("apiErr = %#v", apiErr)
	}
}

func TestSameOriginTreatsDefaultPortsAsEqual(t *testing.T) {
	a := &url.URL{Scheme: "https", Host: "redmine.example.com"}
	b := &url.URL{Scheme: "https", Host: "redmine.example.com:443"}
	if !sameOrigin(a, b) {
		t.Fatalf("sameOrigin = false")
	}
}

func TestCodeForStatusUnauthorizedSuggestsLoginAndOverrides(t *testing.T) {
	code, hint := codeForStatus(http.StatusUnauthorized, nil)
	if code != "auth_failed" {
		t.Fatalf("code = %q", code)
	}
	for _, want := range []string{"redmine auth login", "REDMINE_API_KEY", "--api-key"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("hint does not contain %q: %q", want, hint)
		}
	}
}
