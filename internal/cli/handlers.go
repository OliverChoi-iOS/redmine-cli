package cli

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"redmine-cli/internal/config"
	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
)

func (a *app) handleConfigInit() output.Envelope {
	fileCfg, _, err := config.LoadFile(a.configPath)
	if err != nil {
		return output.Failure("config_missing", "Could not load config file", "Fix the config file or pass --config to another file.", map[string]any{"path": a.configPath, "error": err.Error()})
	}
	cfg := config.Config{
		URL:            a.cfg.URL,
		APIKey:         fileCfg.APIKey,
		DefaultProject: a.cfg.DefaultProject,
		MetadataTTL:    a.cfg.MetadataTTL,
		DefaultFormat:  a.cfg.DefaultFormat,
		AppendFooter:   a.cfg.AppendFooter,
		CloseStatus:    a.cfg.CloseStatus,
		ReopenStatus:   a.cfg.ReopenStatus,
	}
	if cfg.URL == "" {
		return output.Failure("validation_failed", "Missing Redmine URL", "Pass --url or set REDMINE_URL before running config init.", nil)
	}
	if _, err := redmine.New(cfg.URL, cfg.APIKey, a.redmineClientOptions()...).NewRequest("GET", "/"); err != nil {
		return output.Failure("validation_failed", "Invalid Redmine URL", "Use an https:// Redmine URL, or pass --allow-insecure-http for a one-off local HTTP config init.", map[string]any{"error": err.Error()})
	}
	if err := config.WriteFile(a.configPath, cfg); err != nil {
		return output.Failure("config_missing", "Could not write config file", "Check the config directory permissions or pass --config.", map[string]any{"path": a.configPath, "error": err.Error()})
	}
	return a.success(map[string]any{
		"path":             a.configPath,
		"url":              cfg.URL,
		"default_project":  cfg.DefaultProject,
		"api_key_stored":   cfg.APIKey != "",
		"api_key_guidance": "Run redmine auth login to store an API key, or set REDMINE_API_KEY for one-off commands.",
	}, nil)
}

func (a *app) handleAuthStatus() output.Envelope {
	data := map[string]any{
		"url_configured":  a.cfg.URL != "",
		"api_key_present": a.cfg.APIKey != "",
		"api_key_source":  a.apiKeySource,
		"authenticated":   false,
	}
	if a.cfg.URL == "" || a.cfg.APIKey == "" {
		return a.success(data, nil)
	}
	user, err := a.client.CurrentUser(context.Background())
	if err != nil {
		return a.failureFromError(err)
	}
	data["authenticated"] = true
	data["user"] = user
	return a.success(data, nil)
}

func (a *app) handleIssueList(ctx context.Context) output.Envelope {
	limit, env := a.listLimit(50)
	if env.Error != nil {
		return env
	}
	offset, err := a.parsed.IntDefault("offset", 0)
	if err != nil {
		return output.Failure("validation_failed", "Invalid offset", err.Error(), nil)
	}
	maxItems, err := a.parsed.IntDefault("max-items", defaultAllMaxItems)
	if err != nil {
		return output.Failure("validation_failed", "Invalid max-items", err.Error(), nil)
	}

	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	q.Set("sort", a.parsed.StringDefault("sort", "updated_on:desc"))

	status := a.parsed.StringDefault("status", "open")
	if raw := a.parsed.StringDefault("status-id", ""); raw != "" {
		q.Set("status_id", raw)
	} else if status != "" && isRawStatusValue(status) {
		q.Set("status_id", status)
	} else if status != "" {
		id, env := a.resolveNamed(ctx, "status", status)
		if env.Error != nil {
			return env
		}
		q.Set("status_id", strconv.Itoa(id))
	}
	if env := a.applyIssueListFilters(ctx, q); env.Error != nil {
		return env
	}
	for _, raw := range a.parsed.Values("param") {
		key, value, err := parseParam(raw)
		if err != nil {
			return output.Failure("validation_failed", "Invalid --param", err.Error(), map[string]any{"value": raw})
		}
		q.Add(key, value)
	}

	if !a.parsed.Bool("all") {
		result, err := a.client.ListIssues(ctx, q)
		if err != nil {
			return a.failureFromError(err)
		}
		return a.success(map[string]any{"issues": result.Items}, pagination(result.Limit, result.Offset, result.TotalCount, len(result.Items), false, maxItems))
	}

	all, page, env := a.collectIssues(ctx, q, offset, limit, maxItems)
	if env.Error != nil {
		return env
	}
	return a.success(map[string]any{"issues": all}, page)
}

