# redmine-cli

[English](README.md) | [Korean](README_KR.md)

Agent-first Redmine CLI for issue workflows.

This project is scoped as an issue-first CLI for Codex and other AI agents. The default interface is non-interactive, JSON-first, and designed around stable exit codes, stable error codes, explicit write risk levels, and Redmine syntax comments.

## Requirements

- Go 1.24, either through `mise x go@1.24 -- ...` or as a direct `go` installation on `PATH`. Go 1.24 is the verified development toolchain on current macOS; the module keeps Go 1.22 language compatibility in `go.mod`.
- `mise` on `PATH` when running the commands in this README exactly as written.
- A directory for the `redmine` binary on `PATH`. The Quick Start uses `~/.local/bin` and `export PATH="$HOME/.local/bin:$PATH"`.
- A writable Go build cache. In restricted shells, prefix Go commands with `GOCACHE=/tmp/redmine-cli-go-cache`.
- A Redmine base URL, API key, and network access to the Redmine server.
- An API key with permission for the Redmine operations you run.
- An `https://` Redmine URL for normal use. `http://` is blocked by default and should only be used for local fake/test servers with `--allow-insecure-http`.

### Install mise and Go

Recommended macOS setup:

```bash
brew install mise
mise install go@1.24
mise x go@1.24 -- go version
```

If Homebrew is not available, use the official mise installer for your shell:

```bash
curl https://mise.run/zsh | sh
exec zsh
mise install go@1.24
mise x go@1.24 -- go version
```

If you prefer a direct Go installation without mise:

```bash
brew install go
go version
```

The direct Homebrew Go package may install a newer Go release than the verified Go 1.24 toolchain. If you need to reproduce the README commands exactly, use the `mise x go@1.24 -- ...` commands.

Official references:

