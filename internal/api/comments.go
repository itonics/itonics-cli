package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// CreateCommentInput mirrors a create-comment item. Text is plain text or HTML
// (stored as TipTap); mention a user with @email@domain.com and reference an
// element with #{Element label} or #{elementUri} inline.
type CreateCommentInput struct {
	CommentedBy string `json:"commentedBy"`
	Text        string `json:"text"`
}

// UpdateCommentInput mirrors an update-comment item.
type UpdateCommentInput struct {
	CommentURI string `json:"commentUri"`
	UpdatedBy  string `json:"updatedBy"`
	Text       string `json:"text,omitempty"`
}

// ListCommentsParams is the supported $ subset for GET /comments/.
type ListCommentsParams struct {
	Filter  string
	Select  string
	OrderBy string
	Top     int
}

func (p ListCommentsParams) values() url.Values {
	v := url.Values{}
	if p.Filter != "" {
		v.Set("$filter", p.Filter)
	}
	if p.Select != "" {
		v.Set("$select", p.Select)
	}
	if p.OrderBy != "" {
		v.Set("$orderby", p.OrderBy)
	}
	if p.Top > 0 {
		v.Set("$top", fmt.Sprintf("%d", p.Top))
	}
	return v
}

func commentsPath(elementURI string) string {
	return fmt.Sprintf("elements/%s/comments/", elementURI)
}

// ListComments returns comments on an element, following nextLink.
func (c *Client) ListComments(ctx context.Context, elementURI string, p ListCommentsParams) ([]json.RawMessage, error) {
	return c.Paginated(ctx, commentsPath(elementURI), p.values(), "value", p.Top)
}

// CreateComments adds one or more comments to an element. Processed
// all-or-nothing.
func (c *Client) CreateComments(ctx context.Context, elementURI string, items []CreateCommentInput) (json.RawMessage, error) {
	body := map[string]any{"items": items}
	var out json.RawMessage
	if err := c.Do(ctx, "POST", commentsPath(elementURI), nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateComments edits one or more comments by URI.
func (c *Client) UpdateComments(ctx context.Context, elementURI string, items []UpdateCommentInput) (json.RawMessage, error) {
	body := map[string]any{"items": items}
	var out json.RawMessage
	if err := c.Do(ctx, "PATCH", commentsPath(elementURI), nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteComments removes comments from an element by URI.
func (c *Client) DeleteComments(ctx context.Context, elementURI string, commentURIs []string) (json.RawMessage, error) {
	body := map[string]any{"commentUris": commentURIs}
	var out json.RawMessage
	if err := c.Do(ctx, "DELETE", commentsPath(elementURI), nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
