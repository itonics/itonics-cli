package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient builds a Client pointed at srv with conservative defaults
// suitable for unit tests (no retries, short timeouts).
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c := New(srv.URL, "test-key", "SPACE")
	c.HTTP = srv.Client()
	c.RequestTimeout = 2 * time.Second
	c.MaxRetries = 0
	c.RetryBaseDelay = time.Millisecond
	return c
}

func TestDo_AuthHeadersAndJSONBody(t *testing.T) {
	var (
		gotAuth, gotAccept, gotUA, gotCT string
		gotBody                          string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	c.UserAgent = "itonics-cli/test"

	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.Do(context.Background(), "POST", "thing", nil, map[string]string{"a": "b"}, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !out.OK {
		t.Fatal("did not decode response")
	}
	if gotAuth != "ApiKey test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
	if gotUA != "itonics-cli/test" {
		t.Fatalf("User-Agent = %q", gotUA)
	}
	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q", gotCT)
	}
	if !strings.Contains(gotBody, `"a":"b"`) {
		t.Fatalf("body = %q, want JSON {a:b}", gotBody)
	}
}

func TestDo_APIErrorOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	err := c.Do(context.Background(), "GET", "thing", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error on 400")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", apiErr.Status)
	}
	if !strings.Contains(apiErr.Body, "nope") {
		t.Fatalf("Body = %q, want to include 'nope'", apiErr.Body)
	}
	if !strings.Contains(apiErr.Error(), "400") {
		t.Fatalf("Error() = %q, want to include status", apiErr.Error())
	}
}

func TestDo_BadJSONIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	var out map[string]any
	if err := c.Do(context.Background(), "GET", "thing", nil, nil, &out); err == nil {
		t.Fatal("expected JSON decode error")
	}
}

func TestDo_NilOutIgnoresBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ignored":true}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if err := c.Do(context.Background(), "DELETE", "thing", nil, nil, nil); err != nil {
		t.Fatalf("Do nil out: %v", err)
	}
}

func TestDo_RetryOn5xxThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	c.MaxRetries = 3

	var out map[string]any
	if err := c.Do(context.Background(), "GET", "thing", nil, nil, &out); err != nil {
		t.Fatalf("Do with retries: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("server calls = %d, want 3 (two retries)", got)
	}
}

func TestDo_NoRetryOnNonIdempotent(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	c.MaxRetries = 3

	err := c.Do(context.Background(), "POST", "thing", nil, map[string]string{"x": "y"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("server calls = %d, want exactly 1 (POST is not retried)", got)
	}
}

func TestPaginated_FollowsNextLinkAndReinjectsRawFieldValues(t *testing.T) {
	var (
		mu       = make(chan struct{}, 1)
		page1URL string
	)
	// Use a closure-captured variable to compose page1's nextLink — but we
	// need the server URL first. Use a level of indirection via a handler
	// that swaps behavior after the first call.
	var srv *httptest.Server
	var calls int32
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu <- struct{}{}
		defer func() { <-mu }()
		n := atomic.AddInt32(&calls, 1)
		// All requests must carry the auth header — including the nextLink follow-up.
		if got := r.Header.Get("Authorization"); got != "ApiKey test-key" {
			t.Errorf("call %d: Authorization = %q", n, got)
		}
		switch n {
		case 1:
			// Initial GET /Elements?$top=...&rawFieldValues=1
			if got := r.URL.Query().Get("rawFieldValues"); got != "1" {
				t.Errorf("first call missing rawFieldValues, got %q", got)
			}
			page1URL = r.URL.String()
			// nextLink intentionally omits rawFieldValues — the client should re-add it.
			next := srv.URL + "/page2?cursor=abc"
			fmt.Fprintf(w, `{"elements":[{"uri":"e1"},{"uri":"e2"}],"nextLink":%q}`, next)
		case 2:
			// nextLink follow-up. Client must have re-added rawFieldValues.
			if got := r.URL.Query().Get("rawFieldValues"); got != "1" {
				t.Errorf("follow-up missing rawFieldValues, got %q (full URL: %s)", got, r.URL.String())
			}
			if r.URL.Path != "/page2" || r.URL.Query().Get("cursor") != "abc" {
				t.Errorf("follow-up went to wrong URL: %s", r.URL.String())
			}
			_, _ = w.Write([]byte(`{"elements":[{"uri":"e3"}]}`))
		default:
			t.Errorf("unexpected call %d", n)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	v := url.Values{}
	v.Set("rawFieldValues", "1")
	items, err := c.Paginated(context.Background(), "Elements", v, "elements", 0)
	if err != nil {
		t.Fatalf("Paginated: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	_ = page1URL // referenced for clarity; assertion is on the URL above
}

func TestPaginated_StopsAtLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[{"uri":"a"},{"uri":"b"},{"uri":"c"}]}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	items, err := c.Paginated(context.Background(), "Elements", url.Values{}, "elements", 2)
	if err != nil {
		t.Fatalf("Paginated: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d, want 2", len(items))
	}
}

func TestPaginated_UnknownCollection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if _, err := c.Paginated(context.Background(), "X", nil, "bogus", 0); err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestUploadFile_StripsACLHeaderAndStreams(t *testing.T) {
	// Step 1: POST /files returns the pre-signed URL.
	// Step 2: PUT to that URL — must NOT include x-amz-acl, and must read the file body.
	var (
		gotACL      string
		gotMethod   string
		gotCT       string
		gotBody     []byte
		gotCLHeader string
	)

	var uploadSrv *httptest.Server
	uploadSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotACL = r.Header.Get("x-amz-acl")
		gotCT = r.Header.Get("Content-Type")
		gotCLHeader = r.Header.Get("Content-Length")
		b, _ := io.ReadAll(r.Body)
		gotBody = b
		w.WriteHeader(http.StatusOK)
	}))
	defer uploadSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/files") || r.Method != http.MethodPost {
			t.Errorf("unexpected API call: %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprintf(w, `{
			"fileUri": "FILE-URI-1",
			"upload": {
				"url": %q,
				"method": "PUT",
				"headers": {
					"Content-Type": "text/plain",
					"x-amz-acl": "private"
				}
			}
		}`, uploadSrv.URL+"/upload")
	}))
	defer apiSrv.Close()

	c := newTestClient(t, apiSrv)

	// Write a small payload to a temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	payload := []byte("hello world")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	fileURI, err := c.UploadFile(context.Background(), path, "user@example.com", "")
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if fileURI != "FILE-URI-1" {
		t.Fatalf("fileURI = %q", fileURI)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("PUT method = %q", gotMethod)
	}
	if gotACL != "" {
		t.Fatalf("x-amz-acl leaked through: %q", gotACL)
	}
	if gotCT != "text/plain" {
		t.Fatalf("Content-Type on PUT = %q, want text/plain", gotCT)
	}
	if string(gotBody) != "hello world" {
		t.Fatalf("upload body = %q, want %q", gotBody, payload)
	}
	if gotCLHeader == "" {
		t.Fatal("Content-Length missing on streamed upload")
	}
}

func TestUploadFile_PropagatesPUTFailure(t *testing.T) {
	uploadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied"))
	}))
	defer uploadSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"fileUri":"x","upload":{"url":%q,"method":"PUT"}}`, uploadSrv.URL)
	}))
	defer apiSrv.Close()

	c := newTestClient(t, apiSrv)

	dir := t.TempDir()
	path := filepath.Join(dir, "x.bin")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := c.UploadFile(context.Background(), path, "u@e", "application/octet-stream")
	if err == nil {
		t.Fatal("expected PUT failure error")
	}
	if !strings.Contains(err.Error(), "binary PUT failed 403") {
		t.Fatalf("err = %v", err)
	}
}

