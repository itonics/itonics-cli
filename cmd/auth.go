package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/itonics/itonics-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	rootCmd.AddCommand(loginCmd())
	rootCmd.AddCommand(logoutCmd())
	rootCmd.AddCommand(whoamiCmd())
}

// ---------------------------------------------------------------------------
// login
// ---------------------------------------------------------------------------

func loginCmd() *cobra.Command {
	var (
		domain    string
		space     string
		apiKey    string
		stdinKey  bool
		skipCheck bool
	)
	c := &cobra.Command{
		Use:   "login",
		Short: "Save ITONICS credentials (domain, API key, space) to ~/.config/itonics/config.toml",
		Long: `Interactively prompts for the tenant domain, space URI, and API key, then
persists them to a per-user config file (chmod 0600). Existing values are
shown as defaults so re-running login is a quick way to rotate the API key.

Non-interactive usage:
  itonics login --domain ... --space ... --api-key SECRET
  echo $KEY | itonics login --domain ... --space ... --api-key -

By default the new credentials are validated with a lightweight GET against
the API. Pass --skip-check to skip that round-trip.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			existing, _, err := config.Load()
			if err != nil {
				return err
			}

			r := bufio.NewReader(os.Stdin)
			var p config.Profile
			if p.Domain, err = resolveString("ITONICS domain", domain, existing.Domain, r, normalizeDomain); err != nil {
				return err
			}
			if p.Space, err = resolveString("Space URI", space, existing.Space, r, identity); err != nil {
				return err
			}
			if p.APIKey, err = resolveAPIKey(apiKey, stdinKey, existing.APIKey, r); err != nil {
				return err
			}

			if !skipCheck {
				if err := verify(p); err != nil {
					return fmt.Errorf("credentials check failed: %w", err)
				}
			}
			path, err := config.Save(p)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Saved credentials to %s\n", path)
			return nil
		},
	}
	c.Flags().StringVar(&domain, "domain", "", "Tenant domain (e.g. https://acme.itonics.io)")
	c.Flags().StringVar(&space, "space", "", "Space URI")
	c.Flags().StringVar(&apiKey, "api-key", "", "API key. Pass '-' to read from stdin.")
	c.Flags().BoolVar(&stdinKey, "stdin", false, "Read the API key from stdin (alias for --api-key -)")
	c.Flags().BoolVar(&skipCheck, "skip-check", false, "Skip the verification round-trip against the API")
	return c
}

// ---------------------------------------------------------------------------
// logout
// ---------------------------------------------------------------------------

func logoutCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "logout",
		Short: "Remove the saved ITONICS credentials",
		RunE: func(_ *cobra.Command, _ []string) error {
			path, _ := config.Path()
			if !yes {
				if err := confirm("Delete saved credentials at %s?", path); err != nil {
					return err
				}
			}
			path, removed, err := config.Delete()
			if err != nil {
				return err
			}
			if !removed {
				fmt.Fprintf(os.Stderr, "No saved credentials at %s.\n", path)
				return nil
			}
			fmt.Fprintf(os.Stderr, "Removed %s.\n", path)
			return nil
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	return c
}

// ---------------------------------------------------------------------------
// whoami
// ---------------------------------------------------------------------------

func whoamiCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "whoami",
		Short: "Show which credentials would be used and where they come from",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, sources, err := config.Resolve()
			if err != nil {
				return err
			}
			payload := map[string]any{
				"domain":          p.Domain,
				"space":           p.Space,
				"api_key_masked":  p.MaskedKey(),
				"api_key_present": strings.TrimSpace(p.APIKey) != "",
				"sources":         sources,
			}
			if path, _ := config.Path(); path != "" {
				payload["config_path"] = path
			}
			return renderCmd(cmd, payload)
		},
	}
	addFormatFlag(c)
	return c
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func resolveString(label, fromFlag, fromExisting string, r *bufio.Reader, normalize func(string) string) (string, error) {
	if v := strings.TrimSpace(fromFlag); v != "" {
		return normalize(v), nil
	}
	if !isInteractive() {
		if v := strings.TrimSpace(fromExisting); v != "" {
			return normalize(v), nil
		}
		return "", fmt.Errorf("non-interactive: %s required (pass --%s)", label, lowerLabelFlag(label))
	}
	prompt := label
	if d := strings.TrimSpace(fromExisting); d != "" {
		prompt = fmt.Sprintf("%s [%s]", label, d)
	}
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	val := strings.TrimSpace(line)
	if val == "" {
		val = strings.TrimSpace(fromExisting)
	}
	if val == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return normalize(val), nil
}

func resolveAPIKey(fromFlag string, stdinKey bool, existing string, r *bufio.Reader) (string, error) {
	if fromFlag == "-" || stdinKey {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		v := strings.TrimSpace(string(raw))
		if v == "" {
			return "", errors.New("api-key: empty input on stdin")
		}
		return v, nil
	}
	if v := strings.TrimSpace(fromFlag); v != "" {
		return v, nil
	}
	if !isInteractive() {
		if v := strings.TrimSpace(existing); v != "" {
			return v, nil
		}
		return "", errors.New("non-interactive: API key required (pass --api-key)")
	}
	prompt := "API key"
	if strings.TrimSpace(existing) != "" {
		prompt = "API key [keep current]"
	}
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	val := strings.TrimSpace(string(bytes))
	if val == "" {
		val = strings.TrimSpace(existing)
	}
	if val == "" {
		return "", errors.New("API key is required")
	}
	return val, nil
}

// verifyTimeout bounds the credential-check round-trip during `itonics login`.
// Long enough for a sleepy tenant to respond, short enough that a bad domain
// fails the login quickly.
const verifyTimeout = 15 * time.Second

// verify makes a cheap GET to confirm the credentials work.
func verify(p config.Profile) error {
	c := api.New(p.Domain, p.APIKey, p.Space)
	c.UserAgent = "itonics-cli/" + Version
	ctx, cancel := context.WithTimeout(context.Background(), verifyTimeout)
	defer cancel()
	_, err := c.ListElementTypes(ctx, "", "")
	return err
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

func identity(s string) string { return s }

func normalizeDomain(s string) string {
	v := strings.TrimRight(strings.TrimSpace(s), "/")
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
		v = "https://" + v
	}
	if u, err := url.Parse(v); err == nil {
		return strings.TrimRight(u.String(), "/")
	}
	return v
}

func lowerLabelFlag(label string) string {
	switch strings.ToLower(label) {
	case "itonics domain":
		return "domain"
	case "space uri":
		return "space"
	case "api key":
		return "api-key"
	default:
		return strings.ReplaceAll(strings.ToLower(label), " ", "-")
	}
}