func (a *app) applyIssueListFilters(ctx context.Context, q url.Values) output.Envelope {
	if raw := a.parsed.StringDefault("project-id", ""); raw != "" {
		q.Set("project_id", raw)
	} else if raw := a.parsed.StringDefault("project", ""); raw != "" {
		id, env := a.resolveNamed(ctx, "project", raw)
		if env.Error != nil {
			return env
		}
		q.Set("project_id", strconv.Itoa(id))
	}
	if raw := a.parsed.StringDefault("tracker-id", ""); raw != "" {
		q.Set("tracker_id", raw)
	} else if raw := a.parsed.StringDefault("tracker", ""); raw != "" {
		id, env := a.resolveNamed(ctx, "tracker", raw)
		if env.Error != nil {
			return env
		}
		q.Set("tracker_id", strconv.Itoa(id))
	}
	if raw := a.parsed.StringDefault("priority-id", ""); raw != "" {
		q.Set("priority_id", raw)
	} else if raw := a.parsed.StringDefault("priority", ""); raw != "" {
		id, env := a.resolveNamed(ctx, "priority", raw)
		if env.Error != nil {
			return env
		}
		q.Set("priority_id", strconv.Itoa(id))
	}
	if raw := firstNonEmpty(a.parsed.StringDefault("assigned-to-id", ""), a.parsed.StringDefault("assignee", ""), a.parsed.StringDefault("assigned-to", "")); raw != "" {
		if a.parsed.StringDefault("assigned-to-id", "") != "" {
			q.Set("assigned_to_id", raw)
		} else {
			id, env := a.resolveUser(ctx, raw)
			if env.Error != nil {
				return env
			}
			q.Set("assigned_to_id", strconv.Itoa(id))
		}
	}
	if raw := a.parsed.StringDefault("author-id", ""); raw != "" {
		q.Set("author_id", raw)
	} else if raw := a.parsed.StringDefault("author", ""); raw != "" {
		id, env := a.resolveUser(ctx, raw)
		if env.Error != nil {
			return env
		}
		q.Set("author_id", strconv.Itoa(id))
	}
	if raw := a.parsed.StringDefault("updated-after", ""); raw != "" {
		q.Set("updated_on", ">="+raw)
	}
	if raw := a.parsed.StringDefault("created-after", ""); raw != "" {
		q.Set("created_on", ">="+raw)
	}
	if raw := a.parsed.StringDefault("query", ""); raw != "" {
		q.Set("query_id", raw)
	}
	return output.Envelope{}
}

func (a *app) collectIssues(ctx context.Context, q url.Values, offset int, limit int, maxItems int) ([]redmine.Issue, *output.Pagination, output.Envelope) {
	var all []redmine.Issue
	total := 0
	truncated := false
	for len(all) < maxItems {
		q.Set("offset", strconv.Itoa(offset+len(all)))
		q.Set("limit", strconv.Itoa(limit))
		result, err := a.client.ListIssues(ctx, q)
		if err != nil {
			return nil, nil, a.failureFromError(err)
		}
		total = result.TotalCount
		all = append(all, result.Items...)
		if len(result.Items) == 0 || result.Offset+len(result.Items) >= result.TotalCount {
			break
		}
		if len(all) >= maxItems {
			truncated = true
			all = all[:maxItems]
			break
		}
	}
	page := &output.Pagination{
		Limit:      limit,
		Offset:     offset,
		TotalCount: total,
		Returned:   len(all),
		HasMore:    offset+len(all) < total,
		NextOffset: offset + len(all),
		Truncated:  truncated || offset+len(all) < total,
	}
	if page.Truncated {
		page.Hint = "Use --offset or increase --max-items to fetch more results."
	}
	return all, page, output.Envelope{}
}

