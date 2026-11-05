package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"redmine-cli/internal/cache"
	"redmine-cli/internal/config"
	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
)

const Version = "0.1.0-dev"

const defaultAllMaxItems = 1000

var stdin io.Reader = os.Stdin

var commandFlags = map[string]map[string]bool{
	"issue list": {
		"project": true, "project-id": true, "status": true, "status-id": true, "tracker": true, "tracker-id": true,
		"priority": true, "priority-id": true, "assignee": true, "assigned-to": true, "assigned-to-id": true,
		"author": true, "author-id": true, "updated-after": true, "created-after": true, "query": true,
		"sort": true, "limit": true, "offset": true, "all": true, "max-items": true, "param": true,
	},
	"issue get": {
		"include": true, "full": true, "with-notes": true,
	},
	"issue create": {
		"project": true, "project-id": true, "tracker": true, "tracker-id": true, "subject": true,
		"description": true, "description-file": true, "priority": true, "priority-id": true,
		"assignee": true, "assigned-to": true, "assigned-to-id": true, "status": true, "status-id": true,
		"custom-field": true, "custom-field-id": true, "input": true, "raw-json": true,
	},
	"issue update": {
		"set": true, "custom-field": true, "custom-field-id": true, "raw-json": true, "input": true,
		"message": true, "message-file": true,
	},
	"issue comment": {
		"message": true, "message-file": true, "message-format": true, "provenance": true,
	},
	"issue assign": {
		"to": true, "message": true, "message-file": true,
	},
	"issue transition": {
		"to": true, "message": true, "message-file": true,
	},
	"issue close": {
		"to": true, "message": true, "message-file": true,
	},
	"issue reopen": {
		"to": true, "message": true, "message-file": true,
	},
	"issue link": {
		"relation": true,
	},
	"issue attach":    {"file": true, "filename": true, "description": true, "content-type": true},
	"wiki list":       {"project": true},
	"wiki get":        {"project": true, "page-version": true, "include": true},
	"wiki put":        {"project": true, "text": true, "text-file": true, "comments": true, "parent-title": true, "page-version": true},
	"time create":     {"issue": true, "hours": true, "activity-id": true, "spent-on": true, "comments": true, "comments-file": true},
	"time activities": {},
	"project list": {
		"limit": true, "offset": true, "all": true, "max-items": true,
	},
	"project get": {},
	"user me":     {},
	"user search": {
		"limit": true, "offset": true,
	},
	"meta sync":          {"refresh": true},
	"meta statuses":      {"refresh": true},
	"meta trackers":      {"project": true, "refresh": true},
	"meta priorities":    {"refresh": true},
	"meta custom-fields": {"project": true, "refresh": true},
	"config init":        {"url": true, "default-project": true, "metadata-ttl": true, "close-status": true, "reopen-status": true},
	"config show":        {},
	"auth login":         {"stdin": true},
	"auth logout":        {},
	"auth status":        {},
}

var globalFlags = map[string]bool{
	"format": true, "url": true, "api-key": true, "config": true, "cache-dir": true,
	"refresh": true, "no-cache": true, "dry-run": true, "allow-insecure-http": true, "help": true, "version": true,
}

var boolFlags = map[string]bool{
	"all": true, "dry-run": true, "full": true, "help": true, "no-cache": true,
	"refresh": true, "stdin": true, "version": true, "with-notes": true, "allow-insecure-http": true,
}

type parsedArgs struct {
	Positionals []string
	Flags       map[string][]string
}

type app struct {
	parsed       parsedArgs
	stdout       io.Writer
	stderr       io.Writer
	cfg          config.Config
	configPath   string
	configLoaded bool
	apiKeySource string
	cacheDir     string
	cachePath    string
	client       *redmine.Client
	meta         cache.Metadata
	metaLoaded   bool
	metaStatus   cache.Status
	warnings     []string
}

func Run(args []string, stdout, stderr io.Writer) int {
	parsed, err := parseArgs(args)
	if err != nil {
		env := output.Failure("validation_failed", "Invalid arguments", err.Error(), map[string]any{"argv": args})
		_ = output.WriteJSON(stdout, env, false)
		return output.ExitCodeForError(env.Error.Code)
	}

	if len(args) == 0 || parsed.Bool("help") || parsed.HasShortHelp(args) {
		printHelp(stdout)
		return output.ExitOK
	}
	if parsed.Bool("version") || parsed.HasShortVersion(args) {
		fmt.Fprintln(stdout, Version)
		return output.ExitOK
	}

	application, env := newApp(parsed, stdout, stderr)
	if env.Error != nil {
		_ = output.WriteJSON(stdout, env, parsed.StringDefault("format", "") == "pretty-json")
		return output.ExitCodeForError(env.Error.Code)
	}

	env = application.dispatch()
	return application.write(env)
}