func TestUploadFile_MissingFile(t *testing.T) {
	c := New("https://example", "k", "S")
	_, err := c.UploadFile(context.Background(), "/no/such/path/itonics-cli-test-missing", "u@e", "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d := parseRetryAfter("3"); d != 3*time.Second {
		t.Fatalf("seconds: got %v", d)
	}
	if d := parseRetryAfter(""); d != 0 {
		t.Fatalf("empty: got %v", d)
	}
	if d := parseRetryAfter("not a number"); d != 0 {
		t.Fatalf("garbage: got %v", d)
	}
	if d := parseRetryAfter("0"); d != 0 {
		t.Fatalf("zero: got %v", d)
	}
}

func TestGuessMime(t *testing.T) {
	cases := map[string]string{
		"foo.png":       "image/png",
		"foo.txt":       "text/plain",
		"foo":           "application/octet-stream",
		"archive.tar":   "application/x-tar",
		"unknown.xyzzy": "application/octet-stream",
	}
	for in, want := range cases {
		got := GuessMime(in)
		if want == "application/octet-stream" {
			if got != want {
				t.Errorf("GuessMime(%q) = %q, want %q", in, got, want)
			}
			continue
		}
		// Some systems return text/plain or text/plain; charset=utf-8 — the
		// client strips charset, so just check the prefix.
		if !strings.HasPrefix(got, strings.SplitN(want, "/", 2)[0]) {
			t.Errorf("GuessMime(%q) = %q, want type family %q", in, got, want)
		}
	}
}

// TestListElements_DecodesRawMessages exercises ListElements end-to-end
// against a fake server.
func TestListElements_DecodesRawMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("$filter"); got != "x eq 'y'" {
			t.Errorf("filter forwarded = %q", got)
		}
		_, _ = w.Write([]byte(`{"elements":[{"uri":"a","label":"A"}]}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	items, err := c.ListElements(context.Background(), ListElementsParams{Filter: "x eq 'y'"})
	if err != nil {
		t.Fatalf("ListElements: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d", len(items))
	}
	var probe struct {
		URI   string `json:"uri"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(items[0], &probe); err != nil {
		t.Fatal(err)
	}
	if probe.URI != "a" || probe.Label != "A" {
		t.Fatalf("decoded = %+v", probe)
	}
}