func (a *app) handleIssueGet(ctx context.Context) output.Envelope {
	id, env := a.requireID(2, "issue id")
	if env.Error != nil {
		return env
	}
	include := a.parsed.StringDefault("include", "")
	if a.parsed.Bool("full") {
		include = "journals,attachments,relations,children"
	}
	if a.parsed.Bool("with-notes") {
		include = addInclude(include, "journals")
	}
	issue, err := a.client.GetIssue(ctx, id, include)
	if err != nil {
		return a.failureFromError(err)
	}
	return a.success(issue, nil)
}

func (a *app) handleProjectList(ctx context.Context) output.Envelope {
	limit, env := a.listLimit(50)
	if env.Error != nil {
		return env
	}
	offset, err := a.parsed.IntDefault("offset", 0)
	if err != nil {
		return output.Failure("validation_failed", "Invalid offset", err.Error(), nil)
	}
	maxItems, err := a.parsed.IntDefault("max-items", defaultAllMaxItems)
	if err != nil {
		return output.Failure("validation_failed", "Invalid max-items", err.Error(), nil)
	}
	q := values("limit", strconv.Itoa(limit), "offset", strconv.Itoa(offset))
	if !a.parsed.Bool("all") {
		result, err := a.client.ListProjects(ctx, q)
		if err != nil {
			return a.failureFromError(err)
		}
		return a.success(map[string]any{"projects": result.Items}, pagination(result.Limit, result.Offset, result.TotalCount, len(result.Items), false, maxItems))
	}
	var all []redmine.Project
	total := 0
	for len(all) < maxItems {
		q.Set("offset", strconv.Itoa(offset+len(all)))
		result, err := a.client.ListProjects(ctx, q)
		if err != nil {
			return a.failureFromError(err)
		}
		total = result.TotalCount
		all = append(all, result.Items...)
		if len(result.Items) == 0 || result.Offset+len(result.Items) >= result.TotalCount {
			break
		}
	}
	return a.success(map[string]any{"projects": all}, pagination(limit, offset, total, len(all), offset+len(all) < total, maxItems))
}

func (a *app) handleProjectGet(ctx context.Context) output.Envelope {
	raw := a.arg(2)
	if raw == "" {
		return output.Failure("validation_failed", "Missing project identifier", "Provide a project identifier or numeric ID.", nil)
	}
	project, err := a.client.GetProject(ctx, raw)
	if err != nil {
		return a.failureFromError(err)
	}
	return a.success(project, nil)
}

func (a *app) handleUserMe(ctx context.Context) output.Envelope {
	user, err := a.client.CurrentUser(ctx)
	if err != nil {
		return a.failureFromError(err)
	}
	return a.success(user, nil)
}

func (a *app) handleUserSearch(ctx context.Context) output.Envelope {
	query := a.arg(2)
	if query == "" {
		return output.Failure("validation_failed", "Missing user search query", "Provide a login, name, or email query.", nil)
	}
	limit, env := a.listLimit(50)
	if env.Error != nil {
		return env
	}
	offset, err := a.parsed.IntDefault("offset", 0)
	if err != nil {
		return output.Failure("validation_failed", "Invalid offset", err.Error(), nil)
	}
	result, err := a.client.SearchUsers(ctx, values("name", query, "limit", strconv.Itoa(limit), "offset", strconv.Itoa(offset)))
	if err != nil {
		return a.failureFromError(err)
	}
	return a.success(map[string]any{"users": result.Items}, pagination(result.Limit, result.Offset, result.TotalCount, len(result.Items), false, defaultAllMaxItems))
}

