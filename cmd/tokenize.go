package cmd

import (
	"context"

	"github.com/tituscheng/groktc/internal/tokenize"

	"github.com/spf13/cobra"
)

const (
	defaultTokenizeModel = "grok-4.3"
)

var (
	tokenizeModel string
	tokenizeYes   bool

	newTokenizeRunner = tokenize.NewRunner
)

var tokenizeCmd = &cobra.Command{
	Use:          "tokenize [files...]",
	Short:        "Estimate xAI input token usage for files",
	Long:         "Estimate xAI input token usage and cost for one or more text files using the xAI tokenizer API.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		modelExplicit := cmd.Flags().Lookup("model").Changed
		runner := newTokenizeRunner()
		return runner.Run(context.Background(), tokenize.Options{
			Args:          args,
			Model:         tokenizeModel,
			ModelExplicit: modelExplicit,
			Yes:           tokenizeYes,
		})
	},
}

func init() {
	tokenizeCmd.Flags().StringVar(&tokenizeModel, "model", defaultTokenizeModel, "xAI model to use for tokenization and cost estimation")
	tokenizeCmd.Flags().BoolVar(&tokenizeYes, "yes", false, "skip interactive confirmation when discovering files in the current directory")
	rootCmd.AddCommand(tokenizeCmd)
}
