package config

import (
	"fmt"
	"strings"
)

// Minimal TOML serializer/parser for the flat 3-key Profile shape. Avoids a
// dep — if we ever introduce named profiles or nested sections, switch to
// github.com/pelletier/go-toml/v2.

func renderTOML(p Profile) string {
	var b strings.Builder
	b.WriteString("# ITONICS CLI configuration\n")
	b.WriteString("# Stored by `itonics login`. Wipe with `itonics logout`.\n")
	if p.Domain != "" {
		fmt.Fprintf(&b, "domain  = %q\n", p.Domain)
	}
	if p.Space != "" {
		fmt.Fprintf(&b, "space   = %q\n", p.Space)
	}
	if p.APIKey != "" {
		fmt.Fprintf(&b, "api_key = %q\n", p.APIKey)
	}
	return b.String()
}

func parseTOML(data []byte) (Profile, error) {
	var p Profile
	for i, line := range strings.Split(string(data), "\n") {
		raw := strings.TrimSpace(line)
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		eq := strings.Index(raw, "=")
		if eq < 1 {
			return p, fmt.Errorf("line %d: expected key = value", i+1)
		}
		key := strings.TrimSpace(raw[:eq])
		val := strings.TrimSpace(raw[eq+1:])
		// Strip a single layer of double or single quotes.
		if l := len(val); l >= 2 {
			if (val[0] == '"' && val[l-1] == '"') || (val[0] == '\'' && val[l-1] == '\'') {
				val = val[1 : l-1]
			}
		}
		switch key {
		case "domain":
			p.Domain = val
		case "api_key":
			p.APIKey = val
		case "space":
			p.Space = val
		default:
			// Ignore unknown keys; forward-compatible.
		}
	}
	return p, nil
}