func newApp(parsed parsedArgs, stdout, stderr io.Writer) (*app, output.Envelope) {
	configPath := parsed.StringDefault("config", "")
	if configPath == "" {
		var err error
		configPath, err = config.DefaultConfigPath()
		if err != nil {
			return nil, output.Failure("config_missing", "Could not resolve config path", "Set --config to an explicit path.", map[string]any{"error": err.Error()})
		}
	}
	fileCfg, loaded, err := config.LoadFile(configPath)
	if err != nil {
		return nil, output.Failure("config_missing", "Could not load config file", "Fix the config file or pass --config to another file.", map[string]any{"path": configPath, "error": err.Error()})
	}

	envCfg := config.FromEnv()
	flagCfg := config.Config{
		URL:    parsed.StringDefault("url", ""),
		APIKey: parsed.StringDefault("api-key", ""),
	}
	if v := parsed.StringDefault("metadata-ttl", ""); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, output.Failure("validation_failed", "Invalid metadata TTL", "Use a Go duration such as 24h.", map[string]any{"value": v})
		}
		flagCfg.MetadataTTL = d
	}
	if v := parsed.StringDefault("default-project", ""); v != "" {
		flagCfg.DefaultProject = v
	}
	if v := parsed.StringDefault("format", ""); v != "" {
		flagCfg.DefaultFormat = v
	}
	if v := parsed.StringDefault("close-status", ""); v != "" {
		flagCfg.CloseStatus = v
	}
	if v := parsed.StringDefault("reopen-status", ""); v != "" {
		flagCfg.ReopenStatus = v
	}
	cfg := config.Merge(config.Defaults(), fileCfg, envCfg, flagCfg)
	apiKeySource := "none"
	if fileCfg.APIKey != "" {
		apiKeySource = "config"
	}
	if envCfg.APIKey != "" {
		apiKeySource = "env"
	}
	if flagCfg.APIKey != "" {
		apiKeySource = "flag"
	}

	cacheDir := parsed.StringDefault("cache-dir", "")
	if cacheDir == "" {
		cacheDir, err = config.DefaultCacheDir()
		if err != nil {
			return nil, output.Failure("cache_error", "Could not resolve cache directory", "Set --cache-dir to an explicit path.", map[string]any{"error": err.Error()})
		}
	}

	warnings := securityWarnings(cfg.URL, flagCfg.APIKey != "", parsed.Bool("allow-insecure-http"))
	clientOpts := redmineOptions(parsed.Bool("allow-insecure-http"))

	return &app{
		parsed:       parsed,
		stdout:       stdout,
		stderr:       stderr,
		cfg:          cfg,
		configPath:   configPath,
		configLoaded: loaded,
		apiKeySource: apiKeySource,
		cacheDir:     cacheDir,
		cachePath:    cache.MetadataPath(cacheDir),
		client:       redmine.New(cfg.URL, cfg.APIKey, clientOpts...),
		metaStatus:   cache.Status{Metadata: "miss"},
		warnings:     warnings,
	}, output.Envelope{}
}

func (a *app) redmineClientOptions() []redmine.Option {
	return redmineOptions(a.parsed.Bool("allow-insecure-http"))
}

func redmineOptions(allowInsecureHTTP bool) []redmine.Option {
	if allowInsecureHTTP {
		return []redmine.Option{redmine.WithAllowInsecureHTTP()}
	}
	return nil
}

