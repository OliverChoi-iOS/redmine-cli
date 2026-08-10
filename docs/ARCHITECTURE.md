# Architecture

## Overview

The CLI is organized around a small command layer, a typed Redmine client, metadata/name resolution, and a stable JSON output layer.

```text
cmd/redmine
  -> internal/cli
    -> internal/config
    -> internal/redmine
    -> internal/cache
    -> internal/resolver
    -> internal/output
```

## Packages

### `cmd/redmine`

Thin executable entrypoint. It should call `cli.Run(args, stdout, stderr)` and exit with the returned code.

### `internal/cli`

Owns command parsing and command handlers.

Responsibilities:

- Parse global flags.
- Dispatch resource commands.
- Merge JSON input and flags.
- Enforce write risk policy.
- Write only structured results to stdout.
- Return stable exit codes.

The CLI currently uses the standard library. The project can introduce Cobra later if the command surface becomes too large.

### `internal/config`

Resolves Redmine settings.

Effective credential precedence:

1. CLI flags
2. Environment variables
3. Config file

`auth login` stores the URL and API key in the config file with private file permissions. Environment variables and flags override stored credentials for one-off commands. The CLI currently supports one Redmine server. Multiple profiles and keychain storage are not supported yet.

### `internal/redmine`

Typed Redmine REST API client.

Responsibilities:

- Build request URLs.
- Add API key authentication.
- Decode successful responses.
- Decode Redmine error responses.
- Preserve unknown plugin fields as raw JSON where needed.

Redmine remains authoritative for workflow and permission decisions.

### `internal/cache`

Stores metadata used for name resolution.

Responsibilities:

- Track metadata sync time.
- Apply TTL rules.
- Report cache status in output metadata.
- Support explicit refresh and no-cache behavior.

### `internal/resolver`

Maps human names to Redmine IDs.

Responsibilities:

- Resolve statuses, trackers, priorities, projects, users, and custom fields.
- Detect unknown names.
- Detect ambiguous names.
- Prefer exact matches over fuzzy matching.

Current user resolution is limited to `@me`, numeric IDs, exact login, and exact full name.

### `internal/output`

Owns the JSON envelope, error taxonomy, and exit code mapping.

This package is treated as part of the agent-facing API and should change carefully.

## Data Flow

Example: `redmine issue transition 123 --to "Resolved" --message-file note.redmine`

1. CLI parses command and flags.
2. Config resolves URL and API key.
3. Resolver maps `Resolved` to a status ID using metadata cache.
4. Redmine client fetches current issue for before state.
5. Redmine client sends issue update with status and notes.
6. CLI emits JSON envelope with changed fields and operation metadata.

## Write Safety

Commands carry a risk level in `schema/commands.json`.

- Low risk: immediate execution.
- Medium risk: immediate execution plus before/after when possible.
- High risk: deferred or explicit confirmation in future versions.

`--dry-run` validates inputs and name resolution without writing.

## Testing Strategy

Use Go unit tests for pure logic.

Use `httptest` for Redmine client behavior.

Use CLI integration tests with fake Redmine responses to lock stdout JSON and exit codes.

Live E2E tests are optional and gated by environment variables.
