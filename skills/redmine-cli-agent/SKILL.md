---
name: redmine-cli-agent
description: Configure and use this repository's agent-first Redmine CLI safely. Use when Codex needs to build the local `redmine` binary, run `redmine auth login/logout/status`, inspect, list, create, update, transition, close, reopen, link, or comment on Redmine issues; manage stored Redmine URL/API-key credentials; sync metadata; resolve Redmine project/status/tracker/priority/user names; handle JSON envelopes, pagination, dry-runs, warnings, stable error codes, HTTPS-only URL validation, and Redmine CLI/API failures.
---

# Redmine CLI Agent

Use the local `redmine` CLI as the first-choice interface for Redmine work in this repository. Treat stdout as the machine-readable contract and stderr as diagnostics, warnings, and progress.

## Locate or Build the CLI

Prefer `redmine` on `PATH`. If it is missing and the current repository is `redmine-cli`, build the local binary:

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go build -o redmine ./cmd/redmine
./redmine --help
./redmine --version
```

Use the explicit `GOCACHE` in restricted macOS shells that cannot write to the default Go cache. Go 1.24 is the verified development toolchain for this repository.

For immediate local use, either run `./redmine ...` from the repository root or install the binary into a directory on `PATH`:

```bash
mkdir -p ~/.local/bin
install -m 755 redmine ~/.local/bin/redmine
```

Do not create a project `mise.toml` or change `go.mod` just to run this CLI.

## Core Rules

- Prefer default JSON output; use `--format pretty-json` only for readable inspection.
- Parse stdout only. Treat stderr as diagnostics and warnings.
- Branch on `ok`, `error.code`, and process exit code.
- Inspect `meta.warnings` in JSON output when present.
- Use `redmine auth login` for persistent local setup.
- Use `REDMINE_URL`, `REDMINE_API_KEY`, `--url`, or `--api-key` only for CI or one-off overrides.
- Never print, quote, log, or include API key values in final answers.
- Avoid literal `--api-key` values in shell commands because argv can be visible in shell history and process listings.
- Use `https://` Redmine URLs by default.
- Use `--allow-insecure-http` only for a one-off local fake/test Redmine server; expect a warning that the API key may be sent over cleartext HTTP.
- Use Redmine syntax for comments and notes. Do not convert Markdown unless the user explicitly requests Redmine-compatible text and verifies the server format.
- Never assume a list command returned every matching issue unless pagination says so.

## Authentication Setup

Interactive setup:

```bash
redmine auth login --url https://redmine.example.com
redmine auth status --format pretty-json
```

`auth login` verifies the API key with Redmine and stores the URL plus API key in the CLI config file with `0600` permissions. After login, `REDMINE_URL` and `REDMINE_API_KEY` are not required for normal commands.

Non-interactive setup with the API key on stdin:

```bash
printf '%s\n' "$REDMINE_API_KEY" | redmine auth login --url https://redmine.example.com --stdin
```

Remove the stored API key:

```bash
redmine auth logout --format pretty-json
```

This CLI stores credentials in its config file. OS keychain integration and multiple Redmine profiles are not supported yet.

## Standard Setup Flow

Run these in order before doing meaningful work:

```bash
redmine auth login --url https://redmine.example.com
redmine config show --format pretty-json
redmine auth status --format pretty-json
redmine meta sync --format pretty-json
redmine issue list --limit 5 --format pretty-json
```

Expected success shape:

```json
{"ok": true, "data": {}, "meta": {}}
```

If `meta sync` returns warnings for custom fields or users, continue with read commands when possible. Use numeric custom field IDs when custom field metadata is unavailable. User resolution supports `@me`, numeric IDs, exact login, and exact full name where the server allows search.

## Read Workflow

Use compact reads first:

```bash
redmine issue list --project mobile --status open --limit 20 --format pretty-json
redmine issue get 123 --format pretty-json
```

Fetch deeper context only when needed:

```bash
redmine issue get 123 --include journals,attachments,relations,children --format pretty-json
redmine issue get 123 --with-notes --format pretty-json
```

Always inspect `pagination.has_more`, `pagination.next_offset`, `pagination.total_count`, and `pagination.truncated` before assuming a list is complete. Use `--offset` for the next page or `--all --max-items N` for a bounded scan.

## Write Workflow

Run writes only when the user requested the mutation or the task clearly requires it.

Low-risk writes can run directly when the user explicitly asks:

```bash
redmine issue comment 123 --message-file result.redmine --format pretty-json
redmine issue link 123 456 --relation relates --format pretty-json
```

For create/update/transition/assign/close/reopen, prefer `--dry-run` first unless the user clearly requested the exact mutation:

```bash
redmine issue create --subject "Title" --project mobile --tracker Task --priority Normal --dry-run --format pretty-json
redmine issue transition 123 --to Confirmed --message-file note.redmine --dry-run --format pretty-json
```

After dry-run, check `data.request` for resolved IDs and only then run without `--dry-run`.

Use `--message-file` or `--description-file` for long text. Keep files free of API keys and other secrets.

## Current Limitations

- File uploads are not supported yet, including issue attachment upload/download.
- OS keychain integration is not supported yet.
- Multiple Redmine profiles are not supported yet.
- Admin APIs, wiki editing, time tracking, repository APIs, and bulk/destructive operations are not supported yet.
- Markdown-to-Redmine conversion is not supported.
- `.env` files are not loaded automatically.

## Error Handling

Recover by `error.code`:

- `config_missing`: run `redmine auth login`, or pass `--url`/set `REDMINE_URL` for one command.
- `auth_missing` or `auth_failed`: run `redmine auth login`, or use `REDMINE_API_KEY`/`--api-key` for a one-off override.
- `validation_failed` with an HTTP URL: use an `https://` URL, or pass `--allow-insecure-http` only for a local fake/test server.
- `not_found` with an HTML body: check whether the Redmine base URL path is correct.
- `permission_denied`: authenticated but the key lacks permission; try a narrower read or report the permission boundary.
- `unknown_name`: run `redmine meta sync --refresh` once, then use numeric IDs if still unknown.
- `ambiguous_name`: choose from `error.details.candidates`.
- `workflow_rejected`: Redmine rejected the transition; fetch the issue and try a valid intermediate status.
- `network_error` or `timeout`: retry only when safe, and preserve the original error context in the final answer.

## Command Reference

For examples and less common flags, read [references/commands.md](references/commands.md).
