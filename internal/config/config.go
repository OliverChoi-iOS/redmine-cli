package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	EnvURL    = "REDMINE_URL"
	EnvAPIKey = "REDMINE_API_KEY"
)

type Config struct {
	URL            string
	APIKey         string
	DefaultProject string
	MetadataTTL    time.Duration
	DefaultFormat  string
	AppendFooter   bool
	CloseStatus    string
	ReopenStatus   string
}

func Defaults() Config {
	return Config{
		MetadataTTL:   24 * time.Hour,
		DefaultFormat: "json",
		CloseStatus:   "Closed",
		ReopenStatus:  "New",
	}
}

func FromEnv() Config {
	return Config{
		URL:    os.Getenv(EnvURL),
		APIKey: os.Getenv(EnvAPIKey),
	}
}

func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "redmine-cli", "config.toml"), nil
}

func DefaultCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "redmine-cli"), nil
}

func LoadFile(path string) (Config, bool, error) {
	cfg := Defaults()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := stripComment(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			return cfg, true, fmt.Errorf("invalid config line %d", lineNo)
		}
		key = strings.TrimSpace(key)
		value, err := parseTomlScalar(strings.TrimSpace(raw))
		if err != nil {
			return cfg, true, fmt.Errorf("invalid config line %d: %w", lineNo, err)
		}
		switch key {
		case "url":
			cfg.URL = value
		case "default_project":
			cfg.DefaultProject = value
		case "metadata_ttl":
			if value != "" {
				d, err := time.ParseDuration(value)
				if err != nil {
					return cfg, true, fmt.Errorf("invalid metadata_ttl: %w", err)
				}
				cfg.MetadataTTL = d
			}
		case "default_format":
			cfg.DefaultFormat = value
		case "append_footer":
			cfg.AppendFooter = value == "true"
		case "close_status":
			cfg.CloseStatus = value
		case "reopen_status":
			cfg.ReopenStatus = value
		case "api_key":
			cfg.APIKey = value
		default:
			return cfg, true, fmt.Errorf("unknown config key %q", key)
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, true, err
	}
	return cfg, true, nil
}

func Merge(base Config, overlays ...Config) Config {
	out := base
	for _, next := range overlays {
		if next.URL != "" {
			out.URL = next.URL
		}
		if next.APIKey != "" {
			out.APIKey = next.APIKey
		}
		if next.DefaultProject != "" {
			out.DefaultProject = next.DefaultProject
		}
		if next.MetadataTTL != 0 {
			out.MetadataTTL = next.MetadataTTL
		}
		if next.DefaultFormat != "" {
			out.DefaultFormat = next.DefaultFormat
		}
		if next.CloseStatus != "" {
			out.CloseStatus = next.CloseStatus
		}
		if next.ReopenStatus != "" {
			out.ReopenStatus = next.ReopenStatus
		}
		if next.AppendFooter {
			out.AppendFooter = true
		}
	}
	return out
}

func WriteFile(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := validateConfigDir(dir); err != nil {
		return err
	}
	if err := prepareConfigPath(path); err != nil {
		return err
	}

	var b strings.Builder
	writeString(&b, "url", cfg.URL)
	writeString(&b, "default_project", cfg.DefaultProject)
	if cfg.MetadataTTL != 0 {
		writeString(&b, "metadata_ttl", cfg.MetadataTTL.String())
	}
	if cfg.DefaultFormat != "" {
		writeString(&b, "default_format", cfg.DefaultFormat)
	}
	fmt.Fprintf(&b, "append_footer = %t\n", cfg.AppendFooter)
	writeString(&b, "close_status", cfg.CloseStatus)
	writeString(&b, "reopen_status", cfg.ReopenStatus)
	writeString(&b, "api_key", cfg.APIKey)

	tmp, err := os.CreateTemp(dir, ".config.toml.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write([]byte(b.String())); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return os.Chmod(path, 0o600)
}

func validateConfigDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("config parent is not a directory")
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("config parent directory must not be group- or world-writable")
	}
	return nil
}

func prepareConfigPath(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("config file must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("config file must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return os.Chmod(path, 0o600)
	}
	return nil
}

func PublicMap(cfg Config, configPath string, configLoaded bool, apiKeySource string) map[string]any {
	return map[string]any{
		"url":             cfg.URL,
		"default_project": cfg.DefaultProject,
		"metadata_ttl":    cfg.MetadataTTL.String(),
		"default_format":  cfg.DefaultFormat,
		"append_footer":   cfg.AppendFooter,
		"close_status":    cfg.CloseStatus,
		"reopen_status":   cfg.ReopenStatus,
		"api_key_present": cfg.APIKey != "",
		"api_key_source":  apiKeySource,
		"config_path":     configPath,
		"config_loaded":   configLoaded,
	}
}

func stripComment(line string) string {
	inQuote := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == '#' && !inQuote {
			return line[:i]
		}
	}
	return line
}

func parseTomlScalar(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if raw == "true" || raw == "false" {
		return raw, nil
	}
	if strings.HasPrefix(raw, "\"") {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", err
		}
		return value, nil
	}
	return strings.TrimSpace(raw), nil
}

func writeString(b *strings.Builder, key string, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s = %q\n", key, value)
}
