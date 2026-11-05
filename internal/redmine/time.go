package redmine

import (
	"context"
	"net/http"
)

type TimeEntry struct {
	ID        int        `json:"id"`
	Project   Reference  `json:"project"`
	Issue     *Reference `json:"issue,omitempty"`
	User      Reference  `json:"user"`
	Activity  Reference  `json:"activity"`
	Hours     float64    `json:"hours"`
	Comments  string     `json:"comments,omitempty"`
	SpentOn   string     `json:"spent_on"`
	CreatedOn string     `json:"created_on,omitempty"`
	UpdatedOn string     `json:"updated_on,omitempty"`
}

type TimeEntryActivity struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

func (c *Client) CreateTimeEntry(ctx context.Context, entry map[string]any) (TimeEntry, error) {
	var resp struct {
		Entry TimeEntry `json:"time_entry"`
	}
	err := c.Do(ctx, http.MethodPost, "/time_entries.json", nil, map[string]any{"time_entry": entry}, &resp)
	if err == nil && resp.Entry.ID <= 0 {
		err = &APIError{
			Code: "redmine_error", Message: "Redmine returned no time entry ID",
			Hint: "Check the issue's spent-time entries before retrying; the entry may already have been created.",
		}
	}
	return resp.Entry, err
}

func (c *Client) TimeEntryActivities(ctx context.Context) ([]TimeEntryActivity, error) {
	var resp struct {
		Activities []TimeEntryActivity `json:"time_entry_activities"`
	}
	err := c.Do(ctx, http.MethodGet, "/enumerations/time_entry_activities.json", nil, nil, &resp)
	return resp.Activities, err
}