func (a *app) handleMetaSync(ctx context.Context) output.Envelope {
	if err := a.syncMetadata(ctx, true); err != nil {
		return a.failureFromError(err)
	}
	return a.success(a.meta, nil)
}

func (a *app) handleMetaSlice(ctx context.Context, name string) output.Envelope {
	if err := a.ensureMetadata(ctx, a.parsed.Bool("refresh")); err != nil {
		return a.failureFromError(err)
	}
	switch name {
	case "statuses":
		return a.success(map[string]any{"statuses": a.meta.Statuses}, nil)
	case "trackers":
		return a.success(map[string]any{"trackers": a.meta.Trackers}, nil)
	case "priorities":
		return a.success(map[string]any{"priorities": a.meta.Priorities}, nil)
	case "custom_fields":
		return a.success(map[string]any{"custom_fields": a.meta.CustomFields}, nil)
	default:
		return output.Failure("validation_failed", "Unknown metadata slice", "Use statuses, trackers, priorities, or custom-fields.", nil)
	}
}

func (a *app) handleIssueCreate(ctx context.Context) output.Envelope {
	payload, env := readJSONInput(a.parsed.StringDefault("input", ""))
	if env.Error != nil {
		return env
	}
	if env := mergeRawJSON(payload, a.parsed.StringDefault("raw-json", "")); env.Error != nil {
		return env
	}
	if env := a.applyIssueCreateFlags(ctx, payload); env.Error != nil {
		return env
	}
	if strings.TrimSpace(stringValue(payload["subject"])) == "" {
		return output.Failure("validation_failed", "Issue subject is required", "Pass --subject or include subject in --input JSON.", map[string]any{"field": "subject"})
	}
	if payload["project_id"] == nil && strings.TrimSpace(a.cfg.DefaultProject) != "" {
		id, env := a.resolveNamed(ctx, "project", a.cfg.DefaultProject)
		if env.Error != nil {
			return env
		}
		payload["project_id"] = id
	}
	if payload["project_id"] == nil {
		return output.Failure("validation_failed", "Issue project is required", "Pass --project or configure default_project.", map[string]any{"field": "project"})
	}
	if a.parsed.Bool("dry-run") {
		return a.dryRun("low", "issue.create", payload, nil)
	}
	issue, err := a.client.CreateIssue(ctx, payload)
	if err != nil {
		return a.failureFromError(err)
	}
	return a.success(issue, nil)
}

func (a *app) applyIssueCreateFlags(ctx context.Context, payload map[string]any) output.Envelope {
	if raw := a.parsed.StringDefault("subject", ""); raw != "" {
		payload["subject"] = raw
	}
	desc, env := readTextFlag(a.parsed.StringDefault("description", ""), a.parsed.StringDefault("description-file", ""), "description")
	if env.Error != nil {
		return env
	}
	if desc != "" {
		payload["description"] = desc
	}
	if raw := a.parsed.StringDefault("project-id", ""); raw != "" {
		payload["project_id"] = raw
	} else if raw := a.parsed.StringDefault("project", ""); raw != "" {
		id, env := a.resolveNamed(ctx, "project", raw)
		if env.Error != nil {
			return env
		}
		payload["project_id"] = id
	}
	if raw := a.parsed.StringDefault("tracker-id", ""); raw != "" {
		payload["tracker_id"] = raw
	} else if raw := a.parsed.StringDefault("tracker", ""); raw != "" {
		id, env := a.resolveNamed(ctx, "tracker", raw)
		if env.Error != nil {
			return env
		}
		payload["tracker_id"] = id
	}
	if raw := a.parsed.StringDefault("priority-id", ""); raw != "" {
		payload["priority_id"] = raw
	} else if raw := a.parsed.StringDefault("priority", ""); raw != "" {
		id, env := a.resolveNamed(ctx, "priority", raw)
		if env.Error != nil {
			return env
		}
		payload["priority_id"] = id
	}
	if raw := a.parsed.StringDefault("status-id", ""); raw != "" {
		payload["status_id"] = raw
	} else if raw := a.parsed.StringDefault("status", ""); raw != "" {
		id, env := a.resolveNamed(ctx, "status", raw)
		if env.Error != nil {
			return env
		}
		payload["status_id"] = id
	}
	if raw := firstNonEmpty(a.parsed.StringDefault("assigned-to-id", ""), a.parsed.StringDefault("assignee", ""), a.parsed.StringDefault("assigned-to", "")); raw != "" {
		if a.parsed.StringDefault("assigned-to-id", "") != "" {
			payload["assigned_to_id"] = raw
		} else {
			id, env := a.resolveUser(ctx, raw)
			if env.Error != nil {
				return env
			}
			payload["assigned_to_id"] = id
		}
	}
	return a.applyCustomFieldFlags(ctx, payload)
}

