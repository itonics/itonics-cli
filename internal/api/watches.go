package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ListWatchesParams is the supported $ subset for GET /watches.
type ListWatchesParams struct {
	OrderBy string
	Top     int
	Skip    int
}

// ListWatches returns users watching an element.
func (c *Client) ListWatches(ctx context.Context, elementURI string, p ListWatchesParams) (json.RawMessage, error) {
	v := url.Values{}
	if p.OrderBy != "" {
		v.Set("$orderby", p.OrderBy)
	}
	if p.Top > 0 {
		v.Set("$top", fmt.Sprintf("%d", p.Top))
	}
	if p.Skip > 0 {
		v.Set("$skip", fmt.Sprintf("%d", p.Skip))
	}
	var out json.RawMessage
	if err := c.Do(ctx, "GET", fmt.Sprintf("elements/%s/watches", elementURI), v, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddWatches subscribes one or more users to change notifications. 1–100 URIs.
func (c *Client) AddWatches(ctx context.Context, elementURI string, userURIs []string) (json.RawMessage, error) {
	body := map[string]any{"userUris": userURIs}
	var out json.RawMessage
	if err := c.Do(ctx, "POST", fmt.Sprintf("elements/%s/watches", elementURI), nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveWatches removes watchers. Empty list is a no-op (204).
func (c *Client) RemoveWatches(ctx context.Context, elementURI string, userURIs []string) error {
	body := map[string]any{}
	if len(userURIs) > 0 {
		body["userUris"] = userURIs
	}
	return c.Do(ctx, "DELETE", fmt.Sprintf("elements/%s/watches", elementURI), nil, body, nil)
}
