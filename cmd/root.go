// Package cmd wires the cobra command tree for the itonics CLI.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/itonics/itonics-cli/internal/config"
	"github.com/itonics/itonics-cli/internal/output"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X github.com/.../cmd.Version=...".
var Version = "dev"

// flagConfig holds the --env-file path. Persistent flag, so a single
// package-level binding is fine.
var flagConfig string

var rootCmd = &cobra.Command{
	Use:           "itonics",
	Short:         "Command-line client for the ITONICS Innovation OData v2 API",
	Long:          "Manage elements, element types, files, attachments, watches, likes, comments and views on an ITONICS Innovation tenant.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       Version,
}

// Execute is the entry point called from main.go.
func Execute() error {
	rootCmd.SetVersionTemplate("itonics version {{.Version}}\n")
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfig, "env-file", "", "Path to a .env file to load (default: .env in cwd if present)")
	cobra.OnInitialize(loadDotEnv)
}

func loadDotEnv() {
	if flagConfig != "" {
		if err := godotenv.Load(flagConfig); err != nil {
			fmt.Fprintf(os.Stderr, "warning: --env-file %s: %v\n", flagConfig, err)
		}
		return
	}
	// Best-effort .env in cwd; ignore "not found" errors silently.
	_ = godotenv.Load()
}

// newClient resolves credentials from the layered config and returns an api.Client.
//
// Precedence (highest first):
//  1. ITONICS_DOMAIN / ITONICS_API_KEY / ITONICS_SPACE env vars (set by the
//     shell, by --env-file, or by ./.env via godotenv).
//  2. Persisted profile at ~/.config/itonics/config.toml (written by
//     `itonics login`).
func newClient() (*api.Client, error) {
	p, _, err := config.Resolve()
	if err != nil {
		return nil, err
	}
	missing := []string{}
	if strings.TrimSpace(p.Domain) == "" {
		missing = append(missing, "ITONICS_DOMAIN")
	}
	if strings.TrimSpace(p.APIKey) == "" {
		missing = append(missing, "ITONICS_API_KEY")
	}
	if strings.TrimSpace(p.Space) == "" {
		missing = append(missing, "ITONICS_SPACE")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"missing credentials: %s\nRun `itonics login` or set the env vars",
			strings.Join(missing, ", "),
		)
	}
	c := api.New(p.Domain, p.APIKey, p.Space)
	c.UserAgent = fmt.Sprintf("itonics-cli/%s", Version)
	return c, nil
}

// ctx returns a cancellable command context.
func ctx() context.Context { return context.Background() }

// formatFlagName is the flag we install on every output-producing subcommand.
const formatFlagName = "format"

// addFormatFlag installs --format on c. The value is bound to c.Flags(), not
// to a package-level variable, so concurrent or repeated invocations of the
// same command don't leak state across calls.
func addFormatFlag(c *cobra.Command) {
	c.Flags().String(formatFlagName, "json", "Output format: json or table")
}

// renderCmd resolves --format on cmd and writes v with the selected encoding.
func renderCmd(cmd *cobra.Command, v any) error {
	format := "json"
	if cmd != nil {
		if f, err := cmd.Flags().GetString(formatFlagName); err == nil && f != "" {
			format = f
		}
	}
	if !output.IsValid(format) {
		return fmt.Errorf("invalid --%s %q (want json or table)", formatFlagName, format)
	}
	return output.Render(os.Stdout, v, output.Format(format))
}

// parsePropPairs turns repeated URI=VALUE strings into api.Property entries.
func parsePropPairs(pairs []string) ([]api.Property, error) {
	out := make([]api.Property, 0, len(pairs))
	for _, p := range pairs {
		idx := strings.Index(p, "=")
		if idx <= 0 {
			return nil, fmt.Errorf("invalid --prop %q (expected URI=VALUE)", p)
		}
		out = append(out, api.Property{URI: p[:idx], Value: p[idx+1:]})
	}
	return out, nil
}
