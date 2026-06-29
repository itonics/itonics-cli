package cmd

import (
	"encoding/json"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/spf13/cobra"
)

var viewsCmd = &cobra.Command{
	Use:   "views",
	Short: "Manage saved views (page_view pages)",
}

func init() {
	rootCmd.AddCommand(viewsCmd)
	viewsCmd.AddCommand(viewsListCmd())
	viewsCmd.AddCommand(viewsGetCmd())
	viewsCmd.AddCommand(viewsCreateCmd())
	viewsCmd.AddCommand(viewsUpdateCmd())
	viewsCmd.AddCommand(viewsDeleteCmd())
}

func viewsListCmd() *cobra.Command {
	var (
		filter, orderBy string
		top             int
		raw             bool
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List saved views",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			items, err := cli.ListViews(ctx(), api.ListViewsParams{
				Filter: filter, OrderBy: orderBy, Top: top, RawFieldValues: raw,
			})
			if err != nil {
				return err
			}
			return renderCmd(cmd, map[string]any{"value": items})
		},
	}
	c.Flags().StringVarP(&filter, "filter", "f", "", "OData $filter (label, presetType)")
	c.Flags().StringVar(&orderBy, "orderby", "", "OData $orderby expression")
	c.Flags().IntVarP(&top, "top", "n", 0, "Maximum number of items (fetches across pages)")
	c.Flags().BoolVar(&raw, "raw", false, "Return raw field values (layout as base64)")
	addFormatFlag(c)
	return c
}

func viewsGetCmd() *cobra.Command {
	var raw bool
	c := &cobra.Command{
		Use:   "get URI",
		Short: "Get a single view by URI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.GetView(ctx(), args[0], raw)
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().BoolVar(&raw, "raw", false, "Return raw field values (layout as base64)")
	addFormatFlag(c)
	return c
}

func viewsCreateCmd() *cobra.Command {
	var (
		label, createdBy, preset, visibility, folder, html string
		favorite, raw                                      bool
	)
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a page_view (plain HTML + stock Tailwind, no JavaScript)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			in := api.CreateViewInput{
				Label:      label,
				PresetType: preset,
				CreatedBy:  createdBy,
				Visibility: visibility,
				FolderURI:  folder,
			}
			if html != "" {
				in.Layout = &api.ViewLayout{HTML: html}
			}
			if cmd.Flags().Changed("favorite") {
				f := favorite
				in.IsFavorite = &f
			}
			data, err := cli.CreateViews(ctx(), []api.CreateViewInput{in}, raw)
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&label, "label", "", "View label (required)")
	c.Flags().StringVar(&createdBy, "created-by", "", "Creator email (required)")
	c.Flags().StringVar(&preset, "preset", "page_view", "Preset type (page_view only)")
	c.Flags().StringVar(&visibility, "visibility", "", "Visibility: workspace or private")
	c.Flags().StringVar(&folder, "folder", "", "Folder URI")
	c.Flags().StringVar(&html, "html", "", "Layout: plain HTML + Tailwind, or base64:<...>")
	c.Flags().BoolVar(&favorite, "favorite", false, "Mark as favorite")
	c.Flags().BoolVar(&raw, "raw", false, "Return raw field values")
	_ = c.MarkFlagRequired("label")
	_ = c.MarkFlagRequired("created-by")
	addFormatFlag(c)
	return c
}

func viewsUpdateCmd() *cobra.Command {
	var (
		updatedBy, updLabel, updVisibility, updFolder, updHTML string
		updFavorite, raw                                       bool
	)
	c := &cobra.Command{
		Use:   "update URI",
		Short: "Update a view by URI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			in := api.UpdateViewInput{URI: args[0], UpdatedBy: updatedBy}
			if cmd.Flags().Changed("label") {
				v := updLabel
				in.Label = &v
			}
			if cmd.Flags().Changed("visibility") {
				v := updVisibility
				in.Visibility = &v
			}
			if cmd.Flags().Changed("folder") {
				v := updFolder
				in.FolderURI = &v
			}
			if cmd.Flags().Changed("html") {
				in.Layout = &api.ViewLayout{HTML: updHTML}
			}
			if cmd.Flags().Changed("favorite") {
				f := updFavorite
				in.IsFavorite = &f
			}
			data, err := cli.UpdateViews(ctx(), []api.UpdateViewInput{in}, raw)
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&updatedBy, "updated-by", "", "Updater email (required)")
	c.Flags().StringVar(&updLabel, "label", "", "New label")
	c.Flags().StringVar(&updVisibility, "visibility", "", "Visibility: workspace or private")
	c.Flags().StringVar(&updFolder, "folder", "", "Folder URI")
	c.Flags().StringVar(&updHTML, "html", "", "New layout HTML (plain or base64:<...>)")
	c.Flags().BoolVar(&updFavorite, "favorite", false, "Favorite flag")
	c.Flags().BoolVar(&raw, "raw", false, "Return raw field values")
	_ = c.MarkFlagRequired("updated-by")
	addFormatFlag(c)
	return c
}

func viewsDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "delete URI [URI...]",
		Short: "Delete one or more views by URI",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm("Delete %d view(s)?", len(args)); err != nil {
					return err
				}
			}
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.DeleteViews(ctx(), args)
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
