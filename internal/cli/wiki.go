package cli

import (
	"context"
	"errors"
	"strings"

	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
)

func (a *app) handleWiki(ctx context.Context) output.Envelope {
	project := a.parsed.StringDefault("project", a.cfg.DefaultProject)
	if strings.TrimSpace(project) == "" || project == "." || project == ".." || strings.ContainsAny(project, "/\\") {
		return output.Failure("validation_failed", "Invalid wiki project", "Pass --project with a project identifier or numeric ID, or configure default_project.", nil)
	}
	if a.commandKey() == "wiki list" {
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
	if len(a.parsed.Positionals) != 3 || strings.TrimSpace(title) == "" || title == "." || title == ".." || strings.ContainsAny(title, "/\\") {
		return output.Failure("validation_failed", "Invalid wiki title", "Provide one page title without path separators.", nil)
	}
	version, err := a.parsed.IntDefault("page-version", 0)
	if err != nil || (a.parsed.Flags["page-version"] != nil && version <= 0) {
		return output.Failure("validation_failed", "Invalid page version", "Use --page-version with a positive integer.", nil)
	}
	if a.commandKey() == "wiki get" {
		include := a.parsed.StringDefault("include", "")
		if include != "" && include != "attachments" {
			return output.Failure("validation_failed", "Unsupported wiki include", "Use --include attachments.", nil)
		}
		page, err := a.client.GetWikiPage(ctx, project, title, version, include)
		if err != nil {
			return a.failureFromError(err)
		}
		return a.success(page, nil)
	}
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
	extra := map[string]any{"project": project, "title": title}
	if a.parsed.Bool("dry-run") {
		return a.dryRun("medium", "wiki.put", payload, extra)
	}
	before, err := a.client.GetWikiPage(ctx, project, title, 0, "")
	var apiErr *redmine.APIError
	exists := err == nil
	if err != nil && (!errors.As(err, &apiErr) || apiErr.Code != "not_found" || version > 0) {
		return a.failureFromError(err)
	}
	if exists {
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
