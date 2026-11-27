package cli

import (
	"context"
	"errors"
	"strings"

	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
)

// wikiWriteMode controls how a wiki write treats the page's existence.
type wikiWriteMode int

const (
	wikiUpsert    wikiWriteMode = iota // wiki put: create when missing, update when present
	wikiCreateNew                      // wiki create: fail when the page already exists
	wikiUpdateOld                      // wiki update: fail when the page does not exist
)

func (a *app) handleWiki(ctx context.Context) output.Envelope {
	project := a.parsed.StringDefault("project", a.cfg.DefaultProject)
	if strings.TrimSpace(project) == "" || project == "." || project == ".." || strings.ContainsAny(project, "/\\") {
		return output.Failure("validation_failed", "Invalid wiki project", "Pass --project with a project identifier or numeric ID, or configure default_project.", nil)
	}
	command := a.commandKey()
	if command == "wiki list" {
		if len(a.parsed.Positionals) != 2 {
			return output.Failure("validation_failed", "Unexpected wiki list argument", "Use wiki list --project IDENTIFIER_OR_ID.", nil)
		}
		pages, err := a.client.ListWikiPages(ctx, project)
		if err != nil {
			return a.failureFromError(err)
		}
		if pages == nil {
			pages = []redmine.WikiPage{}
		}
		return a.success(map[string]any{"wiki_pages": pages}, nil)
	}
	title := a.arg(2)
	if len(a.parsed.Positionals) != 3 || !validWikiTitle(title) {
		return output.Failure("validation_failed", "Invalid wiki title", "Provide one page title without path separators.", nil)
	}
	version, err := a.parsed.IntDefault("page-version", 0)
	if err != nil || (a.parsed.Flags["page-version"] != nil && version <= 0) {
		return output.Failure("validation_failed", "Invalid page version", "Use --page-version with a positive integer.", nil)
	}
	switch command {
	case "wiki get":
		include := a.parsed.StringDefault("include", "")
		if include != "" && include != "attachments" {
			return output.Failure("validation_failed", "Unsupported wiki include", "Use --include attachments.", nil)
		}
		page, err := a.client.GetWikiPage(ctx, project, title, version, include)
		if err != nil {
			return a.failureFromError(err)
		}
		return a.success(page, nil)
	case "wiki delete":
		return a.handleWikiDelete(ctx, project, title, version)
	case "wiki create":
		return a.handleWikiWrite(ctx, project, title, version, wikiCreateNew)
	case "wiki update":
		return a.handleWikiWrite(ctx, project, title, version, wikiUpdateOld)
	default:
		return a.handleWikiWrite(ctx, project, title, version, wikiUpsert)
	}
}

func validWikiTitle(title string) bool {
	return strings.TrimSpace(title) != "" && title != "." && title != ".." && !strings.ContainsAny(title, "/\\")
}

func (a *app) handleWikiWrite(ctx context.Context, project, title string, version int, mode wikiWriteMode) output.Envelope {
	text, env := readTextFlag(a.parsed.StringDefault("text", ""), a.parsed.StringDefault("text-file", ""), "text")
	if env.Error != nil {
		return env
	}
	if strings.TrimSpace(text) == "" {
		return output.Failure("validation_failed", "Wiki text is required", "Pass --text or --text-file containing Redmine wiki syntax.", nil)
	}
	payload := map[string]any{"text": text}
	for flag, field := range map[string]string{"comments": "comments", "parent-title": "parent_title"} {
		if a.parsed.Flags[flag] != nil {
			payload[field] = a.parsed.StringDefault(flag, "")
		}
	}
	if version > 0 {
		payload["version"] = version
	}
	operation, risk := "wiki.put", "medium"
	switch mode {
	case wikiCreateNew:
		operation, risk = "wiki.create", "low"
	case wikiUpdateOld:
		operation = "wiki.update"
	}
	extra := map[string]any{"project": project, "title": title}
	if a.parsed.Bool("dry-run") {
		return a.dryRun(risk, operation, payload, extra)
	}
	before, err := a.client.GetWikiPage(ctx, project, title, 0, "")
	var apiErr *redmine.APIError
	exists := err == nil
	if err != nil {
		missing := errors.As(err, &apiErr) && apiErr.Code == "not_found"
		switch {
		case !missing:
			return a.failureFromError(err)
		case mode == wikiUpdateOld:
			return output.Failure("not_found", "Wiki page does not exist", "Use wiki create to add a new page, or check the title and project.", map[string]any{"project": project, "title": title})
		case version > 0:
			return a.failureFromError(err)
		}
	}
	if exists {
		if mode == wikiCreateNew {
			return output.Failure("conflict", "Wiki page already exists", "Use wiki update to change an existing page, or choose another title.", map[string]any{"project": project, "title": title, "version": before.Version})
		}
		if version == 0 {
			if before.Version <= 0 {
				return output.Failure("redmine_error", "Wiki response has no valid version", "Fetch the page version before updating it.", nil)
			}
			payload["version"] = before.Version
		}
		if a.parsed.Flags["parent-title"] == nil && before.Parent != nil {
			payload["parent_title"] = before.Parent.Title
		}
	}
	if err := a.client.PutWikiPage(ctx, project, title, payload); err != nil {
		return a.failureFromError(err)
	}
	extra["saved"] = true
	extra["created"] = !exists
	if exists {
		extra["before"] = before
	}
	after, err := a.client.GetWikiPage(ctx, project, title, 0, "")
	if err != nil {
		a.warnings = append(a.warnings, "wiki page was saved but after-state fetch failed")
	} else {
		extra["after"] = after
	}
	return a.success(extra, nil)
}

func (a *app) handleWikiDelete(ctx context.Context, project, title string, version int) output.Envelope {
	extra := map[string]any{"project": project, "title": title}
	request := map[string]any{}
	if version > 0 {
		request["expected_version"] = version
	}
	if a.parsed.Bool("dry-run") {
		extra["confirm_required"] = title
		return a.dryRun("high", "wiki.delete", request, extra)
	}
	if a.parsed.StringDefault("confirm", "") != title {
		return output.Failure("validation_failed", "Wiki delete requires confirmation", "Pass --confirm with the exact page title. Deletion removes the page, its history, and its attachments; child pages become root pages.", map[string]any{"project": project, "title": title})
	}
	before, err := a.client.GetWikiPage(ctx, project, title, 0, "attachments")
	if err != nil {
		return a.failureFromError(err)
	}
	if version > 0 && before.Version != version {
		return output.Failure("conflict", "Wiki page changed since it was read", "Re-read the page and confirm it should still be deleted.", map[string]any{"project": project, "title": title, "expected_version": version, "current_version": before.Version})
	}
	if err := a.client.DeleteWikiPage(ctx, project, title); err != nil {
		return a.failureFromError(err)
	}
	extra["deleted"] = true
	extra["before"] = before
	return a.success(extra, nil)
}
