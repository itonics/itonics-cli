package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ElementTypeProperty is one property in a CreateElementType payload.
type ElementTypeProperty struct {
	Label string `json:"label"`
	Type  string `json:"type"`
}

// CreateElementTypeInput mirrors the API schema.
type CreateElementTypeInput struct {
	Label      string                `json:"label"`
	CreatedBy  string                `json:"createdBy"`
	Icon       string                `json:"icon,omitempty"`
	Properties []ElementTypeProperty `json:"properties"`
}

// ListElementTypes returns all element types in the space.
func (c *Client) ListElementTypes(ctx context.Context, filter, orderBy string) ([]json.RawMessage, error) {
	v := url.Values{}
	if filter != "" {
		v.Set("$filter", filter)
	}
	if orderBy != "" {
		v.Set("$orderby", orderBy)
	}
	return c.Paginated(ctx, "ElementTypes", v, "elementTypes", 0)
}

// CreateElementType posts a single element type.
func (c *Client) CreateElementType(ctx context.Context, in CreateElementTypeInput) (json.RawMessage, error) {
	if in.Properties == nil {
		in.Properties = []ElementTypeProperty{}
	}
	var out json.RawMessage
	body := map[string]any{"elementTypes": []CreateElementTypeInput{in}}
	if err := c.Do(ctx, "POST", "ElementTypes", nil, body, &out); err != nil {
		return nil, fmt.Errorf("create element type: %w", err)
	}
	return out, nil
}
