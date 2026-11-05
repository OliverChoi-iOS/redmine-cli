package redmine

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type WikiParent struct {
	Title string `json:"title"`
}

type WikiPage struct {
	Title       string       `json:"title"`
	Text        string       `json:"text,omitempty"`
	Version     int          `json:"version"`
	Parent      *WikiParent  `json:"parent,omitempty"`
	Author      Reference    `json:"author,omitempty"`
	Comments    string       `json:"comments,omitempty"`
	CreatedOn   string       `json:"created_on,omitempty"`
	UpdatedOn   string       `json:"updated_on,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

func wikiPath(project string) string {
	return "/projects/" + url.PathEscape(project) + "/wiki/"
}

func (c *Client) ListWikiPages(ctx context.Context, project string) ([]WikiPage, error) {
	var resp struct {
		Pages []WikiPage `json:"wiki_pages"`
	}
	err := c.Do(ctx, http.MethodGet, wikiPath(project)+"index.json", nil, nil, &resp)
	return resp.Pages, err
}

func (c *Client) GetWikiPage(ctx context.Context, project, title string, version int, include string) (WikiPage, error) {
	path := wikiPath(project) + url.PathEscape(title)
	if version > 0 {
		path += "/" + strconv.Itoa(version)
	}
	q := url.Values{}
	if include != "" {
		q.Set("include", include)
	}
	var resp struct {
		Page WikiPage `json:"wiki_page"`
	}
	err := c.Do(ctx, http.MethodGet, path+".json", q, nil, &resp)
	return resp.Page, err
}

func (c *Client) PutWikiPage(ctx context.Context, project, title string, page map[string]any) error {
	return c.Do(ctx, http.MethodPut, wikiPath(project)+url.PathEscape(title)+".json", nil, map[string]any{"wiki_page": page}, nil)
}
