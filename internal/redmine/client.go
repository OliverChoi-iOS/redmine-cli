package redmine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultMaxResponseBytes int64 = 10 * 1024 * 1024

var errResponseTooLarge = errors.New("redmine response body is too large")

type Client struct {
	BaseURL           string
	APIKey            string
	HTTP              *http.Client
	AllowInsecureHTTP bool
	MaxResponseBytes  int64
}

type Option func(*Client)

func WithAllowInsecureHTTP() Option {
	return func(c *Client) {
		c.AllowInsecureHTTP = true
	}
}

func WithMaxResponseBytes(limit int64) Option {
	return func(c *Client) {
		c.MaxResponseBytes = limit
	}
}

type Reference struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
}

type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name,omitempty"`
	Identifier  string `json:"identifier,omitempty"`
	Description string `json:"description,omitempty"`
	Status      int    `json:"status,omitempty"`
	CreatedOn   string `json:"created_on,omitempty"`
	UpdatedOn   string `json:"updated_on,omitempty"`
}

type User struct {
	ID          int    `json:"id"`
	Login       string `json:"login,omitempty"`
	Firstname   string `json:"firstname,omitempty"`
	Lastname    string `json:"lastname,omitempty"`
	Mail        string `json:"mail,omitempty"`
	CreatedOn   string `json:"created_on,omitempty"`
	LastLoginOn string `json:"last_login_on,omitempty"`
}

func (u User) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(u.Firstname) + " " + strings.TrimSpace(u.Lastname))
}

type CustomFieldValue struct {
	ID       int    `json:"id"`
	Name     string `json:"name,omitempty"`
	Value    any    `json:"value,omitempty"`
	Multiple bool   `json:"multiple,omitempty"`
}

type Journal struct {
	ID        int         `json:"id"`
	User      Reference   `json:"user,omitempty"`
	Notes     string      `json:"notes,omitempty"`
	CreatedOn string      `json:"created_on,omitempty"`
	Details   []any       `json:"details,omitempty"`
	Raw       interface{} `json:"-"`
}

