package cli

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"redmine-cli/internal/cache"
	"redmine-cli/internal/output"
	"redmine-cli/internal/redmine"
	"redmine-cli/internal/resolver"
)

func (a *app) ensureMetadata(ctx context.Context, force bool) error {
	if a.parsed.Bool("no-cache") {
		return a.syncMetadata(ctx, true)
	}
	if force || a.parsed.Bool("refresh") {
		return a.syncMetadata(ctx, true)
	}
	if a.metaLoaded && a.meta.Fresh(time.Now(), a.cfg.MetadataTTL) {
		return nil
	}

	meta, loaded, err := cache.LoadMetadata(a.cachePath)
	if err != nil {
		a.warnings = append(a.warnings, "metadata cache could not be read; refreshing")
		a.metaStatus = cache.Status{Metadata: "error"}
		return a.syncMetadata(ctx, true)
	}
	a.meta = meta
	a.metaLoaded = loaded
	a.metaStatus = cache.StatusFor(meta, time.Now(), a.cfg.MetadataTTL, loaded, false, false)
	if loaded && meta.Fresh(time.Now(), a.cfg.MetadataTTL) {
		return nil
	}
	return a.syncMetadata(ctx, true)
}

func (a *app) syncMetadata(ctx context.Context, refreshed bool) error {
	meta := cache.Metadata{SyncedAt: time.Now()}

	statuses, err := a.client.Statuses(ctx)
	if err != nil {
		return err
	}
	meta.Statuses = refsToNamed(statuses)

	trackers, err := a.client.Trackers(ctx)
	if err != nil {
		return err
	}
	meta.Trackers = refsToNamed(trackers)

	priorities, err := a.client.Priorities(ctx)
	if err != nil {
		return err
	}
	meta.Priorities = refsToNamed(priorities)

	customFields, err := a.client.CustomFields(ctx)
	if err != nil && !a.optionalMetadataError("custom field", err) {
		return err
	}
	meta.CustomFields = refsToNamed(customFields)

	projects, err := a.client.ListProjects(ctx, values("limit", "100"))
	if err != nil {
		return err
	}
	for _, project := range projects.Items {
		meta.Projects = append(meta.Projects, cache.NamedID{
			ID:         project.ID,
			Name:       project.Name,
			Identifier: project.Identifier,
		})
	}

	users, err := a.client.SearchUsers(ctx, values("limit", "100"))
	if err == nil {
		for _, user := range users.Items {
			meta.Users = append(meta.Users, cache.NamedID{
				ID:        user.ID,
				Name:      user.FullName(),
				Login:     user.Login,
				Firstname: user.Firstname,
				Lastname:  user.Lastname,
			})
		}
	} else {
		a.warnings = append(a.warnings, "user metadata could not be synced; user names will be resolved via search")
	}

	a.meta = meta
	a.metaLoaded = true
	a.metaStatus = cache.Status{Metadata: "refreshed", Refreshed: refreshed}
	if !a.parsed.Bool("no-cache") {
		if err := cache.SaveMetadata(a.cachePath, meta); err != nil {
			a.warnings = append(a.warnings, "metadata cache could not be written")
		}
	} else {
		a.metaStatus = cache.Status{Metadata: "bypassed", Refreshed: true}
	}
	return nil
}

func (a *app) optionalMetadataError(kind string, err error) bool {
	var apiErr *redmine.APIError
	if errors.As(err, &apiErr) && (apiErr.Code == "permission_denied" || apiErr.Code == "not_found") {
		a.warnings = append(a.warnings, kind+" metadata could not be synced; name resolution for it may require numeric IDs")
		return true
	}
	return false
}

func refsToNamed(refs []redmine.Reference) []cache.NamedID {
	out := make([]cache.NamedID, 0, len(refs))
	for _, ref := range refs {
		out = append(out, cache.NamedID{ID: ref.ID, Name: ref.Name})
	}
	return out
}

func values(kv ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		q.Set(kv[i], kv[i+1])
	}
	return q
}

