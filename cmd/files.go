package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Upload and inspect files",
}

func init() {
	rootCmd.AddCommand(filesCmd)
	filesCmd.AddCommand(filesUploadCmd())
	filesCmd.AddCommand(filesGetCmd())
}

func filesUploadCmd() *cobra.Command {
	var createdBy, contentType, attachTo, attachedBy string
	c := &cobra.Command{
		Use:   "upload PATH",
		Short: "Upload a local file (2-step: request URL, PUT binary)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			fileURI, err := cli.UploadFile(ctx(), args[0], createdBy, contentType)
			if err != nil {
				return err
			}
			result := map[string]any{"fileUri": fileURI}
			if attachTo != "" {
				by := attachedBy
				if by == "" {
					by = createdBy
				}
				att, err := cli.AttachFiles(ctx(), attachTo, []string{fileURI}, by)
				if err != nil {
					return err
				}
				result["attachment"] = json.RawMessage(att)
			}
			return renderCmd(cmd, result)
		},
	}
	c.Flags().StringVar(&createdBy, "created-by", "", "Uploader email (required)")
	c.Flags().StringVar(&contentType, "content-type", "", "MIME type (auto-detected from extension if omitted)")
	c.Flags().StringVar(&attachTo, "attach-to", "", "Element URI to attach the file to immediately after upload")
	c.Flags().StringVar(&attachedBy, "attached-by", "", "Email used as attachedBy when --attach-to is set (defaults to --created-by)")
	_ = c.MarkFlagRequired("created-by")
	addFormatFlag(c)
	return c
}

func filesGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "get FILE_URI",
		Short: "Get file metadata + pre-signed download URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.GetFileDetails(ctx(), args[0])
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	addFormatFlag(c)
	return c
}