type Attachment struct {
	ID          int       `json:"id"`
	Filename    string    `json:"filename,omitempty"`
	Filesize    int       `json:"filesize,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	Description string    `json:"description,omitempty"`
	ContentURL  string    `json:"content_url,omitempty"`
	Author      Reference `json:"author,omitempty"`
	CreatedOn   string    `json:"created_on,omitempty"`
}

type Relation struct {
	ID           int    `json:"id"`
	IssueID      int    `json:"issue_id,omitempty"`
	IssueToID    int    `json:"issue_to_id,omitempty"`
	RelationType string `json:"relation_type,omitempty"`
	Delay        int    `json:"delay,omitempty"`
}

type Issue struct {
	ID             int                `json:"id"`
	Project        Reference          `json:"project,omitempty"`
	Tracker        Reference          `json:"tracker,omitempty"`
	Status         Reference          `json:"status,omitempty"`
	Priority       Reference          `json:"priority,omitempty"`
	Author         Reference          `json:"author,omitempty"`
	AssignedTo     *Reference         `json:"assigned_to,omitempty"`
	Parent         *Reference         `json:"parent,omitempty"`
	Subject        string             `json:"subject,omitempty"`
	Description    string             `json:"description,omitempty"`
	StartDate      string             `json:"start_date,omitempty"`
	DueDate        string             `json:"due_date,omitempty"`
	DoneRatio      int                `json:"done_ratio,omitempty"`
	IsPrivate      bool               `json:"is_private,omitempty"`
	EstimatedHours float64            `json:"estimated_hours,omitempty"`
	SpentHours     float64            `json:"spent_hours,omitempty"`
	CustomFields   []CustomFieldValue `json:"custom_fields,omitempty"`
	CreatedOn      string             `json:"created_on,omitempty"`
	UpdatedOn      string             `json:"updated_on,omitempty"`
	ClosedOn       string             `json:"closed_on,omitempty"`
	Journals       []Journal          `json:"journals,omitempty"`
	Attachments    []Attachment       `json:"attachments,omitempty"`
	Relations      []Relation         `json:"relations,omitempty"`
	Children       []Issue            `json:"children,omitempty"`
}

type ListResult[T any] struct {
	Items      []T
	Limit      int
	Offset     int
	TotalCount int
}

type APIError struct {
	Code          string
	Message       string
	Hint          string
	StatusCode    int
	RedmineErrors []string
	Details       map[string]any
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func New(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		BaseURL:          strings.TrimRight(baseURL, "/"),
		APIKey:           apiKey,
		MaxResponseBytes: DefaultMaxResponseBytes,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.HTTP = &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: c.checkRedirect,
	}
	return c
}

func (c *Client) IssueURL(id int) string {
	if c == nil {
		return ""
	}
	return c.BaseURL + "/issues/" + strconv.Itoa(id)
}

func (c *Client) NewRequest(method, path string) (*http.Request, error) {
	return c.NewRequestWithContext(context.Background(), method, path, nil, nil)
}

func (c *Client) NewRequestWithContext(ctx context.Context, method, path string, query url.Values, body any) (*http.Request, error) {
	if c == nil {
		return nil, errors.New("nil redmine client")
	}
	base, err := c.validatedBaseURL()
	if err != nil {
		return nil, err
	}
	u := *base
	u.RawPath = strings.TrimRight(base.EscapedPath(), "/") + path
	u.Path, err = url.PathUnescape(u.RawPath)
	if err != nil {
		return nil, err
	}
	u.RawQuery = ""
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var r io.Reader
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
		r = &buf
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("X-Redmine-API-Key", c.APIKey)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) validatedBaseURL() (*url.URL, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		return nil, errors.New("redmine URL must include https://")
	}
	if u.Host == "" {
		return nil, errors.New("redmine URL must include a host")
	}
	switch u.Scheme {
	case "https":
		return u, nil
	case "http":
		if c.AllowInsecureHTTP {
			return u, nil
		}
		return nil, errors.New("redmine URL must use https:// unless --allow-insecure-http is set")
	default:
		return nil, fmt.Errorf("redmine URL uses unsupported scheme %q", u.Scheme)
	}
}

func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if len(via) == 0 {
		return nil
	}
	if !sameOrigin(via[0].URL, req.URL) {
		return errors.New("redmine redirect changed origin")
	}
	return nil
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	req, err := c.NewRequestWithContext(ctx, method, path, query, body)
	if err != nil {
		return &APIError{
			Code:    "validation_failed",
			Message: "Invalid Redmine URL or request path",
			Hint:    "Use an https:// Redmine URL, or pass --allow-insecure-http for a one-off local HTTP command.",
			Details: map[string]any{"error": err.Error()},
		}
	}

	return c.doRequest(req, out)
}

func (c *Client) doRequest(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return mapTransportError(err)
	}
	defer resp.Body.Close()

	data, readErr := c.readResponseBody(resp.Body)
	if readErr != nil {
		if errors.Is(readErr, errResponseTooLarge) {
			return &APIError{
				Code:       "redmine_error",
				Message:    "Redmine response is too large",
				Hint:       "Reduce the query size or ask the Redmine server owner to inspect the response.",
				StatusCode: resp.StatusCode,
				Details: map[string]any{
					"limit_bytes": c.maxResponseBytes(),
				},
			}
		}
		return &APIError{
			Code:       "network_error",
			Message:    "Failed to read Redmine response",
			Hint:       "Retry the command or inspect the Redmine server logs.",
			StatusCode: resp.StatusCode,
			Details:    map[string]any{"error": readErr.Error()},
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeError(resp.StatusCode, data)
	}

	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return &APIError{
			Code:       "redmine_error",
			Message:    "Redmine returned invalid JSON",
			Hint:       "Retry the command or inspect the Redmine server response.",
			StatusCode: resp.StatusCode,
			Details:    map[string]any{"error": err.Error()},
		}
	}
	return nil
}

func (c *Client) readResponseBody(r io.Reader) ([]byte, error) {
	limit := c.maxResponseBytes()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errResponseTooLarge
	}
	return data, nil
}

func (c *Client) maxResponseBytes() int64 {
	if c != nil && c.MaxResponseBytes > 0 {
		return c.MaxResponseBytes
	}
	return DefaultMaxResponseBytes
}

func (c *Client) ListIssues(ctx context.Context, query url.Values) (ListResult[Issue], error) {
	var resp struct {
		Issues     []Issue `json:"issues"`
		TotalCount int     `json:"total_count"`
		Offset     int     `json:"offset"`
		Limit      int     `json:"limit"`
	}
	err := c.Do(ctx, http.MethodGet, "/issues.json", query, nil, &resp)
	return ListResult[Issue]{
		Items:      resp.Issues,
		Limit:      resp.Limit,
		Offset:     resp.Offset,
		TotalCount: resp.TotalCount,
	}, err
}

func (c *Client) GetIssue(ctx context.Context, id int, include string) (Issue, error) {
	q := url.Values{}
	if include != "" {
		q.Set("include", include)
	}
	var resp struct {
		Issue Issue `json:"issue"`
	}
	err := c.Do(ctx, http.MethodGet, "/issues/"+strconv.Itoa(id)+".json", q, nil, &resp)
	return resp.Issue, err
}

func (c *Client) CreateIssue(ctx context.Context, issue map[string]any) (Issue, error) {
	var resp struct {
		Issue Issue `json:"issue"`
	}
	err := c.Do(ctx, http.MethodPost, "/issues.json", nil, map[string]any{"issue": issue}, &resp)
	return resp.Issue, err
}

func (c *Client) UpdateIssue(ctx context.Context, id int, issue map[string]any) error {
	return c.Do(ctx, http.MethodPut, "/issues/"+strconv.Itoa(id)+".json", nil, map[string]any{"issue": issue}, nil)
}

func (c *Client) CreateRelation(ctx context.Context, id int, relation map[string]any) (Relation, error) {
	var resp struct {
		Relation Relation `json:"relation"`
	}
	err := c.Do(ctx, http.MethodPost, "/issues/"+strconv.Itoa(id)+"/relations.json", nil, map[string]any{"relation": relation}, &resp)
	return resp.Relation, err
}

func (c *Client) ListProjects(ctx context.Context, query url.Values) (ListResult[Project], error) {
	var resp struct {
		Projects   []Project `json:"projects"`
		TotalCount int       `json:"total_count"`
		Offset     int       `json:"offset"`
		Limit      int       `json:"limit"`
	}
	err := c.Do(ctx, http.MethodGet, "/projects.json", query, nil, &resp)
	return ListResult[Project]{
		Items:      resp.Projects,
		Limit:      resp.Limit,
		Offset:     resp.Offset,
		TotalCount: resp.TotalCount,
	}, err
}

func (c *Client) GetProject(ctx context.Context, identifier string) (Project, error) {
	var resp struct {
		Project Project `json:"project"`
	}
	err := c.Do(ctx, http.MethodGet, "/projects/"+url.PathEscape(identifier)+".json", nil, nil, &resp)
	return resp.Project, err
}

func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var resp struct {
		User User `json:"user"`
	}
	err := c.Do(ctx, http.MethodGet, "/users/current.json", nil, nil, &resp)
	return resp.User, err
}

func (c *Client) SearchUsers(ctx context.Context, query url.Values) (ListResult[User], error) {
	var resp struct {
		Users      []User `json:"users"`
		TotalCount int    `json:"total_count"`
		Offset     int    `json:"offset"`
		Limit      int    `json:"limit"`
	}
	err := c.Do(ctx, http.MethodGet, "/users.json", query, nil, &resp)
	return ListResult[User]{
		Items:      resp.Users,
		Limit:      resp.Limit,
		Offset:     resp.Offset,
		TotalCount: resp.TotalCount,
	}, err
}

func (c *Client) Statuses(ctx context.Context) ([]Reference, error) {
	var resp struct {
		Statuses []Reference `json:"issue_statuses"`
	}
	err := c.Do(ctx, http.MethodGet, "/issue_statuses.json", nil, nil, &resp)
	return resp.Statuses, err
}

func (c *Client) Trackers(ctx context.Context) ([]Reference, error) {
	var resp struct {
		Trackers []Reference `json:"trackers"`
	}
	err := c.Do(ctx, http.MethodGet, "/trackers.json", nil, nil, &resp)
	return resp.Trackers, err
}

func (c *Client) Priorities(ctx context.Context) ([]Reference, error) {
	var resp struct {
		Priorities []Reference `json:"issue_priorities"`
	}
	err := c.Do(ctx, http.MethodGet, "/enumerations/issue_priorities.json", nil, nil, &resp)
	return resp.Priorities, err
}

func (c *Client) CustomFields(ctx context.Context) ([]Reference, error) {
	var resp struct {
		CustomFields []Reference `json:"custom_fields"`
	}
	err := c.Do(ctx, http.MethodGet, "/custom_fields.json", nil, nil, &resp)
	return resp.CustomFields, err
}

func mapTransportError(err error) error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
		return &APIError{
			Code:    "timeout",
			Message: "Timed out while contacting Redmine",
			Hint:    "Retry later or check the Redmine URL and network path.",
			Details: map[string]any{"error": err.Error()},
		}
	}
	return &APIError{
		Code:    "network_error",
		Message: "Failed to contact Redmine",
		Hint:    "Check --url, REDMINE_URL, and network connectivity.",
		Details: map[string]any{"error": err.Error()},
	}
}

func decodeError(status int, body []byte) error {
	var decoded struct {
		Errors []string `json:"errors"`
		Error  string   `json:"error"`
	}
	_ = json.Unmarshal(body, &decoded)

	errs := decoded.Errors
	if decoded.Error != "" {
		errs = append(errs, decoded.Error)
	}
	message := strings.TrimSpace(strings.Join(errs, "; "))
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		message = http.StatusText(status)
	}

	code, hint := codeForStatus(status, errs)
	return &APIError{
		Code:          code,
		Message:       message,
		Hint:          hint,
		StatusCode:    status,
		RedmineErrors: errs,
		Details: map[string]any{
			"status":         status,
			"redmine_errors": errs,
		},
	}
}

func codeForStatus(status int, errs []string) (string, string) {
	switch status {
	case http.StatusUnauthorized:
		return "auth_failed", "Run redmine auth login to replace the stored credential, or check REDMINE_API_KEY/--api-key overrides."
	case http.StatusForbidden:
		return "permission_denied", "Use an API key with permission for this Redmine operation."
	case http.StatusNotFound:
		return "not_found", "Check the resource identifier and permissions."
	case http.StatusConflict:
		return "conflict", "Refresh the resource and retry if the change is still valid."
	case http.StatusUnprocessableEntity:
		for _, msg := range errs {
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "status") && (strings.Contains(lower, "invalid") || strings.Contains(lower, "workflow")) {
				return "workflow_rejected", "Try another transition or inspect the Redmine workflow."
			}
		}
		return "validation_failed", "Inspect error.details.redmine_errors and adjust the request."
	case http.StatusTooManyRequests:
		return "rate_limited", "Wait before retrying or reduce request volume."
	default:
		if status >= 500 {
			return "redmine_error", "Retry later or inspect the Redmine server."
		}
		return "redmine_error", fmt.Sprintf("Redmine returned HTTP %d.", status)
	}
}