func (a *app) resolveNamed(ctx context.Context, kind string, value string) (int, output.Envelope) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, output.Failure("validation_failed", "Missing name to resolve", "Provide a name or numeric ID.", map[string]any{"kind": kind})
	}
	if id, err := strconv.Atoi(value); err == nil {
		return id, output.Envelope{}
	}

	if err := a.ensureMetadata(ctx, false); err != nil {
		return 0, a.failureFromError(err)
	}
	id, env := a.resolveNamedFromLoaded(kind, value)
	if env.Error == nil {
		return id, env
	}
	if env.Error.Code != "unknown_name" || a.metaStatus.Refreshed {
		return 0, env
	}

	if err := a.syncMetadata(ctx, true); err != nil {
		return 0, a.failureFromError(err)
	}
	return a.resolveNamedFromLoaded(kind, value)
}

func (a *app) resolveNamedFromLoaded(kind string, value string) (int, output.Envelope) {
	result := resolver.ResolveExact(toResolverItems(a.itemsForKind(kind)), value)
	if result.Err == nil {
		return result.Item.ID, output.Envelope{}
	}
	if errors.Is(result.Err, resolver.ErrAmbiguousName) {
		return 0, output.Failure("ambiguous_name", "Ambiguous "+kind+" name", "Choose one of error.details.candidates or use a numeric ID.", map[string]any{
			"kind":       kind,
			"name":       value,
			"candidates": result.Candidates,
		})
	}
	return 0, output.Failure("unknown_name", "Unknown "+kind+" name", "Run redmine meta sync --refresh or use a numeric ID.", map[string]any{
		"kind": kind,
		"name": value,
	})
}

func (a *app) resolveUser(ctx context.Context, value string) (int, output.Envelope) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, output.Failure("validation_failed", "Missing user value", "Provide @me, a numeric ID, a login, or an exact full name.", nil)
	}
	if id, err := strconv.Atoi(value); err == nil {
		return id, output.Envelope{}
	}
	if value == "@me" || strings.EqualFold(value, "me") {
		user, err := a.client.CurrentUser(ctx)
		if err != nil {
			return 0, a.failureFromError(err)
		}
		return user.ID, output.Envelope{}
	}

	if err := a.ensureMetadata(ctx, false); err == nil {
		result := resolver.ResolveExact(toResolverItems(a.meta.Users), value)
		if result.Err == nil {
			return result.Item.ID, output.Envelope{}
		}
		if errors.Is(result.Err, resolver.ErrAmbiguousName) {
			return 0, output.Failure("ambiguous_name", "Ambiguous user name", "Choose one candidate or use a numeric ID.", map[string]any{
				"kind":       "user",
				"name":       value,
				"candidates": result.Candidates,
			})
		}
	}

	users, err := a.client.SearchUsers(ctx, values("name", value, "limit", "100"))
	if err != nil {
		return 0, a.failureFromError(err)
	}
	var exact []resolver.NamedID
	for _, user := range users.Items {
		item := resolver.NamedID{
			ID:        user.ID,
			Name:      user.FullName(),
			Login:     user.Login,
			Firstname: user.Firstname,
			Lastname:  user.Lastname,
		}
		if resolver.ResolveExact([]resolver.NamedID{item}, value).Err == nil {
			exact = append(exact, item)
		}
	}
	switch len(exact) {
	case 1:
		return exact[0].ID, output.Envelope{}
	case 0:
		return 0, output.Failure("unknown_name", "Unknown user name", "Use @me, a numeric ID, exact login, or exact full name.", map[string]any{"kind": "user", "name": value})
	default:
		return 0, output.Failure("ambiguous_name", "Ambiguous user name", "Choose one candidate or use a numeric ID.", map[string]any{"kind": "user", "name": value, "candidates": exact})
	}
}

func (a *app) itemsForKind(kind string) []cache.NamedID {
	switch kind {
	case "status":
		return a.meta.Statuses
	case "tracker":
		return a.meta.Trackers
	case "priority":
		return a.meta.Priorities
	case "custom_field":
		return a.meta.CustomFields
	case "project":
		return a.meta.Projects
	default:
		return nil
	}
}

func toResolverItems(items []cache.NamedID) []resolver.NamedID {
	out := make([]resolver.NamedID, 0, len(items))
	for _, item := range items {
		out = append(out, resolver.NamedID{
			ID:         item.ID,
			Name:       item.Name,
			Identifier: item.Identifier,
			Login:      item.Login,
			Firstname:  item.Firstname,
			Lastname:   item.Lastname,
		})
	}
	return out
}
