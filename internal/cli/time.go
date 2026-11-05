package cli

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
)

func (a *app) handleTimeActivities(ctx context.Context) output.Envelope {
	if len(a.parsed.Positionals) != 2 {
		return output.Failure("validation_failed", "Unexpected time activities argument", "Use time activities without positional arguments.", nil)
	}
	activities, err := a.client.TimeEntryActivities(ctx)
	if err != nil {
		return a.failureFromError(err)
	}
	if activities == nil {
		activities = []redmine.TimeEntryActivity{}
	}
	return a.success(map[string]any{"activities": activities}, nil)
}

func (a *app) handleTimeCreate(ctx context.Context) output.Envelope {
	if len(a.parsed.Positionals) != 2 {
		return output.Failure("validation_failed", "Unexpected time create argument", "Use time create --issue ID --hours N --activity-id ID --spent-on YYYY-MM-DD.", nil)
	}
	payload := map[string]any{}
	for _, field := range []struct{ flag, key string }{{"issue", "issue_id"}, {"activity-id", "activity_id"}} {
		id, err := strconv.Atoi(a.parsed.StringDefault(field.flag, ""))
		if err != nil || id <= 0 {
			return output.Failure("validation_failed", "Invalid or missing --"+field.flag, "Use --"+field.flag+" with a positive integer.", map[string]any{"field": field.flag})
		}
		payload[field.key] = id
	}
	hours, err := strconv.ParseFloat(a.parsed.StringDefault("hours", ""), 64)
	if err != nil || math.IsNaN(hours) || math.IsInf(hours, 0) || hours <= 0 {
		return output.Failure("validation_failed", "Invalid or missing hours", "Use --hours with a positive number in hours, such as 1.5 for 90 minutes.", nil)
	}
	spentOn := a.parsed.StringDefault("spent-on", "")
	if _, err := time.Parse("2006-01-02", spentOn); err != nil {
		return output.Failure("validation_failed", "Invalid or missing work date", "Use --spent-on YYYY-MM-DD with a valid date.", nil)
	}
	comments, env := readTextFlag(a.parsed.StringDefault("comments", ""), a.parsed.StringDefault("comments-file", ""), "comments")
	if env.Error != nil {
		return env
	}
	if utf8.RuneCountInString(comments) > 255 {
		return output.Failure("validation_failed", "Time entry comments are too long", "Use at most 255 characters for --comments or --comments-file.", nil)
	}
	payload["hours"] = hours
	payload["spent_on"] = spentOn
	if comments != "" {
		payload["comments"] = comments
	}
	if a.parsed.Bool("dry-run") {
		return a.dryRun("low", "time.create", payload, nil)
	}
	entry, err := a.client.CreateTimeEntry(ctx, payload)
	if err != nil {
		var apiErr *redmine.APIError
		if errors.As(err, &apiErr) && (apiErr.Code == "network_error" || apiErr.Code == "timeout" || apiErr.Code == "redmine_error") {
			a.warnings = append(a.warnings, "time entry creation was not confirmed; inspect the issue's spent-time entries before retrying to avoid duplicate entries")
		}
		return a.failureFromError(err)
	}
	return a.success(entry, nil)
}
