package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/itonics/itonics-cli/internal/api"
	"github.com/spf13/cobra"
)

var typesCmd = &cobra.Command{
	Use:   "types",
	Short: "Manage element types",
}

func init() {
	rootCmd.AddCommand(typesCmd)
	typesCmd.AddCommand(typesListCmd())
	typesCmd.AddCommand(typesCreateCmd())
}

func typesListCmd() *cobra.Command {
	var filter, orderBy string
	c := &cobra.Command{
		Use:   "list",
		Short: "List element types",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, err := newClient()
			if err != nil {
				return err
			}
			items, err := cli.ListElementTypes(ctx(), filter, orderBy)
			if err != nil {
				return err
			}
			return renderCmd(cmd, map[string]any{"elementTypes": items})
		},
	}
	c.Flags().StringVarP(&filter, "filter", "f", "", "OData $filter expression")
	c.Flags().StringVar(&orderBy, "orderby", "", "OData $orderby expression")
	addFormatFlag(c)
	return c
}

func typesCreateCmd() *cobra.Command {
	var label, createdBy, icon string
	var propsLabelType []string
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new element type",
		RunE: func(cmd *cobra.Command, _ []string) error {
			props := make([]api.ElementTypeProperty, 0, len(propsLabelType))
			for _, p := range propsLabelType {
				idx := strings.Index(p, ":")
				if idx <= 0 {
					return fmt.Errorf("invalid --prop %q (expected LABEL:TYPE)", p)
				}
				props = append(props, api.ElementTypeProperty{Label: p[:idx], Type: p[idx+1:]})
			}
			cli, err := newClient()
			if err != nil {
				return err
			}
			data, err := cli.CreateElementType(ctx(), api.CreateElementTypeInput{
				Label: label, CreatedBy: createdBy, Icon: icon, Properties: props,
			})
			if err != nil {
				return err
			}
			return renderCmd(cmd, json.RawMessage(data))
		},
	}
	c.Flags().StringVar(&label, "label", "", "Element type label (required)")
	c.Flags().StringVar(&createdBy, "created-by", "", "Creator email (required)")
	c.Flags().StringVar(&icon, "icon", "", "Icon name (e.g. rocket, idea, goal)")
	c.Flags().StringSliceVar(&propsLabelType, "prop", nil, "Property as LABEL:TYPE (repeatable)")
	_ = c.MarkFlagRequired("label")
	_ = c.MarkFlagRequired("created-by")
	addFormatFlag(c)
	return c
}
