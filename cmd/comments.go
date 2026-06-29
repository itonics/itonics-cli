package cmd

import (
	"encoding/json"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/spf13/cobra"
)

var commentsCmd = &cobra.Command{
	Use:   "comments",
	Short: "Manage element comments",
}

func init() {
	rootCmd.AddCommand(commentsCmd)
	commentsCmd.AddCommand(commentsListCmd())
	commentsCmd.AddCommand(commentsCreateCmd())
	commentsCmd.AddCommand(commentsUpdateCmd())
	commentsCmd.AddCommand(commentsDeleteCmd())
}

func commentsListCmd() *cobra.Command {
	var (
		filter, selectFields, orderBy string
		top                           int
	)
	c := &cobra.Command{
		Use:   "list ELEMENT_URI",
		Short: "List comments on an element",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			items, err := cli.ListComments(ctx(), args[0], api.ListCommentsParams{
				Filter: filter, Select: selectFields, OrderBy: orderBy, Top: top,
			})
			if err != nil {
				return err
			}
			return renderCmd(cmd, map[string]any{"value": items})
		},
	}
	c.Flags().StringVarP(&filter, "filter", "f", "", "OData $filter (commentedBy/updatedBy emails, createdOn/updatedOn)")
	c.Flags().StringVar(&selectFields, "select", "", "Comma-separated fields to select")
	c.Flags().StringVar(&orderBy, "orderby", "", "OData $orderby (createdOn|updatedOn|commentedBy)")
	c.Flags().IntVarP(&top, "top", "n", 0, "Maximum number of items (fetches across pages)")
	addFormatFlag(c)
	return c
}

func commentsCreateCmd() *cobra.Command {
	var commentedBy, text string
	c := &cobra.Command{
		Use:   "create ELEMENT_URI",
		Short: "Add a comment to an element",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			in := api.CreateCommentInput{CommentedBy: commentedBy, Text: text}
			data, err := cli.CreateComments(ctx(), args[0], []api.CreateCommentInput{in})
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&commentedBy, "commented-by", "", "Author email (required)")
	c.Flags().StringVar(&text, "text", "", "Comment text or HTML; @email mentions and #{label} refs (required)")
	_ = c.MarkFlagRequired("commented-by")
	_ = c.MarkFlagRequired("text")
	addFormatFlag(c)
	return c
}

func commentsUpdateCmd() *cobra.Command {
	var updatedBy, text string
	c := &cobra.Command{
		Use:   "update ELEMENT_URI COMMENT_URI",
		Short: "Edit a comment by URI",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			in := api.UpdateCommentInput{CommentURI: args[1], UpdatedBy: updatedBy, Text: text}
			data, err := cli.UpdateComments(ctx(), args[0], []api.UpdateCommentInput{in})
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&updatedBy, "updated-by", "", "Updater email (required)")
	c.Flags().StringVar(&text, "text", "", "New comment text or HTML")
	_ = c.MarkFlagRequired("updated-by")
	addFormatFlag(c)
	return c
}

func commentsDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "delete ELEMENT_URI COMMENT_URI [COMMENT_URI...]",
		Short: "Delete one or more comments from an element by URI",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm("Delete %d comment(s)?", len(args[1:])); err != nil {
					return err
				}
			}
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.DeleteComments(ctx(), args[0], args[1:])
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	addFormatFlag(c)
	return c
}
