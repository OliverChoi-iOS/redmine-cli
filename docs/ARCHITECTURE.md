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

## Implementation Choice

The project uses Go as a single-binary CLI because the primary caller is an agent running shell commands across varied environments. The tool should be fast to install, predictable to execute, and free from runtime dependency setup during normal use.

Go fits that constraint set:

- Single-binary distribution keeps agent setup simple.
- `net/http` and `encoding/json` are sufficient for a typed Redmine API client and stable JSON envelopes.
- Cross-compilation supports future GitHub Releases binaries and Homebrew distribution.
- `httptest` makes fake Redmine server tests straightforward.
- The language is approachable for contributors while still providing enough type safety for request/response contracts.

Rust was considered for stronger type guarantees and the `clap`/`serde` ecosystem. Go was chosen because initial implementation speed, contributor accessibility, and low-friction binary distribution were more important for this tool. Node and Python were avoided because agent environments more often run into runtime and dependency drift with those stacks.

The original planning also assumed that name/ID resolution and metadata caching would be core to safe agent usage. That remains true: the resolver and cache layer let agents use human-readable Redmine names while still sending explicit IDs to Redmine when possible.

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
- Stream binary uploads through the same authentication, redirect, bounded response, and error handling as JSON requests.

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

Wiki `put` creates or updates a page. Existing pages are read before writing, and their version is sent with the update; an explicit `--page-version` protects changes made since a caller's earlier read. An omitted parent is preserved. Because creation uses the same PUT endpoint, creation can race with another writer.

Issue attachments require two writes: upload bytes to obtain a token, then update the issue with that token. The CLI reports an uploaded file whose link was not confirmed; it does not automatically retry either write. Both wiki and attachment dry-runs validate local inputs without network requests. After a successful write, a failed final read produces a warning while preserving success.

Spent-time registration uses a single POST with an explicit issue, decimal hours, activity ID, and work date. Its dry-run validates local inputs without network requests. Activities are fetched live without extending the metadata cache. Successful creation requires a returned entry ID; an unconfirmed response produces a warning to check Redmine before retrying, since repeating the command can create duplicate hours.

## Testing Strategy

Use Go unit tests for pure logic.

Use `httptest` for Redmine client behavior.

Use CLI integration tests with fake Redmine responses to lock stdout JSON and exit codes.

Live E2E tests are optional and gated by environment variables.
