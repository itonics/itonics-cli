package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Property is a single name=value property written on an element.
type Property struct {
	URI      string `json:"uri"`
	Value    string `json:"value"`
	ValueURI string `json:"valueUri,omitempty"`
}

// CreateElementInput mirrors the API's CreateElement schema.
type CreateElementInput struct {
	ElementTypeURI string     `json:"elementTypeUri"`
	Label          string     `json:"label"`
	Summary        string     `json:"summary"`
	CreatedBy      string     `json:"createdBy"`
	Status         string     `json:"status,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	Properties     []Property `json:"properties,omitempty"`
}

// UpdateElementInput mirrors UpdateElement; only set fields that should change.
type UpdateElementInput struct {
	URI            string     `json:"uri"`
	UpdatedBy      string     `json:"updatedBy"`
	Label          *string    `json:"label,omitempty"`
	Summary        *string    `json:"summary,omitempty"`
	Status         *string    `json:"status,omitempty"`
	ElementTypeURI *string    `json:"elementTypeUri,omitempty"`
	Tags           *[]string  `json:"tags,omitempty"`
	Properties     []Property `json:"properties,omitempty"`
}

// ListElementsParams collects $-prefixed OData params plus rawFieldValues.
type ListElementsParams struct {
	Filter         string
	Search         string
	Select         string
	OrderBy        string
	Top            int
	RawFieldValues bool
}

func (p ListElementsParams) values() url.Values {
	v := url.Values{}
	if p.Filter != "" {
		v.Set("$filter", p.Filter)
	}
	if p.Search != "" {
		v.Set("$search", p.Search)
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
	if p.RawFieldValues {
		v.Set("rawFieldValues", "1")
	}
	return v
}

// ListElements returns all matching elements, following nextLink. If
// params.Top is set, stops after that many.
func (c *Client) ListElements(ctx context.Context, params ListElementsParams) ([]json.RawMessage, error) {
	return c.Paginated(ctx, "Elements", params.values(), "elements", params.Top)
}

// GetElement fetches a single element by URI.
func (c *Client) GetElement(ctx context.Context, uri string, expand string, raw bool) (json.RawMessage, error) {
	v := url.Values{}
	if expand != "" {
		v.Set("$expand", expand)
	}
	if raw {
		v.Set("rawFieldValues", "1")
	}
	var resp struct {
		Elements []json.RawMessage `json:"elements"`
	}
	if err := c.Do(ctx, "GET", "Elements/"+uri, v, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Elements) == 0 {
		return nil, fmt.Errorf("element %s not found", uri)
	}
	return resp.Elements[0], nil
}

// CreateElements posts one or more elements. Returns the raw API response.
func (c *Client) CreateElements(ctx context.Context, elements []CreateElementInput, raw bool) (json.RawMessage, error) {
	body := map[string]any{"elements": elements}
	if raw {
		body["rawFieldValues"] = true
	}
	var out json.RawMessage
	if err := c.Do(ctx, "POST", "Elements", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateElements PATCHes one or more elements.
func (c *Client) UpdateElements(ctx context.Context, elements []UpdateElementInput, raw bool) (json.RawMessage, error) {
	body := map[string]any{"elements": elements}
	if raw {
		body["rawFieldValues"] = true
	}
	var out json.RawMessage
	if err := c.Do(ctx, "PATCH", "Elements", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteElements removes elements by URI.
func (c *Client) DeleteElements(ctx context.Context, uris []string) (json.RawMessage, error) {
	body := map[string]any{"elementUris": uris}
	var out json.RawMessage
	if err := c.Do(ctx, "DELETE", "Elements", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
