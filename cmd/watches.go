package cmd

import (
	"encoding/json"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/spf13/cobra"
)

var watchesCmd = &cobra.Command{
	Use:   "watches",
	Short: "Manage element watchers",
}

func init() {
	rootCmd.AddCommand(watchesCmd)
	watchesCmd.AddCommand(watchesListCmd())
	watchesCmd.AddCommand(watchesAddCmd())
	watchesCmd.AddCommand(watchesRemoveCmd())
}

func watchesListCmd() *cobra.Command {
	var (
		orderBy   string
		top, skip int
	)
	c := &cobra.Command{
		Use:   "list ELEMENT_URI",
		Short: "List users watching an element",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.ListWatches(ctx(), args[0], api.ListWatchesParams{
				OrderBy: orderBy, Top: top, Skip: skip,
			})
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&orderBy, "orderby", "", "UserURI [asc|desc] (default asc)")
	c.Flags().IntVarP(&top, "top", "n", 0, "Max items")
	c.Flags().IntVar(&skip, "skip", 0, "Skip N items")
	addFormatFlag(c)
	return c
}

func watchesAddCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add ELEMENT_URI USER_URI [USER_URI...]",
		Short: "Add one or more users as watchers (max 100)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.AddWatches(ctx(), args[0], args[1:])
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	addFormatFlag(c)
	return c
}

func watchesRemoveCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "remove ELEMENT_URI [USER_URI...]",
		Short: "Remove watchers (empty list = no-op)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			if err := cli.RemoveWatches(ctx(), args[0], args[1:]); err != nil {
				return err
			}
			return renderCmd(cmd, map[string]any{"removed": args[1:], "element": args[0]})
		},
	}
	addFormatFlag(c)
	return c
}