func (a *app) handleIssueComment(ctx context.Context) output.Envelope {
	id, env := a.requireID(2, "issue id")
	if env.Error != nil {
		return env
	}
	message, env := readTextFlag(a.parsed.StringDefault("message", ""), a.parsed.StringDefault("message-file", ""), "message")
	if env.Error != nil {
		return env
	}
	if message == "" {
		return output.Failure("validation_failed", "Comment message is required", "Pass --message or --message-file.", map[string]any{"field": "message"})
	}
	payload := map[string]any{"notes": message}
	if a.parsed.Bool("dry-run") {
		return a.dryRun("low", "issue.comment", payload, map[string]any{"issue_id": id})
	}
	if err := a.client.UpdateIssue(ctx, id, payload); err != nil {
		return a.failureFromError(err)
	}
	return a.success(map[string]any{"issue_id": id, "commented": true, "notes_length": len(message)}, nil)
}

func (a *app) handleIssueUpdate(ctx context.Context) output.Envelope {
	id, env := a.requireID(2, "issue id")
	if env.Error != nil {
		return env
	}
	payload, env := readJSONInput(a.parsed.StringDefault("input", ""))
	if env.Error != nil {
		return env
	}
	if env := mergeRawJSON(payload, a.parsed.StringDefault("raw-json", "")); env.Error != nil {
		return env
	}
	for _, raw := range a.parsed.Values("set") {
		if env := applySet(payload, raw); env.Error != nil {
			return env
		}
	}
	if env := a.applySemanticSetFlags(ctx, payload); env.Error != nil {
		return env
	}
	if env := a.applyCustomFieldFlags(ctx, payload); env.Error != nil {
		return env
	}
	message, env := readTextFlag(a.parsed.StringDefault("message", ""), a.parsed.StringDefault("message-file", ""), "message")
	if env.Error != nil {
		return env
	}
	if message != "" {
		payload["notes"] = message
	}
	if len(payload) == 0 {
		return output.Failure("validation_failed", "No update fields were provided", "Use --set, --input, --raw-json, custom fields, or --message.", nil)
	}
	return a.updateIssueWithBeforeAfter(ctx, id, payload, "issue.update")
}

func (a *app) applySemanticSetFlags(ctx context.Context, payload map[string]any) output.Envelope {
	for _, raw := range a.parsed.Values("set") {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "status":
			id, env := a.resolveNamed(ctx, "status", value)
			if env.Error != nil {
				return env
			}
			payload["status_id"] = id
			delete(payload, "status")
		case "tracker":
			id, env := a.resolveNamed(ctx, "tracker", value)
			if env.Error != nil {
				return env
			}
			payload["tracker_id"] = id
			delete(payload, "tracker")
		case "priority":
			id, env := a.resolveNamed(ctx, "priority", value)
			if env.Error != nil {
				return env
			}
			payload["priority_id"] = id
			delete(payload, "priority")
		case "project":
			id, env := a.resolveNamed(ctx, "project", value)
			if env.Error != nil {
				return env
			}
			payload["project_id"] = id
			delete(payload, "project")
		case "assignee", "assigned_to":
			id, env := a.resolveUser(ctx, value)
			if env.Error != nil {
				return env
			}
			payload["assigned_to_id"] = id
			delete(payload, key)
		}
	}
	return output.Envelope{}
}

