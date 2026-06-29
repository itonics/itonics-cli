package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ViewLayout is the page_view layout. On input the API accepts only html
// (plain HTML + stock Tailwind, or a base64:<...> string); no custom css.
type ViewLayout struct {
	HTML string `json:"html"`
}

// CreateViewInput mirrors CreateViewRequest. Creation supports the page_view
// preset only.
type CreateViewInput struct {
	Label      string          `json:"label"`
	PresetType string          `json:"presetType"`
	CreatedBy  string          `json:"createdBy"`
	Visibility string          `json:"visibility,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`
	FolderURI  string          `json:"folderUri,omitempty"`
	Layout     *ViewLayout     `json:"layout,omitempty"`
	IsFavorite *bool           `json:"isFavorite,omitempty"`
}

// UpdateViewInput mirrors UpdateViewRequest; only set fields that should change.
type UpdateViewInput struct {
	URI        string      `json:"uri"`
	UpdatedBy  string      `json:"updatedBy"`
	Label      *string     `json:"label,omitempty"`
	Visibility *string     `json:"visibility,omitempty"`
	FolderURI  *string     `json:"folderUri,omitempty"`
	Layout     *ViewLayout `json:"layout,omitempty"`
	IsFavorite *bool       `json:"isFavorite,omitempty"`
}

// ListViewsParams is the supported $ subset for GET /views/.
type ListViewsParams struct {
	Filter         string
	OrderBy        string
	Top            int
	RawFieldValues bool
}

func (p ListViewsParams) values() url.Values {
	v := url.Values{}
	if p.Filter != "" {
		v.Set("$filter", p.Filter)
	}
	if p.OrderBy != "" {
		v.Set("$orderby", p.OrderBy)
	}
	if p.Top > 0 {
		v.Set("$top", fmt.Sprintf("%d", p.Top))
	}
	if p.RawFieldValues {
		v.Set("rawFieldValues", "1")
	}
	return v
}

// ListViews returns saved views, following nextLink.
func (c *Client) ListViews(ctx context.Context, p ListViewsParams) ([]json.RawMessage, error) {
	return c.Paginated(ctx, "views/", p.values(), "value", p.Top)
}

// GetView fetches a single view by URI.
func (c *Client) GetView(ctx context.Context, uri string, raw bool) (json.RawMessage, error) {
	v := url.Values{}
	if raw {
		v.Set("rawFieldValues", "1")
	}
	var out json.RawMessage
	if err := c.Do(ctx, "GET", "views/"+uri, v, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateViews posts one or more views. Returns the raw API response.
func (c *Client) CreateViews(ctx context.Context, views []CreateViewInput, raw bool) (json.RawMessage, error) {
	body := map[string]any{"views": views}
	if raw {
		body["rawFieldValues"] = true
	}
	var out json.RawMessage
	if err := c.Do(ctx, "POST", "views/", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateViews PATCHes one or more views by URI.
func (c *Client) UpdateViews(ctx context.Context, views []UpdateViewInput, raw bool) (json.RawMessage, error) {
	body := map[string]any{"views": views}
	if raw {
		body["rawFieldValues"] = true
	}
	var out json.RawMessage
	if err := c.Do(ctx, "PATCH", "views/", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteViews removes views by URI.
func (c *Client) DeleteViews(ctx context.Context, uris []string) (json.RawMessage, error) {
	body := map[string]any{"uris": uris}
	var out json.RawMessage
	if err := c.Do(ctx, "DELETE", "views/", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
