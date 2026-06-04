package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestIsValid(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"json", true},
		{"table", true},
		{"yaml", false},
		{"", false},
		{"JSON", false}, // case sensitive on purpose; matches existing CLI behavior
	}
	for _, tc := range cases {
		if got := IsValid(tc.in); got != tc.want {
			t.Errorf("IsValid(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestRender_JSONListOfElements(t *testing.T) {
	payload := map[string]any{
		"elements": []map[string]any{
			{"uri": "u1", "label": "First"},
			{"uri": "u2", "label": "Second"},
		},
	}
	var buf bytes.Buffer
	if err := Render(&buf, payload, JSON); err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Round-trip through json to verify the encoder produced valid JSON.
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("JSON output should end with newline")
	}
	if !strings.Contains(buf.String(), `"label": "First"`) {
		t.Errorf("indented output missing expected line:\n%s", buf.String())
	}
}

func TestRender_JSONDefaultFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, map[string]int{"n": 1}, ""); err != nil {
		t.Fatalf("Render with empty format: %v", err)
	}
	if !strings.Contains(buf.String(), `"n": 1`) {
		t.Fatalf("empty format should default to JSON: %s", buf.String())
	}
}

func TestRender_TableForElementsList(t *testing.T) {
	payload := map[string]any{
		"elements": []map[string]any{
			{"uri": "elem-uri-1", "label": "Hello", "status": "published"},
			{"uri": "elem-uri-2", "label": "World", "status": "draft"},
		},
	}
	var buf bytes.Buffer
	if err := Render(&buf, payload, Table); err != nil {
		t.Fatalf("Render(table): %v", err)
	}
	out := buf.String()
	// Header columns from pickColumns preferred order: uri, label, status.
	header := strings.Split(out, "\n")[0]
	uriIdx := strings.Index(header, "uri")
	labelIdx := strings.Index(header, "label")
	statusIdx := strings.Index(header, "status")
	if uriIdx < 0 || labelIdx < 0 || statusIdx < 0 {
		t.Fatalf("missing expected columns in header:\n%s", out)
	}
	if !(uriIdx < labelIdx && labelIdx < statusIdx) {
		t.Errorf("column order wrong: uri=%d label=%d status=%d", uriIdx, labelIdx, statusIdx)
	}
	// Each row's data appears.
	for _, want := range []string{"elem-uri-1", "Hello", "published", "elem-uri-2", "World", "draft"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
	// There should be a separator line of dashes.
	if !strings.Contains(out, "---") {
		t.Errorf("table missing dashed separator:\n%s", out)
	}
}

func TestRender_TableFallsBackToJSONForUnrecognizedShape(t *testing.T) {
	payload := map[string]any{"scalar": 42}
	var buf bytes.Buffer
	if err := Render(&buf, payload, Table); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"scalar"`) {
		t.Fatalf("expected JSON fallback, got:\n%s", out)
	}
}

func TestRender_TableEmptyListShowsNoResults(t *testing.T) {
	payload := map[string]any{"elements": []any{}}
	var buf bytes.Buffer
	if err := Render(&buf, payload, Table); err != nil {
		t.Fatalf("Render: %v", err)
	}
	// asRowList returns ok=true with len==0, which currently falls back to
	// JSON in Render() because writeTable is only called when len(rows) > 0.
	// Verify the user gets *something* useful (JSON), not a panic.
	out := buf.String()
	if !strings.Contains(out, `"elements"`) {
		t.Fatalf("expected JSON fallback for empty list, got:\n%s", out)
	}
}

func TestRender_TableForTopLevelSlice(t *testing.T) {
	// json.RawMessage is rendered via a top-level array — the Render flow
	// goes through marshal+unmarshal so this exercises the []any path.
	payload := json.RawMessage(`[{"uri":"x","label":"y"}]`)
	var buf bytes.Buffer
	if err := Render(&buf, payload, Table); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), "uri") {
		t.Fatalf("missing uri column:\n%s", buf.String())
	}
}

func TestToString_FloatRendering(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"plain", "plain"},
		{float64(3), "3"},
		{float64(3.5), "3.5"},
		{true, "true"},
		{[]int{1, 2}, "[1,2]"},
	}
	for _, tc := range cases {
		if got := toString(tc.in); got != tc.want {
			t.Errorf("toString(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
