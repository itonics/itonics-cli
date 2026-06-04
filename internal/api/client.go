// Package api is a thin client for the ITONICS Innovation OData v2 API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Default timeouts and retry policy. Tweakable via Client fields.
const (
	defaultRequestTimeout = 60 * time.Second
	defaultMaxRetries     = 2
	defaultRetryBaseDelay = 250 * time.Millisecond
)

// Client talks to a single ITONICS tenant + space.
type Client struct {
	Domain    string
	APIKey    string
	Space     string
	HTTP      *http.Client
	UserAgent string

	// RequestTimeout bounds each individual HTTP round-trip the client makes
	// (excluding the binary upload PUT, which uses the caller's context).
	// Zero means use defaultRequestTimeout.
	RequestTimeout time.Duration

	// MaxRetries is the number of retry attempts for transient failures on
	// idempotent verbs (GET, DELETE, HEAD). Zero disables retries; negative
	// means use defaultMaxRetries.
	MaxRetries int

	// RetryBaseDelay is the initial backoff between retries. The actual delay
	// grows linearly (1x, 2x, 3x …). Zero means use defaultRetryBaseDelay.
	RetryBaseDelay time.Duration

	baseURL string
}

// New builds a Client. domain must include scheme; trailing slashes are
// stripped. space is the OData space URI.
func New(domain, apiKey, space string) *Client {
	d := strings.TrimRight(domain, "/")
	return &Client{
		Domain: d,
		APIKey: apiKey,
		Space:  space,
		// No top-level timeout on http.Client; we use per-request contexts so
		// long-running uploads aren't killed by the API-call deadline.
		HTTP:           &http.Client{},
		UserAgent:      "itonics-cli",
		RequestTimeout: defaultRequestTimeout,
		MaxRetries:     defaultMaxRetries,
		RetryBaseDelay: defaultRetryBaseDelay,
		baseURL:        fmt.Sprintf("%s/rest/external/odata/v2/%s", d, space),
	}
}

// BaseURL exposes the resolved /rest/external/odata/v2/<space> root.
func (c *Client) BaseURL() string { return c.baseURL }

// APIError surfaces a non-2xx response from the ITONICS API.
type APIError struct {
	Status int
	Body   string
	URL    string
}

func (e *APIError) Error() string {
	body := e.Body
	if len(body) > 500 {
		body = body[:500] + "…"
	}
	return fmt.Sprintf("ITONICS API error %d: %s", e.Status, body)
}

// requestTimeout returns the effective per-request timeout.
func (c *Client) requestTimeout() time.Duration {
	if c.RequestTimeout > 0 {
		return c.RequestTimeout
	}
	return defaultRequestTimeout
}

// maxRetries returns the effective retry count.
func (c *Client) maxRetries() int {
	if c.MaxRetries < 0 {
		return defaultMaxRetries
	}
	return c.MaxRetries
}

// retryBaseDelay returns the effective base backoff.
func (c *Client) retryBaseDelay() time.Duration {
	if c.RetryBaseDelay > 0 {
		return c.RetryBaseDelay
	}
	return defaultRetryBaseDelay
}

// setAuthHeaders installs the headers every authenticated request needs.
// Centralized so callers (Do, Paginated, follow-ups) cannot drift.
func (c *Client) setAuthHeaders(req *http.Request, hasJSONBody bool) {
	req.Header.Set("Authorization", "ApiKey "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	if hasJSONBody {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string, params url.Values, body any) (*http.Request, error) {
	u, err := url.Parse(c.baseURL + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return nil, err
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}
	// We may retry, so cache the marshalled body and use a GetBody factory.
	var (
		bodyBytes  []byte
		bodyReader io.Reader
	)
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	if bodyBytes != nil {
		buf := bodyBytes // capture for closure
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(buf)), nil
		}
	}
	c.setAuthHeaders(req, body != nil)
	return req, nil
}

// isIdempotent reports whether method is safe to retry on transient failures.
func isIdempotent(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodPut:
		return true
	}
	return false
}

// shouldRetryStatus reports whether a status code is worth retrying.
func shouldRetryStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, // 408
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	}
	return false
}

// parseRetryAfter reads a Retry-After header (seconds or HTTP-date). Returns
// zero on parse failure or missing header.
func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// doOnce sends a single round-trip with a bounded per-call context. The
// returned body is fully buffered to allow retries to inspect it.
func (c *Client) doOnce(ctx context.Context, req *http.Request) (int, []byte, http.Header, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.requestTimeout())
	defer cancel()

	// Clone with the bounded context. We can't simply WithContext because the
	// original req carries the user's ctx already.
	r := req.Clone(reqCtx)
	if req.GetBody != nil {
		rc, err := req.GetBody()
		if err != nil {
			return 0, nil, nil, err
		}
		r.Body = rc
	}
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, resp.Header, err
	}
	return resp.StatusCode, data, resp.Header, nil
}

