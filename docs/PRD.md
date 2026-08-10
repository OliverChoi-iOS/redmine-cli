# PRD: Agent-first Redmine CLI

## Objective

Build a Go single-binary Redmine CLI optimized for AI agents such as Codex. The tool should make common issue workflows safe, parseable, and recoverable while remaining convenient enough for humans during development and debugging.

## Primary User

The primary user is an AI agent running shell commands in a non-interactive environment.

Secondary users are engineers who want to inspect or debug the same issue workflows manually.

## Product Principles

- Agent-first, human-friendly second.
- JSON-first output.
- Non-interactive by default.
- Stable error codes and small stable exit code set.
- Semantic commands first, raw escape hatches available.
- Redmine server remains authoritative for workflow and permission validation.
- Do not hide partial data. Pagination and truncation must be explicit.
- Do not silently write unknown fields.
- Preserve Redmine syntax for comments. Do not auto-convert Markdown.

## Supported Commands

Issue commands:

- `issue list`
- `issue get`
- `issue create`
- `issue update`
- `issue comment`
- `issue assign`
- `issue transition`
- `issue close`
- `issue reopen`
- `issue link`

Discovery commands:

- `project list`
- `project get`
- `user me`
- `user search`
- `meta sync`
- `meta statuses`
- `meta trackers`
- `meta priorities`
- `meta custom-fields`

Configuration commands:

- `config init`
- `config show`
- `auth login`
- `auth logout`
- `auth status`

## Deferred Features

- Attachments upload/download
- Multiple profiles
- OS keychain login
- Admin APIs
- Wiki editing
- Time tracking
- Bulk operations
- Destructive operations
- Markdown conversion
- Full workflow transition computation

## Output Contract

Default output is JSON.

Success:

```json
{
  "ok": true,
  "data": {},
  "meta": {}
}
```

Failure:

```json
{
  "ok": false,
  "error": {
    "code": "validation_failed",
    "message": "Human-readable message",
    "hint": "Optional recovery hint",
    "details": {}
  },
  "meta": {}
}
```

`stdout` must contain only the command result. `stderr` is for diagnostics.

## Error Codes

Stable error codes:

- `config_missing`
- `auth_missing`
- `auth_failed`
- `network_error`
- `timeout`
- `redmine_error`
- `not_found`
- `validation_failed`
- `ambiguous_name`
- `unknown_name`
- `unsupported_filter`
- `workflow_rejected`
- `conflict`
- `rate_limited`
- `permission_denied`
- `input_parse_error`
- `file_not_found`
- `cache_error`
- `internal_error`

Exit codes:

- `0`: success
- `1`: general error
- `2`: usage or input error
- `3`: config or auth error
- `4`: network or server error
- `5`: not found
- `6`: permission denied
- `7`: conflict or workflow rejected

## Write Risk Policy

Low risk:

- `issue comment`
- `issue create`
- `issue link`

Medium risk:

- `issue update`
- `issue assign`
- `issue transition`
- `issue close`
- `issue reopen`

High risk:

- Delete operations
- Destructive bulk updates

High-risk operations are not supported.

All write commands should support `--dry-run`.

## Input Model

Simple usage is flag-based.

Complex create/update payloads support JSON input:

```bash
redmine issue create --input issue.json
redmine issue create --input -
```

Flags override fields from `--input`.

Description and comment bodies support file input:

```bash
redmine issue create --description-file body.redmine
redmine issue comment 123 --message-file result.redmine
```

## Authentication Model

`auth login` is the preferred setup path. It reads the Redmine URL and API key from stdin/prompts, verifies the key with `/users/current.json`, and stores the URL plus API key in the config file using private file permissions.

`auth logout` removes the stored API key while preserving non-secret settings.

`REDMINE_API_KEY` and `--api-key` override the stored key for a single command. `auth status` reports whether a key is present and its source, but never prints the key value.

## Redmine Syntax Comments

Comments and update notes accept Redmine syntax directly.

The CLI does not convert Markdown to Redmine syntax.

Example:

```text
h3. Work result

* Cause: ...
* Change: ...
* Verification: ...
```

## Metadata Cache and Name Resolution

The CLI should resolve names to IDs for statuses, trackers, priorities, projects, users, and custom fields where possible.

Cache policy:

- TTL default: 24h
- `redmine meta sync` refreshes explicitly
- `--refresh` refreshes before command execution
- `--no-cache` bypasses cache
- Name miss triggers one automatic refresh
- JSON `meta.cache` reports cache hit/miss/refresh status

Ambiguity and unknown names must fail with structured errors.

## Pagination

List commands are bounded by default.

Defaults:

- `--limit 50`
- `--sort updated_on:desc`
- `--status open` for `issue list`

`--all` is explicit and capped by `--max-items`, default `1000`.

Output includes pagination metadata with `has_more`, `next_offset`, and `truncated` when relevant.

## Validation

Client-side validation is intentionally minimal.

For `issue create`, the CLI requires:

- `subject`
- `project` from `--project` or `default_project`

Redmine remains authoritative for:

- Required custom fields
- Workflow rules
- Permissions
- Project-specific trackers/statuses

Redmine validation failures should map to `validation_failed` and include Redmine error messages in `error.details.redmine_errors`.

## Testing Requirements

Release quality requires:

- Unit tests for config resolution
- Unit tests for name resolution
- Unit tests for JSON input plus flag override behavior
- Unit tests for output envelopes and error mapping
- HTTP client tests using `httptest`
- CLI integration tests with a fake Redmine server
- JSON contract tests for stdout and exit codes

Live Redmine E2E tests are optional and should only run when environment variables such as `REDMINE_E2E_URL` and `REDMINE_E2E_API_KEY` are present.
