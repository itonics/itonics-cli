// Package config persists ITONICS CLI credentials under the user's config
// directory (~/.config/itonics/config.toml by default) and resolves them from
// the layered sources documented in Lookup.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Profile is the persisted on-disk shape. Keys are lowercase to keep the TOML
// file human-readable.
type Profile struct {
	Domain string `toml:"domain"`
	APIKey string `toml:"api_key"`
	Space  string `toml:"space"`
}

// Empty reports whether the profile has no meaningful content.
func (p Profile) Empty() bool {
	return strings.TrimSpace(p.Domain) == "" &&
		strings.TrimSpace(p.APIKey) == "" &&
		strings.TrimSpace(p.Space) == ""
}

// MaskedKey returns a fixed-length masked form of APIKey for display.
func (p Profile) MaskedKey() string {
	k := strings.TrimSpace(p.APIKey)
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return strings.Repeat("•", len(k))
	}
	return k[:4] + strings.Repeat("•", 8) + k[len(k)-4:]
}

// Path returns the on-disk config path. Honors $ITONICS_CONFIG, then
// $XDG_CONFIG_HOME/itonics/config.toml, then ~/.config/itonics/config.toml.
func Path() (string, error) {
	if p := strings.TrimSpace(os.Getenv("ITONICS_CONFIG")); p != "" {
		return p, nil
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "itonics", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "itonics", "config.toml"), nil
}

// Load reads the persisted profile. Returns an empty Profile (no error) when
// the file does not exist.
func Load() (Profile, string, error) {
	path, err := Path()
	if err != nil {
		return Profile{}, "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Profile{}, path, nil
	}
	if err != nil {
		return Profile{}, path, fmt.Errorf("read %s: %w", path, err)
	}
	p, err := parseTOML(data)
	if err != nil {
		return Profile{}, path, fmt.Errorf("parse %s: %w", path, err)
	}
	return p, path, nil
}

// Save writes the profile with mode 0600.
func Save(p Profile) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return path, fmt.Errorf("create config dir: %w", err)
	}
	body := renderTOML(p)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return path, fmt.Errorf("write %s: %w", path, err)
	}
	// Re-set permissions in case the file already existed with looser bits.
	_ = os.Chmod(path, 0o600)
	return path, nil
}

// Delete removes the persisted profile. Returns the path it tried to remove.
// Returns nil if the file was already absent.
func Delete() (string, bool, error) {
	path, err := Path()
	if err != nil {
		return "", false, err
	}
	err = os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return path, false, nil
	}
	if err != nil {
		return path, false, err
	}
	return path, true, nil
}

// Resolve merges sources in the documented precedence and returns the
// effective profile along with a human-readable origin label per field.
//
// Precedence (highest first):
//  1. Explicit env vars (ITONICS_DOMAIN / ITONICS_API_KEY / ITONICS_SPACE) —
//     these already include anything loaded from .env / --env-file by cobra.
//  2. Persisted profile at config.Path().
func Resolve() (Profile, map[string]string, error) {
	on := func(_ string, val string, key string) (string, string) {
		v := strings.TrimSpace(val)
		if v == "" {
			return "", ""
		}
		return v, "env(" + key + ")"
	}
	d, dSrc := on("domain", os.Getenv("ITONICS_DOMAIN"), "ITONICS_DOMAIN")
	k, kSrc := on("api_key", os.Getenv("ITONICS_API_KEY"), "ITONICS_API_KEY")
	s, sSrc := on("space", os.Getenv("ITONICS_SPACE"), "ITONICS_SPACE")

	if d == "" || k == "" || s == "" {
		stored, path, err := Load()
		if err != nil {
			return Profile{}, nil, err
		}
		if d == "" && strings.TrimSpace(stored.Domain) != "" {
			d = stored.Domain
			dSrc = "file(" + path + ")"
		}
		if k == "" && strings.TrimSpace(stored.APIKey) != "" {
			k = stored.APIKey
			kSrc = "file(" + path + ")"
		}
		if s == "" && strings.TrimSpace(stored.Space) != "" {
			s = stored.Space
			sSrc = "file(" + path + ")"
		}
	}
	return Profile{Domain: d, APIKey: k, Space: s}, map[string]string{
		"domain":  dSrc,
		"api_key": kSrc,
		"space":   sSrc,
	}, nil
}
