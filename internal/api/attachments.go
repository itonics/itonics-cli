package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// AttachFiles links one or more uploaded files to an element. Idempotent.
func (c *Client) AttachFiles(ctx context.Context, elementURI string, fileURIs []string, attachedBy string) (json.RawMessage, error) {
	body := map[string]any{"fileUris": fileURIs, "attachedBy": attachedBy}
	var out json.RawMessage
	if err := c.Do(ctx, "POST", fmt.Sprintf("elements/%s/attachments", elementURI), nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListAttachmentsParams is the supported $ subset for GET /attachments.
type ListAttachmentsParams struct {
	Select  string
	OrderBy string
	Top     int
	Skip    int
}

// ListAttachments returns attachments linked to an element.
func (c *Client) ListAttachments(ctx context.Context, elementURI string, p ListAttachmentsParams) (json.RawMessage, error) {
	v := url.Values{}
	if p.Select != "" {
		v.Set("$select", p.Select)
	}
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
	if err := c.Do(ctx, "GET", fmt.Sprintf("elements/%s/attachments", elementURI), v, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DetachFiles unlinks files from an element. Empty list is a no-op.
func (c *Client) DetachFiles(ctx context.Context, elementURI string, fileURIs []string, detachedBy string) (json.RawMessage, error) {
	body := map[string]any{"fileUris": fileURIs, "detachedBy": detachedBy}
	var out json.RawMessage
	if err := c.Do(ctx, "DELETE", fmt.Sprintf("elements/%s/attachments", elementURI), nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
