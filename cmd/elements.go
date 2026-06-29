package cmd

import (
	"encoding/json"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/spf13/cobra"
)

var elementsCmd = &cobra.Command{
	Use:   "elements",
	Short: "Manage elements",
}

func init() {
	rootCmd.AddCommand(elementsCmd)
	elementsCmd.AddCommand(elementsListCmd())
	elementsCmd.AddCommand(elementsGetCmd())
	elementsCmd.AddCommand(elementsCreateCmd())
	elementsCmd.AddCommand(elementsUpdateCmd())
	elementsCmd.AddCommand(elementsDeleteCmd())
}

func elementsListCmd() *cobra.Command {
	var (
		filter, selectFields, orderBy string
		top                           int
		raw                           bool
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List elements with optional OData filtering",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			items, err := c.ListElements(ctx(), api.ListElementsParams{
				Filter: filter, Select: selectFields,
				OrderBy: orderBy, Top: top, RawFieldValues: raw,
			})
			if err != nil {
				return err
			}
			return renderCmd(cmd, map[string]any{"elements": items})
		},
	}
	c.Flags().StringVarP(&filter, "filter", "f", "", "OData $filter, e.g. \"contains(label,'x')\" (no full-text $search)")
	c.Flags().StringVar(&selectFields, "select", "", "Comma-separated fields to select")
	c.Flags().StringVar(&orderBy, "orderby", "", "OData $orderby expression")
	c.Flags().IntVarP(&top, "top", "n", 0, "Maximum number of items (fetches across pages)")
	c.Flags().BoolVar(&raw, "raw", false, "Return raw field values (rawFieldValues=1)")
	addFormatFlag(c)
	return c
}

func elementsGetCmd() *cobra.Command {
	var (
		expand string
		getRaw bool
	)
	c := &cobra.Command{
		Use:   "get URI",
		Short: "Get a single element by URI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			el, err := c.GetElement(ctx(), args[0], expand, getRaw)
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(el))
		},
	}
	c.Flags().StringVar(&expand, "expand", "", "OData $expand navigation property")
	c.Flags().BoolVar(&getRaw, "raw", false, "Return raw field values")
	addFormatFlag(c)
	return c
}

func elementsCreateCmd() *cobra.Command {
	var (
		elementType, label, summary, createdBy, status string
		tags                                           []string
		props                                          []string
	)
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new element",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			parsed, err := parsePropPairs(props)
			if err != nil {
				return err
			}
			in := api.CreateElementInput{
				ElementTypeURI: elementType,
				Label:          label,
				Summary:        summary,
				CreatedBy:      createdBy,
				Status:         status,
				Tags:           tags,
				Properties:     parsed,
			}
			data, err := cli.CreateElements(ctx(), []api.CreateElementInput{in}, false)
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&elementType, "type", "", "Element type URI (required)")
	c.Flags().StringVar(&label, "label", "", "Element label (required)")
	c.Flags().StringVar(&summary, "summary", "", "Element summary")
	c.Flags().StringVar(&createdBy, "created-by", "", "Creator email (required)")
	c.Flags().StringVar(&status, "status", "published", "Element status: published, draft, archived, deleted")
	c.Flags().StringSliceVar(&tags, "tag", nil, "Tag (repeatable)")
	c.Flags().StringSliceVar(&props, "prop", nil, "Property as URI=VALUE (repeatable)")
	_ = c.MarkFlagRequired("type")
	_ = c.MarkFlagRequired("label")
	_ = c.MarkFlagRequired("created-by")
	addFormatFlag(c)
	return c
}

func elementsUpdateCmd() *cobra.Command {
	var (
		updatedBy                       string
		updLabel, updSummary, updStatus string
		updTags                         []string
		updProps                        []string
	)
	c := &cobra.Command{
		Use:   "update URI",
		Short: "Update an existing element",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve "was the flag set?" off of cmd.Flags() so concurrent or
			// repeated invocations don't share state via package-level vars.
			setLabel := cmd.Flags().Changed("label")
			setSummary := cmd.Flags().Changed("summary")
			setStatus := cmd.Flags().Changed("status")
			setTags := cmd.Flags().Changed("tag")

			parsed, err := parsePropPairs(updProps)
			if err != nil {
				return err
			}
			client, err := newClient()
			if err != nil {
				return err
			}
			in := api.UpdateElementInput{URI: args[0], UpdatedBy: updatedBy, Properties: parsed}
			if setLabel {
				v := updLabel
				in.Label = &v
			}
			if setSummary {
				v := updSummary
				in.Summary = &v
			}
			if setStatus {
				v := updStatus
				in.Status = &v
			}
			if setTags {
				tagsCopy := append([]string(nil), updTags...)
				in.Tags = &tagsCopy
			}
			data, err := client.UpdateElements(ctx(), []api.UpdateElementInput{in}, false)
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&updatedBy, "updated-by", "", "Updater email (required)")
	c.Flags().StringVar(&updLabel, "label", "", "New label")
	c.Flags().StringVar(&updSummary, "summary", "", "New summary")
	c.Flags().StringVar(&updStatus, "status", "", "New status")
	c.Flags().StringSliceVar(&updTags, "tag", nil, "Tag (repeatable, replaces existing)")
	c.Flags().StringSliceVar(&updProps, "prop", nil, "Property as URI=VALUE (repeatable)")
	_ = c.MarkFlagRequired("updated-by")
	addFormatFlag(c)
	return c
}

func elementsDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "delete URI [URI...]",
		Short: "Delete one or more elements by URI",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				if err := confirm("Delete %d element(s)?", len(args)); err != nil {
					return err
				}
			}
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.DeleteElements(ctx(), args)
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
