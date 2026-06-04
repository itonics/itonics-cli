package cmd

import (
	"encoding/json"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/spf13/cobra"
)

var likesCmd = &cobra.Command{
	Use:   "likes",
	Short: "Manage element likes",
}

func init() {
	rootCmd.AddCommand(likesCmd)
	likesCmd.AddCommand(likesListCmd())
	likesCmd.AddCommand(likesAddCmd())
	likesCmd.AddCommand(likesRemoveCmd())
}

func likesListCmd() *cobra.Command {
	var (
		filter, orderBy, selectFields string
		top, skip                     int
	)
	c := &cobra.Command{
		Use:   "list ELEMENT_URI",
		Short: "List users who liked an element",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.ListLikes(ctx(), args[0], api.ListLikesParams{
				Filter: filter, OrderBy: orderBy, Select: selectFields,
				Top: top, Skip: skip,
			})
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVarP(&filter, "filter", "f", "", "OData $filter (UserURI eq, likedOn gt 'ISO-8601')")
	c.Flags().StringVar(&orderBy, "orderby", "", "Default: likedOn asc, then UserURI")
	c.Flags().StringVar(&selectFields, "select", "", "Comma-separated fields to select")
	c.Flags().IntVarP(&top, "top", "n", 0, "Max items")
	c.Flags().IntVar(&skip, "skip", 0, "Skip N items")
	addFormatFlag(c)
	return c
}

func likesAddCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add ELEMENT_URI USER_URI [USER_URI...]",
		Short: "Add one or more likes by user URI (max 100 per request)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.AddLikes(ctx(), args[0], args[1:])
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	addFormatFlag(c)
	return c
}

func likesRemoveCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "remove ELEMENT_URI [USER_URI...]",
		Short: "Remove likes (empty list = no-op)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			if err := cli.RemoveLikes(ctx(), args[0], args[1:]); err != nil {
				return err
			}
			return renderCmd(cmd, map[string]any{"removed": args[1:], "element": args[0]})
		},
	}
	addFormatFlag(c)
	return c
}