func (a *app) handleIssueAssign(ctx context.Context) output.Envelope {
	id, env := a.requireID(2, "issue id")
	if env.Error != nil {
		return env
	}
	to := a.parsed.StringDefault("to", "")
	if to == "" {
		return output.Failure("validation_failed", "Assignment target is required", "Pass --to @me, a numeric ID, exact login, or exact full name.", nil)
	}
	userID, env := a.resolveUser(ctx, to)
	if env.Error != nil {
		return env
	}
	payload := map[string]any{"assigned_to_id": userID}
	message, env := readTextFlag(a.parsed.StringDefault("message", ""), a.parsed.StringDefault("message-file", ""), "message")
	if env.Error != nil {
		return env
	}
	if message != "" {
		payload["notes"] = message
	}
	return a.updateIssueWithBeforeAfter(ctx, id, payload, "issue.assign")
}

func (a *app) handleIssueTransition(ctx context.Context) output.Envelope {
	id, env := a.requireID(2, "issue id")
	if env.Error != nil {
		return env
	}
	to := a.parsed.StringDefault("to", "")
	if to == "" {
		return output.Failure("validation_failed", "Transition status is required", "Pass --to <status>.", nil)
	}
	statusID, env := a.resolveNamed(ctx, "status", to)
	if env.Error != nil {
		return env
	}
	payload := map[string]any{"status_id": statusID}
	message, env := readTextFlag(a.parsed.StringDefault("message", ""), a.parsed.StringDefault("message-file", ""), "message")
	if env.Error != nil {
		return env
	}
	if message != "" {
		payload["notes"] = message
	}
	return a.updateIssueWithBeforeAfter(ctx, id, payload, "issue.transition")
}

func (a *app) handleIssueClose(ctx context.Context) output.Envelope {
	if a.parsed.StringDefault("to", "") == "" {
		a.parsed.Flags["to"] = []string{a.cfg.CloseStatus}
	}
	return a.handleIssueTransitionWithOperation(ctx, "issue.close")
}

func (a *app) handleIssueReopen(ctx context.Context) output.Envelope {
	if a.parsed.StringDefault("to", "") == "" {
		a.parsed.Flags["to"] = []string{a.cfg.ReopenStatus}
	}
	return a.handleIssueTransitionWithOperation(ctx, "issue.reopen")
}

func (a *app) handleIssueTransitionWithOperation(ctx context.Context, op string) output.Envelope {
	id, env := a.requireID(2, "issue id")
	if env.Error != nil {
		return env
	}
	statusID, env := a.resolveNamed(ctx, "status", a.parsed.StringDefault("to", ""))
	if env.Error != nil {
		return env
	}
	payload := map[string]any{"status_id": statusID}
	message, env := readTextFlag(a.parsed.StringDefault("message", ""), a.parsed.StringDefault("message-file", ""), "message")
	if env.Error != nil {
		return env
	}
	if message != "" {
		payload["notes"] = message
	}
	return a.updateIssueWithBeforeAfter(ctx, id, payload, op)
}

func (a *app) handleIssueLink(ctx context.Context) output.Envelope {
	id, env := a.requireID(2, "issue id")
	if env.Error != nil {
		return env
	}
	target, env := a.requireID(3, "target issue id")
	if env.Error != nil {
		return env
	}
	relationType := a.parsed.StringDefault("relation", "relates")
	payload := map[string]any{"issue_to_id": target, "relation_type": relationType}
	if a.parsed.Bool("dry-run") {
		return a.dryRun("low", "issue.link", payload, map[string]any{"issue_id": id, "target_issue_id": target})
	}
	relation, err := a.client.CreateRelation(ctx, id, payload)
	if err != nil {
		return a.failureFromError(err)
	}
	return a.success(relation, nil)
}

