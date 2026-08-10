package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"redmine-cli/internal/config"
	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"

	"golang.org/x/term"
)

func (a *app) handleAuthLogin(ctx context.Context) output.Envelope {
	reader := bufio.NewReader(stdin)

	urlValue, env := a.loginURL(reader)
	if env.Error != nil {
		return env
	}

	apiKey, env := a.loginAPIKey(reader)
	if env.Error != nil {
		return env
	}

	client := redmine.New(urlValue, apiKey, a.redmineClientOptions()...)
	a.warnings = append(a.warnings, securityWarnings(urlValue, false, a.parsed.Bool("allow-insecure-http"))...)
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return a.failureFromError(err)
	}

	cfg, _, err := config.LoadFile(a.configPath)
	if err != nil {
		return output.Failure("config_missing", "Could not load config file", "Fix the config file or pass --config to another file.", map[string]any{"path": a.configPath, "error": err.Error()})
	}
	cfg.URL = urlValue
	cfg.APIKey = apiKey
	if err := config.WriteFile(a.configPath, cfg); err != nil {
		return output.Failure("config_missing", "Could not write config file", "Check the config directory permissions or pass --config.", map[string]any{"path": a.configPath, "error": err.Error()})
	}

	a.cfg = cfg
	a.client = client
	a.apiKeySource = "config"

	return a.success(map[string]any{
		"path":               a.configPath,
		"url":                urlValue,
		"api_key_stored":     true,
		"credential_storage": "config_file",
		"authenticated":      true,
		"user":               user,
	}, nil)
}

func (a *app) handleAuthLogout() output.Envelope {
	cfg, loaded, err := config.LoadFile(a.configPath)
	if err != nil {
		return output.Failure("config_missing", "Could not load config file", "Fix the config file or pass --config to another file.", map[string]any{"path": a.configPath, "error": err.Error()})
	}
	if !loaded {
		return a.success(map[string]any{
			"path":                a.configPath,
			"api_key_removed":     false,
			"env_api_key_present": os.Getenv(config.EnvAPIKey) != "",
		}, nil)
	}

	removed := cfg.APIKey != ""
	cfg.APIKey = ""
	if err := config.WriteFile(a.configPath, cfg); err != nil {
		return output.Failure("config_missing", "Could not write config file", "Check the config directory permissions or pass --config.", map[string]any{"path": a.configPath, "error": err.Error()})
	}

	a.cfg.APIKey = ""
	a.apiKeySource = "none"
	if os.Getenv(config.EnvAPIKey) != "" {
		a.apiKeySource = "env"
	}

	return a.success(map[string]any{
		"path":                a.configPath,
		"api_key_removed":     removed,
		"env_api_key_present": os.Getenv(config.EnvAPIKey) != "",
	}, nil)
}

func (a *app) loginURL(reader *bufio.Reader) (string, output.Envelope) {
	if urlValue := strings.TrimSpace(a.parsed.StringDefault("url", "")); urlValue != "" {
		return urlValue, output.Envelope{}
	}
	if a.parsed.Bool("stdin") {
		if a.cfg.URL != "" {
			return a.cfg.URL, output.Envelope{}
		}
		urlValue, err := readLoginLine(reader, a.stderr, "", false, false)
		if err != nil {
			return "", output.Failure("input_parse_error", "Could not read Redmine URL", "Provide --url or pass URL on stdin before the API key.", map[string]any{"error": err.Error()})
		}
		if strings.TrimSpace(urlValue) == "" {
			return "", output.Failure("validation_failed", "Redmine URL is required", "Provide --url or enter a Redmine URL.", nil)
		}
		return strings.TrimSpace(urlValue), output.Envelope{}
	}

	prompt := "Redmine URL: "
	if a.cfg.URL != "" {
		prompt = fmt.Sprintf("Redmine URL [%s]: ", a.cfg.URL)
	}
	line, err := readLoginLine(reader, a.stderr, prompt, false, true)
	if err != nil {
		return "", output.Failure("input_parse_error", "Could not read Redmine URL", "Provide --url or enter a Redmine URL.", map[string]any{"error": err.Error()})
	}
	urlValue := strings.TrimSpace(line)
	if urlValue == "" {
		urlValue = a.cfg.URL
	}
	if urlValue == "" {
		return "", output.Failure("validation_failed", "Redmine URL is required", "Provide --url or enter a Redmine URL.", nil)
	}
	return urlValue, output.Envelope{}
}

func (a *app) loginAPIKey(reader *bufio.Reader) (string, output.Envelope) {
	if apiKey := strings.TrimSpace(a.parsed.StringDefault("api-key", "")); apiKey != "" {
		return apiKey, output.Envelope{}
	}
	if apiKey := strings.TrimSpace(os.Getenv(config.EnvAPIKey)); apiKey != "" {
		return apiKey, output.Envelope{}
	}

	var (
		line string
		err  error
	)
	if a.parsed.Bool("stdin") {
		line, err = readLoginLine(reader, a.stderr, "", true, false)
	} else {
		line, err = readLoginLine(reader, a.stderr, "Redmine API key: ", true, true)
	}
	if err != nil {
		return "", output.Failure("input_parse_error", "Could not read Redmine API key", "Provide REDMINE_API_KEY, --api-key, --stdin, or enter the API key.", map[string]any{"error": err.Error()})
	}
	apiKey := strings.TrimSpace(line)
	if apiKey == "" {
		return "", output.Failure("auth_missing", "Redmine API key is required", "Provide REDMINE_API_KEY, --api-key, --stdin, or enter the API key.", nil)
	}
	return apiKey, output.Envelope{}
}

func readLoginLine(reader *bufio.Reader, stderr io.Writer, prompt string, hidden bool, promptEnabled bool) (string, error) {
	if promptEnabled && prompt != "" {
		fmt.Fprint(stderr, prompt)
	}
	if hidden {
		if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			data, err := term.ReadPassword(int(f.Fd()))
			if promptEnabled {
				fmt.Fprintln(stderr)
			}
			return string(data), err
		}
	}
	line, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) && line != "" {
		err = nil
	}
	return strings.TrimRight(line, "\r\n"), err
}
