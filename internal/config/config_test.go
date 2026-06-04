package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withIsolatedConfig redirects Path() and the env-var inputs into a temp dir
// for the duration of fn. It restores env state on cleanup.
func withIsolatedConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ITONICS_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("ITONICS_DOMAIN", "")
	t.Setenv("ITONICS_API_KEY", "")
	t.Setenv("ITONICS_SPACE", "")
	// Force XDG to a known-empty value so Path() doesn't accidentally pick up
	// the developer's real config when ITONICS_CONFIG is unset (it isn't,
	// but belt-and-braces).
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestProfile_MaskedKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want string
	}{
		{"empty", "", ""},
		{"short", "abc", "•••"},
		{"exactly_8", "abcdefgh", "••••••••"},
		{"longer", "abcdefghij", "abcd••••••••ghij"},
		{"realistic", "AKIAIOSFODNN7EXAMPLE", "AKIA••••••••MPLE"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := Profile{APIKey: tc.key}.MaskedKey()
			if got != tc.want {
				t.Fatalf("MaskedKey(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestProfile_Empty(t *testing.T) {
	if !(Profile{}).Empty() {
		t.Fatal("zero Profile should report Empty()")
	}
	if (Profile{Domain: "https://x"}).Empty() {
		t.Fatal("non-empty Domain should not be Empty()")
	}
	if !(Profile{Domain: "   "}).Empty() {
		t.Fatal("whitespace-only Domain should be Empty()")
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	withIsolatedConfig(t)

	in := Profile{
		Domain: "https://acme.itonics.io",
		APIKey: "sk-secret-1234",
		Space:  "SPACE_URI",
	}
	path, err := Save(in)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Fatalf("file mode = %v, want 0o600", mode)
	}

	got, gotPath, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotPath != path {
		t.Fatalf("Load returned path %q, want %q", gotPath, path)
	}
	if got != in {
		t.Fatalf("round-trip mismatch:\n got: %+v\nwant: %+v", got, in)
	}
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	withIsolatedConfig(t)
	p, _, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if !p.Empty() {
		t.Fatalf("Load on missing file = %+v, want empty", p)
	}
}

func TestSave_OverwritesAndTightensMode(t *testing.T) {
	dir := withIsolatedConfig(t)
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("# pre-existing\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := Save(Profile{Domain: "https://x", APIKey: "k", Space: "s"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Fatalf("Save did not tighten mode: got %v, want 0o600", mode)
	}
}

func TestResolve_EnvBeatsFile(t *testing.T) {
	withIsolatedConfig(t)
	// Persist a baseline file profile.
	if _, err := Save(Profile{
		Domain: "https://file.example",
		APIKey: "FILE_KEY",
		Space:  "FILE_SPACE",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv("ITONICS_DOMAIN", "https://env.example")
	t.Setenv("ITONICS_API_KEY", "ENV_KEY")
	// Leave ITONICS_SPACE unset — file should win for that one field.

	p, sources, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Domain != "https://env.example" {
		t.Fatalf("Domain = %q, want env value", p.Domain)
	}
	if p.APIKey != "ENV_KEY" {
		t.Fatalf("APIKey = %q, want env value", p.APIKey)
	}
	if p.Space != "FILE_SPACE" {
		t.Fatalf("Space = %q, want file fallback", p.Space)
	}
	if !strings.HasPrefix(sources["domain"], "env(") {
		t.Fatalf("domain source = %q, want env(...)", sources["domain"])
	}
	if !strings.HasPrefix(sources["space"], "file(") {
		t.Fatalf("space source = %q, want file(...)", sources["space"])
	}
}

func TestResolve_AllEnv_NoFileRead(t *testing.T) {
	withIsolatedConfig(t)
	// Note: no Save() — the file does not exist.
	t.Setenv("ITONICS_DOMAIN", "https://env.example")
	t.Setenv("ITONICS_API_KEY", "ENV_KEY")
	t.Setenv("ITONICS_SPACE", "ENV_SPACE")

	p, sources, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Domain != "https://env.example" || p.APIKey != "ENV_KEY" || p.Space != "ENV_SPACE" {
		t.Fatalf("Resolve = %+v, want env triple", p)
	}
	for k, v := range sources {
		if !strings.HasPrefix(v, "env(") {
			t.Fatalf("source[%s] = %q, want env(...)", k, v)
		}
	}
}

func TestParseTOML_TolerantOfQuotesAndComments(t *testing.T) {
	in := []byte(`# header comment
domain = "https://x"
api_key = 'literal-string'
space   = SPACE_NO_QUOTES
# trailing
`)
	p, err := parseTOML(in)
	if err != nil {
		t.Fatalf("parseTOML: %v", err)
	}
	if p.Domain != "https://x" {
		t.Fatalf("Domain = %q", p.Domain)
	}
	if p.APIKey != "literal-string" {
		t.Fatalf("APIKey = %q", p.APIKey)
	}
	if p.Space != "SPACE_NO_QUOTES" {
		t.Fatalf("Space = %q", p.Space)
	}
}

func TestRenderTOML_EmitsQuotedAndSkipsZeroFields(t *testing.T) {
	out := renderTOML(Profile{Domain: "https://x", Space: "S"})
	if !strings.Contains(out, `domain  = "https://x"`) {
		t.Fatalf("missing quoted domain:\n%s", out)
	}
	if strings.Contains(out, "api_key") {
		t.Fatalf("rendered api_key for empty key:\n%s", out)
	}
}

func TestDelete_AbsentIsNoOp(t *testing.T) {
	withIsolatedConfig(t)
	_, removed, err := Delete()
	if err != nil {
		t.Fatalf("Delete on absent: %v", err)
	}
	if removed {
		t.Fatal("Delete on absent reported removed=true")
	}
}

func TestDelete_PresentReturnsTrue(t *testing.T) {
	withIsolatedConfig(t)
	if _, err := Save(Profile{Domain: "https://x", APIKey: "k", Space: "s"}); err != nil {
		t.Fatal(err)
	}
	_, removed, err := Delete()
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !removed {
		t.Fatal("Delete reported removed=false but file existed")
	}
}
