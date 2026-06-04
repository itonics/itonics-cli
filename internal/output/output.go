// Package output renders API responses as JSON or a simple ASCII table.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Format is the requested output mode.
type Format string

const (
	JSON  Format = "json"
	Table Format = "table"
)

// IsValid reports whether s is a supported Format.
func IsValid(s string) bool {
	return s == string(JSON) || s == string(Table)
}

// Render writes data to w in the requested format. raw is the API payload
// (struct or json.RawMessage); table mode tries to auto-detect columns when
// the payload is a list of objects with uri/label fields.
func Render(w io.Writer, raw any, f Format) error {
	if f == JSON || f == "" {
		return writeJSON(w, raw)
	}
	// Table: marshal then re-decode to a flexible form so we can introspect.
	buf, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var any1 any
	if err := json.Unmarshal(buf, &any1); err != nil {
		return err
	}
	if rows, ok := asRowList(any1); ok && len(rows) > 0 {
		return writeTable(w, rows)
	}
	// Fall back to JSON.
	return writeJSON(w, raw)
}

func writeJSON(w io.Writer, raw any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(raw)
}

// asRowList tries to extract a list of objects suitable for tabular display.
// Accepts: top-level slice, {"elements":[...]}, {"elementTypes":[...]},
// {"value":[...]}.
func asRowList(v any) ([]map[string]any, bool) {
	switch t := v.(type) {
	case []any:
		return toRows(t), true
	case map[string]any:
		for _, key := range []string{"elements", "elementTypes", "value"} {
			if arr, ok := t[key].([]any); ok {
				return toRows(arr), true
			}
		}
	}
	return nil, false
}

func toRows(arr []any) []map[string]any {
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func writeTable(w io.Writer, rows []map[string]any) error {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No results.")
		return nil
	}
	cols := pickColumns(rows[0])
	widths := make(map[string]int, len(cols))
	for _, c := range cols {
		widths[c] = len(c)
	}
	for _, r := range rows {
		for _, c := range cols {
			if l := len(truncate(toString(r[c]), 50)); l > widths[c] {
				widths[c] = l
			}
		}
	}
	// Header.
	fmt.Fprintln(w, joinCols(cols, widths, func(c string) string { return c }))
	// Underline.
	fmt.Fprintln(w, joinCols(cols, widths, func(c string) string { return strings.Repeat("-", widths[c]) }))
	for _, r := range rows {
		fmt.Fprintln(w, joinCols(cols, widths, func(c string) string {
			return truncate(toString(r[c]), 50)
		}))
	}
	return nil
}

func pickColumns(sample map[string]any) []string {
	preferred := []string{"uri", "fileUri", "label", "fileName", "status", "elementType", "userUri", "mimeType", "size", "description"}
	cols := []string{}
	for _, k := range preferred {
		if _, ok := sample[k]; ok {
			cols = append(cols, k)
		}
	}
	if len(cols) == 0 {
		for k := range sample {
			cols = append(cols, k)
			if len(cols) >= 5 {
				break
			}
		}
	}
	return cols
}

func joinCols(cols []string, widths map[string]int, val func(string) string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = padRight(val(c), widths[c])
	}
	return strings.Join(parts, "  ")
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		// JSON numbers come back as float64; render integers cleanly.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		return fmt.Sprintf("%t", t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
