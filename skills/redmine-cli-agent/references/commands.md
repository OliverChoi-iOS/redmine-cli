# Redmine CLI Command Notes

## Core Commands

```bash
redmine auth login
redmine auth logout
redmine config show --format pretty-json
redmine auth status --format pretty-json
redmine meta sync --format pretty-json
redmine meta statuses --format pretty-json
redmine meta trackers --format pretty-json
redmine meta priorities --format pretty-json
redmine meta custom-fields --format pretty-json
```

## Authentication

Interactive setup:

```bash
redmine auth login --url https://redmine.example.com
redmine auth status --format pretty-json
```

Non-interactive setup when the Redmine URL is already known:

```bash
printf '%s\n' "$REDMINE_API_KEY" | redmine auth login --url https://redmine.example.com --stdin
```

Verify stored auth without exposing the key:

```bash
redmine auth status --format pretty-json
```

Remove the stored API key:

```bash
redmine auth logout --format pretty-json
```

Use `REDMINE_API_KEY` or `--api-key` only for one-off overrides. Do not paste key values into final answers.

## HTTPS and Local Test Servers

Authenticated Redmine URLs must use `https://` by default:

```bash
redmine issue list --url https://redmine.example.com --limit 5 --format pretty-json
```

Use `--allow-insecure-http` only for local fake/test servers:

```bash
redmine issue list --url http://127.0.0.1:3000 --allow-insecure-http --limit 5 --format pretty-json
```

Expect a warning because the API key may be sent over cleartext HTTP. Prefer stored login, stdin, or `REDMINE_API_KEY` over literal `--api-key` values.

## Listing

`issue list` defaults to open issues sorted by recently updated:

```bash
redmine issue list --limit 50 --format pretty-json
redmine issue list --project mobile --status open --limit 20 --format pretty-json
redmine issue list --assigned-to @me --status open --format pretty-json
redmine issue list --offset 50 --limit 50 --format pretty-json
```

Use `--all --max-items N` only when the user needs a bounded full scan:

```bash
redmine issue list --project mobile --all --max-items 200 --format pretty-json
```

## Issue Details

```bash
redmine issue get 123 --format pretty-json
redmine issue get 123 --with-notes --format pretty-json
redmine issue get 123 --include journals,attachments,relations,children --format pretty-json
```

`attachments` in `--include` fetches attachment metadata exposed by Redmine. Use `issue attach` to upload a file. File downloads are not supported.

## Wiki Pages and Attachments

```bash
redmine wiki list --project mobile --format pretty-json
redmine wiki get Guide --project mobile --include attachments --format pretty-json
redmine wiki get Guide --project mobile --page-version 3 --format pretty-json
redmine wiki put Guide --project mobile --text-file guide.redmine --comments "Update guide" --page-version 3 --dry-run --format pretty-json
redmine issue attach 123 --file evidence.png --description "Reproduction" --dry-run --format pretty-json
```

Remove `--dry-run` to write. Wiki `--project` accepts an identifier or numeric ID and defaults to `default_project`. `wiki put` creates a missing page or replaces its text. Use `--text` or `--text-file` for Redmine wiki syntax. Pass the version read previously as `--page-version` to protect against stale edits; a mismatch returns `conflict` (exit 7). Without it, the command fetches the current version just before writing. `--parent-title TITLE` changes the parent; an empty title makes a root page. Omission preserves the existing parent. Creation uses the same PUT endpoint as updating and can race with another writer.

`issue attach` accepts one non-empty regular file. `--filename` changes the remote name and `--content-type` overrides the inferred MIME type. Size and permission limits come from Redmine. If the issue update fails after upload, the result reports `upload_completed: true` and `attachment_link_confirmed: false` in `error.details`; inspect the issue before retrying because the original update may have succeeded despite a lost response. Retrying uploads again and can duplicate attachments; unused uploads may remain on the server.

Dry-runs validate local input without server requests; server permissions and automatic wiki version/parent resolution happen only during execution. A successful write with a failed final fetch returns `ok: true` plus a warning. Fetch the result again instead of repeating the write.

## Spent Time

```bash
redmine time activities --format pretty-json
redmine time create --issue 123 --hours 1.5 --activity-id 9 --spent-on 2026-11-05 --comments "Fix login bug" --dry-run --format pretty-json
```

Replace the activity ID with one from the server. Inspect `data.request`, then remove `--dry-run` to register the entry. The issue, positive decimal hours, activity ID, and `YYYY-MM-DD` work date are required. `1.5` hours means 90 minutes. Optional `--comments` or `--comments-file PATH` accepts up to 255 characters including line breaks. A dry-run makes no network requests; permissions, project settings, and server limits are checked when writing.

Registration uses the authenticated user and returns the created entry, including its ID. Activities are fetched live without the metadata cache. This records actual hours and does not change the issue's `estimated_hours`. Each execution creates a new entry; after an uncertain response, check the issue's spent-time entries in Redmine before retrying. The CLI does not automatically retry and cannot yet list, edit, or delete time entries, log to a project without an issue, or log for another user.

## Comments

For short explicit user-provided comments:

```bash
redmine issue comment 123 --message "h3. Done" --format pretty-json
```

For longer Redmine syntax:

```bash
redmine issue comment 123 --message-file result.redmine --format pretty-json
```

## Create

```bash
redmine issue create \
  --subject "Title" \
  --project mobile \
  --tracker Task \
  --priority Normal \
  --description-file body.redmine \
  --dry-run \
  --format pretty-json
```

JSON input is useful for complex payloads:

```bash
redmine issue create --input issue.json --subject "Override title" --dry-run --format pretty-json
```

Flags override JSON input.

## Update

```bash
redmine issue update 123 --set subject="New subject" --message-file note.redmine --dry-run --format pretty-json
redmine issue assign 123 --to @me --message "Taking this." --dry-run --format pretty-json
redmine issue transition 123 --to Confirmed --message-file note.redmine --dry-run --format pretty-json
redmine issue close 123 --message-file resolution.redmine --dry-run --format pretty-json
redmine issue reopen 123 --message "h3. Reopened\n\n* Reason: Reproduced again." --dry-run --format pretty-json
```

Supported semantic `--set` names include `status`, `tracker`, `priority`, `project`, `assignee`, and `assigned_to`. Use `--raw-json` for plugin-specific fields:

```bash
redmine issue update 123 --raw-json '{"custom_fields":[{"id":4,"value":"1.2.3"}]}' --dry-run --format pretty-json
```

## Metadata

Refresh metadata explicitly when name resolution fails:

```bash
redmine meta sync --refresh --format pretty-json
redmine meta statuses --refresh --format pretty-json
redmine meta trackers --project mobile --refresh --format pretty-json
redmine meta priorities --refresh --format pretty-json
redmine meta custom-fields --project mobile --refresh --format pretty-json
```

Use numeric IDs when Redmine metadata is unavailable or ambiguous.

## Table Output

Use table output only for manual inspection:

```bash
redmine issue list --limit 10 --format table
redmine project list --format table
```

Do not parse table output in automation.
