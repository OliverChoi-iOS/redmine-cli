package redmine

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

type Upload struct {
	Token       string `json:"token"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description,omitempty"`
}

func (c *Client) UploadFile(ctx context.Context, filename string, content io.Reader, size int64) (Upload, error) {
	req, err := c.NewRequestWithContext(ctx, http.MethodPost, "/uploads.json", url.Values{"filename": {filename}}, nil)
	if err != nil {
		return Upload{}, &APIError{Code: "validation_failed", Message: "Invalid upload request", Details: map[string]any{"error": err.Error()}}
	}
	req.Body = io.NopCloser(io.LimitReader(content, size))
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	var resp struct {
		Upload Upload `json:"upload"`
	}
	if err := c.doRequest(req, &resp); err != nil {
		return Upload{}, err
	}
	if resp.Upload.Token == "" {
		return Upload{}, &APIError{Code: "redmine_error", Message: "Redmine returned no upload token", Hint: "Check the Redmine upload API response before retrying."}
	}
	resp.Upload.Filename = filename
	return resp.Upload, nil
}
