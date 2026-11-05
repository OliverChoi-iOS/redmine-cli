package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestFromEnv(t *testing.T) {
	t.Setenv(EnvURL, "https://redmine.example.com")
	t.Setenv(EnvAPIKey, "secret")

	cfg := FromEnv()
	if cfg.URL != "https://redmine.example.com" {
		t.Fatalf("URL = %q", cfg.URL)
	}
	if cfg.APIKey != "secret" {
		t.Fatalf("APIKey = %q", cfg.APIKey)
	}
	if cfg.DefaultProject != "" || cfg.MetadataTTL != 0 || cfg.DefaultFormat != "" || cfg.AppendFooter || cfg.CloseStatus != "" || cfg.ReopenStatus != "" {
		t.Fatalf("FromEnv returned non-environment settings: %#v", cfg)
	}
}

func TestMergeEnvironmentOnlyOverridesCredentials(t *testing.T) {
	fileCfg := Defaults()
	fileCfg.URL = "https://stored.example.com"
	fileCfg.APIKey = "stored-secret"
	fileCfg.DefaultProject = "mobile"
	fileCfg.MetadataTTL = 12 * time.Hour
	fileCfg.DefaultFormat = "table"
	fileCfg.AppendFooter = true
	fileCfg.CloseStatus = "Done"
	fileCfg.ReopenStatus = "Backlog"

	tests := []struct {
		name       string
		envURL     string
		envAPIKey  string
		wantURL    string
		wantAPIKey string
	}{
		{
			name:       "empty environment preserves stored credentials",
			wantURL:    fileCfg.URL,
			wantAPIKey: fileCfg.APIKey,
		},
		{
			name:       "environment overrides stored credentials",
			envURL:     "https://override.example.com",
			envAPIKey:  "override-secret",
			wantURL:    "https://override.example.com",
			wantAPIKey: "override-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvURL, tt.envURL)
			t.Setenv(EnvAPIKey, tt.envAPIKey)

			got := Merge(Defaults(), fileCfg, FromEnv())
			if got.URL != tt.wantURL || got.APIKey != tt.wantAPIKey {
				t.Fatalf("credentials = (%q, %q), want (%q, %q)", got.URL, got.APIKey, tt.wantURL, tt.wantAPIKey)
			}
			if got.DefaultProject != fileCfg.DefaultProject || got.MetadataTTL != fileCfg.MetadataTTL || got.DefaultFormat != fileCfg.DefaultFormat || got.AppendFooter != fileCfg.AppendFooter || got.CloseStatus != fileCfg.CloseStatus || got.ReopenStatus != fileCfg.ReopenStatus {
				t.Fatalf("file settings were overwritten: got %#v, want %#v", got, fileCfg)
			}
		})
	}
}

func TestLoadFileAllowsStoredAPIKey(t *testing.T) {
	path := t.TempDir() + "/config.toml"
	if err := os.WriteFile(path, []byte(`url = "https://redmine.example.com"
api_key = "stored-secret"
`), 0o600); err != nil {
		t.Fatalf("WriteFile fixture: %v", err)
	}

	cfg, loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if !loaded {
		t.Fatalf("loaded = false")
	}
	if cfg.APIKey != "stored-secret" {
		t.Fatalf("APIKey = %q", cfg.APIKey)
	}
}

func TestWriteFileStoresAPIKeyWithPrivatePermissions(t *testing.T) {
	path := t.TempDir() + "/config.toml"
	cfg := Defaults()
	cfg.URL = "https://redmine.example.com"
	cfg.APIKey = "stored-secret"

	if err := WriteFile(path, cfg); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), `api_key = "stored-secret"`) {
		t.Fatalf("config did not contain api_key: %s", string(data))
	}
	assertPrivateConfigFile(t, path)
}

func TestWriteFileRepairsPermissiveExistingConfigBeforeRewrite(t *testing.T) {
	path := t.TempDir() + "/config.toml"
	if err := os.WriteFile(path, []byte(`url = "https://old.example.com"`), 0o644); err != nil {
		t.Fatalf("WriteFile fixture: %v", err)
	}

	cfg := Defaults()
	cfg.URL = "https://redmine.example.com"
	cfg.APIKey = "stored-secret"
	cfg.DefaultProject = "mobile"
	if err := WriteFile(path, cfg); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	assertPrivateConfigFile(t, path)
	loaded, ok, err := LoadFile(path)
	if err != nil || !ok {
		t.Fatalf("LoadFile = loaded %t err %v", ok, err)
	}
	if loaded.APIKey != "stored-secret" || loaded.DefaultProject != "mobile" {
		t.Fatalf("loaded config = %#v", loaded)
	}
}

func TestWriteFileRejectsSymlinkConfigPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.toml")
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(target, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile fixture: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(1314)) {
			t.Skip("creating symlinks requires Windows Developer Mode or administrator privileges")
		}
		t.Fatalf("Symlink: %v", err)
	}

	cfg := Defaults()
	cfg.URL = "https://redmine.example.com"
	cfg.APIKey = "stored-secret"
	if err := WriteFile(path, cfg); err == nil {
		t.Fatalf("WriteFile returned nil error")
	}
}

func TestPublicMapDoesNotExposeAPIKey(t *testing.T) {
	cfg := Defaults()
	cfg.URL = "https://redmine.example.com"
	cfg.APIKey = "stored-secret"

	public := PublicMap(cfg, "/tmp/config.toml", true, "config")
	if strings.Contains(strings.Join(mapValues(public), " "), "stored-secret") {
		t.Fatalf("public map leaked api key: %#v", public)
	}
	if public["api_key_present"] != true || public["api_key_source"] != "config" {
		t.Fatalf("public map = %#v", public)
	}
}

func mapValues(values map[string]any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimSpace(valueString(value)))
	}
	return out
}

func valueString(value any) string {
	return fmt.Sprint(value)
}