// doWithRetry sends req, retrying transient failures on idempotent verbs.
func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (int, []byte, http.Header, error) {
	attempts := 1
	if isIdempotent(req.Method) {
		attempts += c.maxRetries()
	}
	var (
		status int
		body   []byte
		hdrs   http.Header
		err    error
	)
	for i := 0; i < attempts; i++ {
		status, body, hdrs, err = c.doOnce(ctx, req)
		// Network error: retry if attempts remain and ctx is alive.
		if err != nil {
			if ctx.Err() != nil {
				return status, body, hdrs, err
			}
			if i < attempts-1 {
				select {
				case <-ctx.Done():
					return status, body, hdrs, ctx.Err()
				case <-time.After(c.retryBaseDelay() * time.Duration(i+1)):
				}
				continue
			}
			return status, body, hdrs, err
		}
		// HTTP error: retry on transient statuses.
		if shouldRetryStatus(status) && i < attempts-1 {
			delay := parseRetryAfter(hdrs.Get("Retry-After"))
			if delay == 0 {
				delay = c.retryBaseDelay() * time.Duration(i+1)
			}
			select {
			case <-ctx.Done():
				return status, body, hdrs, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return status, body, hdrs, nil
	}
	return status, body, hdrs, err
}

// Do issues an authenticated request and decodes a JSON response into out.
// Pass out=nil to discard the body.
func (c *Client) Do(ctx context.Context, method, path string, params url.Values, body, out any) error {
	req, err := c.newRequest(ctx, method, path, params, body)
	if err != nil {
		return err
	}
	status, data, _, err := c.doWithRetry(ctx, req)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return &APIError{Status: status, Body: string(data), URL: req.URL.String()}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// page is a generic OData envelope: { value/elements/... , nextLink }.
type page struct {
	Elements     json.RawMessage `json:"elements"`
	ElementTypes json.RawMessage `json:"elementTypes"`
	Value        json.RawMessage `json:"value"`
	NextLink     string          `json:"nextLink"`
}

// newPaginatedRequest builds the first or follow-up GET in a paginated walk,
// sharing auth-header setup with Do via setAuthHeaders.
func (c *Client) newPaginatedRequest(ctx context.Context, path string, params url.Values, nextURL string) (*http.Request, error) {
	if nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, err
		}
		c.setAuthHeaders(req, false)
		return req, nil
	}
	return c.newRequest(ctx, http.MethodGet, path, params, nil)
}

// Paginated follows nextLink and accumulates items from the named collection
// (e.g. "elements", "elementTypes", "value"). It re-injects rawFieldValues if
// the server drops it from nextLink (a known quirk).
func (c *Client) Paginated(ctx context.Context, path string, params url.Values, collection string, limit int) ([]json.RawMessage, error) {
	switch collection {
	case "elements", "elementTypes", "value":
	default:
		return nil, fmt.Errorf("unknown collection %q", collection)
	}
	wantRaw := params != nil && params.Get("rawFieldValues") != ""
	var out []json.RawMessage
	nextURL := ""

	for {
		req, err := c.newPaginatedRequest(ctx, path, params, nextURL)
		if err != nil {
			return nil, err
		}
		status, body, _, err := c.doWithRetry(ctx, req)
		if err != nil {
			return nil, err
		}
		if status < 200 || status >= 300 {
			return nil, &APIError{Status: status, Body: string(body), URL: req.URL.String()}
		}
		var p page
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		var raw json.RawMessage
		switch collection {
		case "elements":
			raw = p.Elements
		case "elementTypes":
			raw = p.ElementTypes
		case "value":
			raw = p.Value
		}
		var items []json.RawMessage
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, fmt.Errorf("decode %s page: %w", collection, err)
			}
		}
		out = append(out, items...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
		if p.NextLink == "" {
			return out, nil
		}
		nextURL = p.NextLink
		if wantRaw && !strings.Contains(nextURL, "rawFieldValues") {
			sep := "&"
			if !strings.Contains(nextURL, "?") {
				sep = "?"
			}
			nextURL += sep + "rawFieldValues=1"
		}
	}
}

// --- Helpers ---

// GuessMime returns a content type by extension, falling back to
// application/octet-stream.
func GuessMime(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return "application/octet-stream"
	}
	if m := mime.TypeByExtension(ext); m != "" {
		return strings.Split(m, ";")[0]
	}
	return "application/octet-stream"
}

// openForUpload opens path for reading and returns the file plus its size.
// Used by UploadFile to stream the body without buffering in memory.
func openForUpload(path string) (*os.File, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	if st.IsDir() {
		_ = f.Close()
		return nil, 0, errors.New("path is a directory")
	}
	return f, st.Size(), nil
}
