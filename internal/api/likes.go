package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ListLikesParams is the supported $ subset for GET /likes.
type ListLikesParams struct {
	Filter  string
	OrderBy string
	Select  string
	Top     int
	Skip    int
}

// ListLikes returns users who liked an element. $filter supports UserURI eq|ne ...
// and likedOn gt|lt 'ISO-8601' (combine with and).
func (c *Client) ListLikes(ctx context.Context, elementURI string, p ListLikesParams) (json.RawMessage, error) {
	v := url.Values{}
	if p.Filter != "" {
		v.Set("$filter", p.Filter)
	}
	if p.OrderBy != "" {
		v.Set("$orderby", p.OrderBy)
	}
	if p.Select != "" {
		v.Set("$select", p.Select)
	}
	if p.Top > 0 {
		v.Set("$top", fmt.Sprintf("%d", p.Top))
	}
	if p.Skip > 0 {
		v.Set("$skip", fmt.Sprintf("%d", p.Skip))
	}
	var out json.RawMessage
	if err := c.Do(ctx, "GET", fmt.Sprintf("elements/%s/likes", elementURI), v, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddLikes records likes from the given user URIs (1–100 per request).
func (c *Client) AddLikes(ctx context.Context, elementURI string, userURIs []string) (json.RawMessage, error) {
	body := map[string]any{"userUris": userURIs}
	var out json.RawMessage
	if err := c.Do(ctx, "POST", fmt.Sprintf("elements/%s/likes", elementURI), nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveLikes removes likes. Empty list is a no-op (204).
func (c *Client) RemoveLikes(ctx context.Context, elementURI string, userURIs []string) error {
	body := map[string]any{}
	if len(userURIs) > 0 {
		body["userUris"] = userURIs
	}
	return c.Do(ctx, "DELETE", fmt.Sprintf("elements/%s/likes", elementURI), nil, body, nil)
}