func (a *app) updateIssueWithBeforeAfter(ctx context.Context, id int, payload map[string]any, op string) output.Envelope {
	if a.parsed.Bool("dry-run") {
		return a.dryRun("medium", op, payload, map[string]any{"issue_id": id})
	}
	before, beforeErr := a.client.GetIssue(ctx, id, "")
	if beforeErr != nil {
		return a.failureFromError(beforeErr)
	}
	if err := a.client.UpdateIssue(ctx, id, payload); err != nil {
		return a.failureFromError(err)
	}
	after, afterErr := a.client.GetIssue(ctx, id, "")
	if afterErr != nil {
		a.warnings = append(a.warnings, "issue was updated but after-state fetch failed")
		return a.success(map[string]any{"issue_id": id, "updated": true, "before": before, "request": payload}, nil)
	}
	return a.success(map[string]any{"issue_id": id, "updated": true, "before": before, "after": after}, nil)
}

func (a *app) applyCustomFieldFlags(ctx context.Context, payload map[string]any) output.Envelope {
	for _, raw := range a.parsed.Values("custom-field-id") {
		key, value, env := parseKeyValue(raw, "--custom-field-id")
		if env.Error != nil {
			return env
		}
		id, err := strconv.Atoi(key)
		if err != nil {
			return output.Failure("validation_failed", "Invalid custom field ID", "Use --custom-field-id ID=Value.", map[string]any{"id": key})
		}
		appendCustomField(payload, id, parseCustomValue(value))
	}
	for _, raw := range a.parsed.Values("custom-field") {
		key, value, env := parseKeyValue(raw, "--custom-field")
		if env.Error != nil {
			return env
		}
		id, env := a.resolveNamed(ctx, "custom_field", key)
		if env.Error != nil {
			return env
		}
		appendCustomField(payload, id, parseCustomValue(value))
	}
	return output.Envelope{}
}

func (a *app) dryRun(risk string, operation string, payload map[string]any, extra map[string]any) output.Envelope {
	data := map[string]any{
		"operation": operation,
		"request":   payload,
	}
	for key, value := range extra {
		data[key] = value
	}
	env := a.success(data, nil)
	dry := true
	env.DryRun = &dry
	env.Meta["operation"] = map[string]any{"risk": risk, "dry_run": true}
	return env
}

func (a *app) listLimit(def int) (int, output.Envelope) {
	limit, err := a.parsed.IntDefault("limit", def)
	if err != nil {
		return 0, output.Failure("validation_failed", "Invalid limit", err.Error(), nil)
	}
	if limit <= 0 {
		return 0, output.Failure("validation_failed", "Invalid limit", "Use a positive integer limit.", map[string]any{"limit": limit})
	}
	return limit, output.Envelope{}
}

func pagination(limit int, offset int, total int, returned int, truncated bool, maxItems int) *output.Pagination {
	next := offset + returned
	hasMore := next < total
	page := &output.Pagination{
		Limit:      limit,
		Offset:     offset,
		TotalCount: total,
		NextOffset: next,
		HasMore:    hasMore,
		Returned:   returned,
		Truncated:  truncated || returned >= maxItems && hasMore,
	}
	if page.Truncated || page.HasMore {
		page.Hint = "Use --offset or --all with --max-items to fetch more results."
	}
	return page
}

func addInclude(include string, value string) string {
	if include == "" {
		return value
	}
	parts := strings.Split(include, ",")
	for _, part := range parts {
		if strings.TrimSpace(part) == value {
			return include
		}
	}
	return include + "," + value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func isRawStatusValue(value string) bool {
	if value == "open" || value == "*" || value == "closed" {
		return true
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