func (a *app) dispatch() output.Envelope {
	key := a.commandKey()
	if _, ok := commandFlags[key]; !ok {
		return output.Failure(
			"validation_failed",
			"Unknown command",
			"Run redmine --help to see supported command groups.",
			map[string]any{"argv": a.parsed.Positionals},
		)
	}
	if err := a.validateFlags(key); err != nil {
		return output.Failure("validation_failed", "Unsupported flag for command", err.Error(), map[string]any{"command": key})
	}

	switch key {
	case "config init":
		return a.handleConfigInit()
	case "config show":
		return a.success(config.PublicMap(a.cfg, a.configPath, a.configLoaded, a.apiKeySource), nil)
	case "auth status":
		return a.handleAuthStatus()
	case "auth login":
		return a.handleAuthLogin(context.Background())
	case "auth logout":
		return a.handleAuthLogout()
	}

	if fail := a.requireClient(); fail.Error != nil {
		return fail
	}

	switch key {
	case "issue list":
		return a.handleIssueList(context.Background())
	case "issue get":
		return a.handleIssueGet(context.Background())
	case "issue create":
		return a.handleIssueCreate(context.Background())
	case "issue update":
		return a.handleIssueUpdate(context.Background())
	case "issue comment":
		return a.handleIssueComment(context.Background())
	case "issue assign":
		return a.handleIssueAssign(context.Background())
	case "issue transition":
		return a.handleIssueTransition(context.Background())
	case "issue close":
		return a.handleIssueClose(context.Background())
	case "issue reopen":
		return a.handleIssueReopen(context.Background())
	case "issue link":
		return a.handleIssueLink(context.Background())
	case "issue attach":
		return a.handleIssueAttach(context.Background())
	case "wiki list", "wiki get", "wiki put":
		return a.handleWiki(context.Background())
	case "time create":
		return a.handleTimeCreate(context.Background())
	case "time activities":
		return a.handleTimeActivities(context.Background())
	case "project list":
		return a.handleProjectList(context.Background())
	case "project get":
		return a.handleProjectGet(context.Background())
	case "user me":
		return a.handleUserMe(context.Background())
	case "user search":
		return a.handleUserSearch(context.Background())
	case "meta sync":
		return a.handleMetaSync(context.Background())
	case "meta statuses":
		return a.handleMetaSlice(context.Background(), "statuses")
	case "meta trackers":
		return a.handleMetaSlice(context.Background(), "trackers")
	case "meta priorities":
		return a.handleMetaSlice(context.Background(), "priorities")
	case "meta custom-fields":
		return a.handleMetaSlice(context.Background(), "custom_fields")
	default:
		return output.Failure("validation_failed", "Unknown command", "Run redmine --help to see supported command groups.", map[string]any{"command": key})
	}
}

func (a *app) write(env output.Envelope) int {
	format := a.cfg.DefaultFormat
	if format == "" {
		format = "json"
	}
	switch format {
	case "json", "pretty-json", "table":
	default:
		env = output.Failure("validation_failed", "Unsupported output format", "Use --format json, --format pretty-json, or --format table.", map[string]any{"format": format})
	}

	if len(a.warnings) > 0 {
		env.Warnings = append(env.Warnings, a.warnings...)
	}

	if format == "table" && env.OK {
		if err := writeTable(a.stdout, env.Data); err != nil {
			fallback := output.Failure("internal_error", "Could not render table output", "Use --format json for the stable output contract.", map[string]any{"error": err.Error()})
			_ = output.WriteJSON(a.stdout, fallback, false)
			return output.ExitCodeForError(fallback.Error.Code)
		}
		return output.ExitOK
	}

	pretty := format == "pretty-json"
	_ = output.WriteJSON(a.stdout, env, pretty)
	if env.OK {
		return output.ExitOK
	}
	return output.ExitCodeForError(env.Error.Code)
}

func (a *app) success(data any, pagination *output.Pagination) output.Envelope {
	meta := map[string]any{
		"url": a.cfg.URL,
	}
	if a.metaStatus.Metadata != "" && a.metaStatus.Metadata != "miss" {
		meta["cache"] = map[string]any{
			"metadata":    a.metaStatus.Metadata,
			"refreshed":   a.metaStatus.Refreshed,
			"age_seconds": a.metaStatus.Age,
		}
	}
	env := output.Success(data, meta)
	env.Pagination = pagination
	return env
}

func (a *app) requireClient() output.Envelope {
	if strings.TrimSpace(a.cfg.URL) == "" {
		return output.Failure("config_missing", "Redmine URL is not configured", "Run redmine auth login, or pass --url/set REDMINE_URL for a one-off command.", nil)
	}
	if strings.TrimSpace(a.cfg.APIKey) == "" {
		return output.Failure("auth_missing", "Redmine API key is not configured", "Run redmine auth login, or pass --api-key/set REDMINE_API_KEY for a one-off command.", nil)
	}
	return output.Envelope{}
}

func securityWarnings(redmineURL string, apiKeyFlagUsed bool, allowInsecureHTTP bool) []string {
	var warnings []string
	if apiKeyFlagUsed {
		warnings = append(warnings, "--api-key can expose the API key in shell history or process listings; prefer redmine auth login, REDMINE_API_KEY, or stdin.")
	}
	if allowInsecureHTTP && strings.HasPrefix(strings.ToLower(strings.TrimSpace(redmineURL)), "http://") {
		warnings = append(warnings, "API key may be sent over cleartext HTTP because --allow-insecure-http is set.")
	}
	return warnings
}

func (a *app) failureFromError(err error) output.Envelope {
	var apiErr *redmine.APIError
	if errors.As(err, &apiErr) {
		return output.Failure(apiErr.Code, apiErr.Message, apiErr.Hint, apiErr.Details)
	}
	return output.Failure("internal_error", "Command failed", "Inspect error.details.error and retry after fixing the cause.", map[string]any{"error": err.Error()})
}

func (a *app) commandKey() string {
	if len(a.parsed.Positionals) < 2 {
		return strings.Join(a.parsed.Positionals, " ")
	}
	return strings.Join(a.parsed.Positionals[:2], " ")
}

