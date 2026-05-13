package cmd

import (
	"context"

	"github.com/tituscheng/groktc/internal/model"

	"github.com/spf13/cobra"
)

var (
	modelSetID string
	modelList  bool

	newModelRunner = model.NewRunner
)

var modelCmd = &cobra.Command{
	Use:          "model",
	Short:        "List and select the default xAI model",
	Long:         "List available xAI language models and select the default model used by groktc.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := newModelRunner()
		return runner.Run(context.Background(), model.RunnerOptions{
			SetModelID: modelSetID,
			ListOnly:   modelList,
		})
	},
}

func init() {
	modelCmd.Flags().StringVar(&modelSetID, "set", "", "set the selected default model by ID or alias")
	modelCmd.Flags().BoolVar(&modelList, "list", false, "list models and current selection without interactive picker")
	rootCmd.AddCommand(modelCmd)
}