- [mise installation methods](https://mise.jdx.dev/installing-mise.html)
- [Go download and install guide](https://go.dev/doc/install)

## Quick Start

Build the CLI, authenticate once, and then run Redmine commands without setting credential environment variables for every invocation.

### 1. Build the CLI

From the repository root:

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go build -o redmine ./cmd/redmine
mkdir -p "$HOME/.local/bin"
install -m 755 redmine "$HOME/.local/bin/redmine"
export PATH="$HOME/.local/bin:$PATH"
redmine --help
redmine --version
```

The explicit `GOCACHE` makes the build work in restricted shells that cannot write to the default Go cache. You can omit it when your normal Go cache is writable.

The `export PATH=...` line makes `redmine` available in the current shell. To keep it available in new terminals, add the same line to your shell profile, such as `~/.zshrc`, if `~/.local/bin` is not already on `PATH`.

### 2. Log in once

Replace the example URL with the base URL of your Redmine installation:

```bash
redmine auth login --url https://redmine.example.com
redmine auth status --format pretty-json
```

`auth login` verifies the API key and stores the URL and key in the CLI config file with `0600` permissions. A successful status response contains `"ok": true` and `"authenticated": true`.

Redmine URLs must use `https://` by default. For a local test server only, pass `--allow-insecure-http` on that single command.

### 3. Run the first read

```bash
redmine issue list --limit 5 --format pretty-json
```

After login, `REDMINE_URL` and `REDMINE_API_KEY` are not required. Inspect `pagination.has_more` before assuming a list is complete.

## Supported Commands

Included:

- `redmine issue list`
- `redmine issue get`
- `redmine issue create`
- `redmine issue update`
- `redmine issue comment`
- `redmine issue assign`
- `redmine issue transition`
- `redmine issue close`
- `redmine issue reopen`
- `redmine issue link`
- `redmine project list`
- `redmine project get`
- `redmine user me`
- `redmine user search`
- `redmine meta sync`
- `redmine meta statuses`
- `redmine meta trackers`
- `redmine meta priorities`
- `redmine meta custom-fields`
- `redmine config init`
- `redmine config show`
- `redmine auth login`
- `redmine auth logout`
- `redmine auth status`

## Limitations

- HTTPS is required by default. `--allow-insecure-http` is a one-run local/test escape hatch and prints a warning because API keys may be sent over cleartext HTTP.
- Credentials are stored in the CLI config file with `0600` permissions. OS keychain integration is not supported yet.
- One Redmine server/profile is supported. Multiple Redmine profiles are not supported yet.
- File uploads are not supported yet, including issue attachment upload/download.
- Admin APIs, wiki editing, time tracking, repository APIs, and bulk/destructive operations are not supported yet.
- Comments are sent as Redmine syntax. Markdown-to-Redmine conversion is not supported yet.
- `.env` files are not loaded automatically. Use `redmine auth login` for local use, or `REDMINE_URL` and `REDMINE_API_KEY` for CI and one-off overrides.

## Design Defaults

- Default output is JSON.
- `--format table` is for humans and is not a stable API.
- `stdout` is reserved for machine-readable command output.
- `stderr` is for diagnostics, warnings, and progress.
- Comments use Redmine syntax. Markdown conversion is intentionally not supported.
- Writes are risk-tiered:
  - Low risk writes run immediately.
  - Medium risk writes run immediately and return before/after changes when possible.
  - High risk writes are deferred or require explicit confirmation in future releases.
- Metadata cache is included:
  - TTL defaults to 24h.
  - Name resolution misses trigger one refresh attempt.
  - Cache status is included in JSON metadata.

## Configuration

This CLI currently targets one Redmine server.

For local use, prefer the persistent login flow:

```bash
redmine auth login --url https://redmine.example.com
redmine config show --format pretty-json
redmine auth status --format pretty-json
```

`config show` reports the config path and credential source without printing the API key.

Credential resolution order:

1. CLI flags: `--url`, `--api-key`
2. Environment: `REDMINE_URL`, `REDMINE_API_KEY`
3. Config file written by `redmine auth login`
4. Keychain support: not supported yet

Environment variables and flags are intended for CI or one-off overrides. They replace only the stored URL or API key for that invocation; other config file settings remain unchanged. `http://` URLs are rejected unless `--allow-insecure-http` is passed for that one run.

For CI, inject `REDMINE_API_KEY` through the CI secret store rather than writing a literal key into a command or file:

```bash
export REDMINE_URL="https://redmine.example.com"
# REDMINE_API_KEY is injected by the CI secret store.
redmine issue list --limit 5 --format pretty-json
```

Do not commit API keys in `.env` files, scripts, examples, command history, or process-visible arguments. Prefer `redmine auth login`, `REDMINE_API_KEY`, or `auth login --stdin` over passing literal values to `--api-key`. This CLI does not automatically load `.env` files.

`redmine auth login` stores the URL and API key in the config file with `0600` permissions, similar to GitLab CLI file-based credential storage. `REDMINE_URL`, `REDMINE_API_KEY`, `--url`, and `--api-key` still override stored credentials for a single run. `--api-key` is an unsafe escape hatch because command-line arguments can appear in shell history and process listings. `redmine auth logout` removes the stored API key.

The following example shows the non-secret settings. `auth login` adds the API key; do not add a literal key manually.

```toml
url = "https://redmine.example.com"
default_project = "mobile"
metadata_ttl = "24h"
default_format = "json"
append_footer = false
close_status = "Closed"
reopen_status = "New"
```

## Development

The CLI is implemented with the Go standard library and uses fake Redmine servers in tests for the main command contracts.

Go was chosen because this CLI is meant to be installed quickly and called reliably by agents across different environments. A single binary keeps runtime setup small, while Go's standard `net/http`, `encoding/json`, cross-compilation support, and `httptest` package are enough for a typed Redmine client, stable JSON contract, and fake Redmine server tests.

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go test ./...
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go run ./cmd/redmine --help
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go run ./cmd/redmine --version
```

The implementation should keep the JSON output contract in `schema/output.schema.json` stable.

## Troubleshooting

### `go: command not found`

Run Go through the verified mise toolchain:

```bash
mise x go@1.24 -- go version
```

### `redmine: command not found`

Install the built binary into a directory on `PATH`, then update `PATH` for the current shell:

```bash
mkdir -p "$HOME/.local/bin"
install -m 755 redmine "$HOME/.local/bin/redmine"
export PATH="$HOME/.local/bin:$PATH"
redmine --version
```

Add `export PATH="$HOME/.local/bin:$PATH"` to your shell profile, such as `~/.zshrc`, if you want new terminals to find `redmine` automatically.

### Go cache `operation not permitted`

Point `GOCACHE` at a writable temporary directory:

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go test ./...
```

### macOS `missing LC_UUID load command`

Use Go 1.24 for builds and tests on current macOS:

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go test ./...
```

### `validation_failed` for an `http://` Redmine URL

Use an `https://` Redmine URL for authenticated commands. For a local fake Redmine server in tests only, pass `--allow-insecure-http` on that invocation.

### `config_missing`, `auth_missing`, or `auth_failed`

Refresh the stored credentials, verify them, and retry the command:

```bash
redmine auth login --url https://redmine.example.com
redmine auth status --format pretty-json
```

For a one-off command, use `REDMINE_URL` and `REDMINE_API_KEY`, or the matching `--url` and `--api-key` flags.