func (a *app) arg(index int) string {
	if index < len(a.parsed.Positionals) {
		return a.parsed.Positionals[index]
	}
	return ""
}

func (a *app) requireID(index int, name string) (int, output.Envelope) {
	raw := a.arg(index)
	if raw == "" {
		return 0, output.Failure("validation_failed", "Missing required argument", "Provide "+name+".", map[string]any{"argument": name})
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, output.Failure("validation_failed", "Invalid numeric argument", "Use a positive integer for "+name+".", map[string]any{"argument": name, "value": raw})
	}
	return id, output.Envelope{}
}

func (a *app) validateFlags(command string) error {
	allowed := commandFlags[command]
	for name := range a.parsed.Flags {
		if globalFlags[name] || allowed[name] {
			continue
		}
		return fmt.Errorf("--%s is not supported by %s", name, command)
	}
	return nil
}

func parseArgs(args []string) (parsedArgs, error) {
	parsed := parsedArgs{Flags: map[string][]string{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-h" {
			parsed.Flags["help"] = append(parsed.Flags["help"], "true")
			continue
		}
		if arg == "-v" {
			parsed.Flags["version"] = append(parsed.Flags["version"], "true")
			continue
		}
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			if arg == "--" {
				parsed.Positionals = append(parsed.Positionals, args[i+1:]...)
				break
			}
			parsed.Positionals = append(parsed.Positionals, arg)
			continue
		}

		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")
		if name == "" {
			return parsed, fmt.Errorf("empty flag name")
		}
		if boolFlags[name] {
			if !hasValue {
				value = "true"
			}
			parsed.Flags[name] = append(parsed.Flags[name], value)
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("--%s requires a value", name)
			}
			i++
			value = args[i]
		}
		parsed.Flags[name] = append(parsed.Flags[name], value)
	}
	return parsed, nil
}

func (p parsedArgs) Values(name string) []string {
	return append([]string(nil), p.Flags[name]...)
}

func (p parsedArgs) StringDefault(name string, def string) string {
	values := p.Flags[name]
	if len(values) == 0 {
		return def
	}
	return values[len(values)-1]
}

func (p parsedArgs) Bool(name string) bool {
	raw := p.StringDefault(name, "")
	return raw == "true" || raw == "1" || raw == "yes"
}

func (p parsedArgs) IntDefault(name string, def int) (int, error) {
	raw := p.StringDefault(name, "")
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("--%s must be an integer", name)
	}
	return n, nil
}

func (p parsedArgs) HasShortHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" {
			return true
		}
	}
	return false
}

func (p parsedArgs) HasShortVersion(args []string) bool {
	for _, arg := range args {
		if arg == "-v" {
			return true
		}
	}
	return false
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `redmine is an agent-first Redmine CLI.

Usage:
  redmine <resource> <command> [arguments] [flags]

Resources:
  issue      list, get, create, update, comment, assign, transition, close, reopen, link, attach
  wiki       list, get, put (create or update)
  time       create, activities
  project    list, get
  user       me, search
  meta       sync, statuses, trackers, priorities, custom-fields
  config     init, show
  auth       login, logout, status

Global flags:
  --format json|pretty-json|table
  --url URL
  --api-key API_KEY
  --config PATH
  --cache-dir PATH
  --refresh
  --no-cache
  --dry-run
  --allow-insecure-http

Wiki and attachments:
  wiki list --project IDENTIFIER_OR_ID
  wiki get TITLE --project IDENTIFIER_OR_ID [--page-version N] [--include attachments]
  wiki put TITLE --project IDENTIFIER_OR_ID --text-file PATH [--comments TEXT] [--page-version N] [--dry-run]
  issue attach ID --file PATH [--filename NAME] [--description TEXT] [--content-type TYPE] [--dry-run]
  wiki put creates missing pages and checks the current version when updating existing pages.

Spent time:
  time activities
  time create --issue ID --hours N --activity-id ID --spent-on YYYY-MM-DD [--comments TEXT | --comments-file PATH] [--dry-run]
  Hours are decimal hours: 1.5 means 90 minutes. Comments allow at most 255 characters.
  Work date and activity ID are required. Entries are created for the authenticated user.

Auth:
  auth login stores URL and API key in the config file with 0600 permissions.
  After login, REDMINE_URL and REDMINE_API_KEY are not required.
  Environment variables and flags are optional overrides for a single run.
  --api-key can be visible in shell history and process listings; prefer auth login, REDMINE_API_KEY, or stdin.
  http:// Redmine URLs are rejected unless --allow-insecure-http is set for a one-off local command.

Defaults:
  issue list uses --status open --sort updated_on:desc --limit 50.
  stdout is reserved for command output; stderr is used for diagnostics.`)
}
