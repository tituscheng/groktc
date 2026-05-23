package cmd

import "github.com/tituscheng/groktc/internal/estimate"

func init() {
	rootCmd.AddCommand(estimate.NewCommand())
}
