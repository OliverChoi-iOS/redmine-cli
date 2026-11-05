# redmine-cli

[English](README.md) | [Korean](README_KR.md)

Agent-first Redmine CLI for issue workflows.

This project is scoped as an issue-first CLI for Codex and other AI agents. The default interface is non-interactive, JSON-first, and designed around stable exit codes, stable error codes, explicit write risk levels, and Redmine syntax comments.

## Requirements

- No Go or mise installation is needed for release binaries. Go is only needed for [development](#development).
- A directory for the `redmine` binary on `PATH`. The Quick Start uses `~/.local/bin` and `export PATH="$HOME/.local/bin:$PATH"`.
- macOS/Linux: `curl`, `tar`, and `sha256sum` or `shasum`. Windows: PowerShell 5.1 or later.
- A Redmine base URL, API key, and network access to the Redmine server.
- An API key with permission for the Redmine operations you run.
- An `https://` Redmine URL for normal use. `http://` is blocked by default and should only be used for local fake/test servers with `--allow-insecure-http`.

## Quick Start

Install the CLI, authenticate once, and run Redmine commands.

### 1. Install the CLI

macOS and Linux (Intel/AMD64 or ARM64):

```bash
curl -fsSL https://github.com/OliverChoi-iOS/redmine-cli/releases/latest/download/install.sh -o install.sh
sh install.sh
export PATH="$HOME/.local/bin:$PATH"
redmine --version
```

The installer verifies the archive's SHA-256 checksum and version, then installs to `~/.local/bin`. Review the downloaded script before running it. Re-run it to update; authentication settings are preserved. Add the `export PATH=...` line to your shell profile (for example, `~/.zshrc`) for new terminals.

To select a version or destination:

```bash
VERSION=v0.1.0 INSTALL_DIR="$HOME/.local/bin" sh install.sh
```

Windows PowerShell (AMD64 or ARM64):

```powershell
Invoke-WebRequest https://github.com/OliverChoi-iOS/redmine-cli/releases/latest/download/install.ps1 -OutFile install.ps1
& ./install.ps1
$env:Path = "$env:LOCALAPPDATA\redmine-cli\bin;$env:Path"
redmine --version
```

The default destination is `%LOCALAPPDATA%\redmine-cli\bin`. Add it to your user PATH for new terminals. Use `./install.ps1 -Version v0.1.0 -InstallDir C:\Tools\redmine` to customize installation. Follow your organization's PowerShell execution policy if script execution is restricted.

Windows supports `auth login`, `auth status`, and `auth logout`. Credentials are saved in `%APPDATA%\redmine-cli\config.toml` with a protected Windows ACL granting access only to the current user. The destination must support Windows ACLs (for example, NTFS); shared directories writable by other users are rejected. The installer and CLI do not require administrator privileges.

You can also download and extract an archive from [Releases](https://github.com/OliverChoi-iOS/redmine-cli/releases), verify it against `checksums.txt`, and place `redmine` (or `redmine.exe`) on PATH. Installation downloads become available after the first stable release is published. Checksums detect corrupted downloads; release signing is not configured.

To uninstall, remove the installed binary. Config and cache files are kept.

### 2. Log in once

Replace the example URL with the base URL of your Redmine installation:

```bash
redmine auth login --url https://redmine.example.com
redmine auth status --format pretty-json
```

`auth login` verifies the API key and stores the URL and key in the CLI config file with `0600` permissions on macOS/Linux or a current-user-only ACL on Windows. A successful status response contains `"ok": true` and `"authenticated": true`.

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
- `redmine issue attach`
- `redmine wiki list`
- `redmine wiki get`
- `redmine wiki put`
- `redmine time create`
- `redmine time activities`
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

## Wiki pages and issue attachments

Use a project identifier or numeric ID for `--project`; omitting it uses `default_project`.

```bash
redmine wiki list --project mobile
redmine wiki get "GettingStarted" --project mobile --include attachments
redmine wiki put "GettingStarted" --project mobile --text-file guide.redmine --comments "Update setup" --dry-run
# Remove --dry-run to create or update the page.
redmine wiki put "GettingStarted" --project mobile --text-file guide.redmine --page-version 4
redmine issue attach 123 --file evidence.png --description "Reproduction screenshot" --dry-run
# Remove --dry-run to upload and attach the file.
```

`wiki put` sends the supplied text as Redmine wiki syntax without conversion. It creates missing pages and replaces existing page text. When updating, it fetches and sends the current version to detect concurrent edits during the command. Pass the version you originally read as `--page-version N` to also protect edits made since that read; a stale version returns `conflict` (exit 7). `wiki get --page-version N` reads historical content. Existing parent pages are preserved unless `--parent-title` is supplied; `--parent-title ""` makes a root page. An initial create can race with another writer because the API uses the same PUT endpoint for creation and updates.

`issue attach` requires a non-empty regular file and uploads one file per command. `--filename` changes its remote name; `--content-type` overrides the MIME type inferred from the extension (otherwise `application/octet-stream`). Server size limits and permissions apply. Redmine 7.1+ detects content types itself. Uploading and linking are separate requests: a linking error includes `error.details.upload_completed` and `attachment_link_confirmed: false`. Check the issue before retrying, since a retry uploads again and may duplicate an attachment if the original response was lost. A failed link may leave an unused upload on the server.

Both write commands validate local input without contacting Redmine under `--dry-run`; server permissions, the automatic wiki version, and existing parent are checked during execution. Successful writes return `before`/`after` where available. If only the final read fails, `ok` remains true with a warning; do not repeat the write just to fetch its result.

API references: [Wiki pages](https://www.redmine.org/projects/redmine/wiki/Rest_WikiPages), [attaching files](https://www.redmine.org/projects/redmine/wiki/Rest_api#Attaching-files).

## Log spent time

List activity IDs, then create an entry for an issue as the authenticated user:

```bash
redmine time activities --format pretty-json
redmine time create --issue 123 --hours 1.5 --activity-id 9 --spent-on 2026-11-05 --comments "Fix login bug" --dry-run
# Replace 9 with an activity ID from your server. Remove --dry-run to register the entry.
```

`--issue`, `--hours`, `--activity-id`, and `--spent-on` are required. Hours must be a positive finite number in hours (`1.5` is 90 minutes); dates use `YYYY-MM-DD`. Explicit dates and activity IDs prevent unintended server defaults. Optional `--comments` or `--comments-file PATH` accepts at most 255 characters, including file line breaks. `--dry-run` validates local input and returns `data.request` without contacting Redmine.

Successful registration returns the created entry, including its ID, hours, work date, user, and activity. This records actual spent time; it does not change `estimated_hours`. Redmine enforces logging permissions, project activity settings, required custom fields, and hour/date limits. Activity IDs are read directly from `/enumerations/time_entry_activities.json`, without the metadata cache.

Each execution creates a new entry. The CLI does not automatically retry; after a network error or an unconfirmed response, inspect the issue's spent-time entries in Redmine before retrying to avoid duplicate hours. Time entry listing, editing, deletion, project-only entries, and logging on behalf of another user are not supported yet.

API references: [Time entries](https://www.redmine.org/projects/redmine/wiki/Rest_TimeEntries), [activity enumeration](https://www.redmine.org/projects/redmine/wiki/Rest_Enumerations#enumerationstime_entry_activitiesformat).

## Limitations

- HTTPS is required by default. `--allow-insecure-http` is a one-run local/test escape hatch and prints a warning because API keys may be sent over cleartext HTTP.
- Credentials are stored in the CLI config file with `0600` permissions on macOS/Linux or a current-user-only ACL on Windows. This restricts file access; it does not encrypt the key. OS keychain integration is not supported yet.
- One Redmine server/profile is supported. Multiple Redmine profiles are not supported yet.
- File downloads, wiki attachments, and project Files uploads are not supported yet. `issue attach` uploads one file per invocation.
- Admin APIs, repository APIs, and bulk/destructive operations are not supported yet. Time entry registration is supported; listing, editing, and deletion are not.
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

`redmine auth login` stores the URL and API key in a private config file (`0600` on macOS/Linux; a protected current-user-only ACL on Windows), similar to GitLab CLI file-based credential storage. `REDMINE_URL`, `REDMINE_API_KEY`, `--url`, and `--api-key` still override stored credentials for a single run. `--api-key` is an unsafe escape hatch because command-line arguments can appear in shell history and process listings. `redmine auth logout` removes the stored API key.

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

Go 1.24 is the verified development toolchain; `go.mod` keeps Go 1.22 language compatibility. Install Go directly using the [Go installation guide](https://go.dev/doc/install), or use [mise](https://mise.jdx.dev/installing-mise.html):

```bash
brew install mise
mise install go@1.24
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go build -o redmine ./cmd/redmine
./redmine --version
```

The CLI is implemented with the Go standard library and uses fake Redmine servers in tests for the main command contracts.

Go was chosen because this CLI is meant to be installed quickly and called reliably by agents across different environments. A single binary keeps runtime setup small, while Go's standard `net/http`, `encoding/json`, cross-compilation support, and `httptest` package are enough for a typed Redmine client, stable JSON contract, and fake Redmine server tests.

```bash
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go test ./...
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go run ./cmd/redmine --help
GOCACHE=/tmp/redmine-cli-go-cache mise x go@1.24 -- go run ./cmd/redmine --version
```

The implementation should keep the JSON output contract in `schema/output.schema.json` stable.

### Publish a release

Commit the release workflow and installers to `main`, then push a new stable version tag (replace `v0.1.0` with the next unused version):

```bash
git push origin main
git tag v0.1.0
git push origin v0.1.0
```

[Release Actions](https://github.com/OliverChoi-iOS/redmine-cli/actions/workflows/release.yml) runs the full Go suite and installer tests on macOS, Linux, and Windows for pushes to `main` and pull requests. A stable `vMAJOR.MINOR.PATCH` tag additionally builds six CGO-free binaries (darwin/linux/windows × amd64/arm64) and publishes archives, both installers, `checksums.txt`, and generated release notes. `redmine --version` reports the tag without `v`.

The workflow uses the built-in `GITHUB_TOKEN` with `contents: write`; no personal access token is needed. A failed publication may leave a draft release. Inspect it in GitHub Releases before retrying. Published releases are not overwritten.

Run offline installer tests locally with `sh tests/install_test.sh` or, on Windows with Go installed, `./tests/install_test.ps1`.

## Troubleshooting

### `go: command not found`

Run Go through the verified mise toolchain:

```bash
mise x go@1.24 -- go version
```

### `redmine: command not found`

If you used the release installer, update `PATH` for the current shell:

```bash
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
