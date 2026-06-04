package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// UploadInfo is the upload sub-object of UploadFileResponse.
type UploadInfo struct {
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	ExpiresInSeconds float64           `json:"expiresInSeconds"`
	Headers          map[string]string `json:"headers"`
}

// UploadFileResponse is the body of POST /files.
type UploadFileResponse struct {
	FileURI string     `json:"fileUri"`
	Upload  UploadInfo `json:"upload"`
}

// CreateFileUploadURL requests a pre-signed upload URL.
func (c *Client) CreateFileUploadURL(ctx context.Context, fileName, createdBy, contentType string) (*UploadFileResponse, error) {
	body := map[string]string{"fileName": fileName, "createdBy": createdBy}
	if contentType != "" {
		body["contentType"] = contentType
	}
	var out UploadFileResponse
	if err := c.Do(ctx, "POST", "files", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFileDetails returns metadata + a short-lived pre-signed download URL.
func (c *Client) GetFileDetails(ctx context.Context, fileURI string) (json.RawMessage, error) {
	var out json.RawMessage
	if err := c.Do(ctx, "GET", "files/"+fileURI, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UploadFile is the 2-step upload: request URL, PUT binary. Returns fileUri.
//
// The body is streamed directly from disk; nothing is buffered in memory, so
// multi-hundred-MB attachments are safe.
//
// The API advertises an `x-amz-acl: private` header that the underlying bucket
// rejects (ACLs disabled), so we strip ACL headers before the PUT.
func (c *Client) UploadFile(ctx context.Context, path, createdBy, contentType string) (string, error) {
	f, size, err := openForUpload(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	name := baseName(path)
	if contentType == "" {
		contentType = GuessMime(name)
	}
	info, err := c.CreateFileUploadURL(ctx, name, createdBy, contentType)
	if err != nil {
		return "", err
	}

	method := info.Upload.Method
	if method == "" {
		method = "PUT"
	}
	req, err := http.NewRequestWithContext(ctx, method, info.Upload.URL, f)
	if err != nil {
		return "", err
	}
	// Stream directly from the file. ContentLength is required for the
	// pre-signed URL signature to validate, and lets net/http skip chunked
	// transfer encoding (which S3 rejects).
	req.ContentLength = size
	for k, v := range info.Upload.Headers {
		if strings.EqualFold(k, "x-amz-acl") {
			continue
		}
		req.Header.Set(k, v)
	}
	// The signed PUT against the upstream bucket uses the caller's context
	// directly so very large uploads aren't capped by RequestTimeout.
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("binary PUT: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300] + "…"
		}
		return "", fmt.Errorf("binary PUT failed %d: %s", resp.StatusCode, snippet)
	}
	return info.FileURI, nil
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}
