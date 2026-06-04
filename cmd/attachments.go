package cmd

import (
	"encoding/json"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/spf13/cobra"
)

var attachmentsCmd = &cobra.Command{
	Use:   "attachments",
	Short: "Link uploaded files to elements",
}

func init() {
	rootCmd.AddCommand(attachmentsCmd)
	attachmentsCmd.AddCommand(attachmentsListCmd())
	attachmentsCmd.AddCommand(attachmentsAttachCmd())
	attachmentsCmd.AddCommand(attachmentsDetachCmd())
}

func attachmentsListCmd() *cobra.Command {
	var (
		selectFields, orderBy string
		top, skip             int
	)
	c := &cobra.Command{
		Use:   "list ELEMENT_URI",
		Short: "List attachments linked to an element",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.ListAttachments(ctx(), args[0], api.ListAttachmentsParams{
				Select: selectFields, OrderBy: orderBy, Top: top, Skip: skip,
			})
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&selectFields, "select", "", "Comma-separated fields to select")
	c.Flags().StringVar(&orderBy, "orderby", "", "createdOn|fileName|mimeType|size [asc|desc]")
	c.Flags().IntVarP(&top, "top", "n", 0, "Max items")
	c.Flags().IntVar(&skip, "skip", 0, "Skip N items")
	addFormatFlag(c)
	return c
}

func attachmentsAttachCmd() *cobra.Command {
	var attachedBy string
	c := &cobra.Command{
		Use:   "attach ELEMENT_URI FILE_URI [FILE_URI...]",
		Short: "Link previously-uploaded files to an element by fileUri",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.AttachFiles(ctx(), args[0], args[1:], attachedBy)
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&attachedBy, "attached-by", "", "Email attributed for the attach (required)")
	_ = c.MarkFlagRequired("attached-by")
	addFormatFlag(c)
	return c
}

func attachmentsDetachCmd() *cobra.Command {
	var detachedBy string
	c := &cobra.Command{
		Use:   "detach ELEMENT_URI FILE_URI [FILE_URI...]",
		Short: "Unlink files from an element",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.DetachFiles(ctx(), args[0], args[1:], detachedBy)
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&detachedBy, "detached-by", "", "Email attributed for the detach (required)")
	_ = c.MarkFlagRequired("detached-by")
	addFormatFlag(c)
	return c
}
