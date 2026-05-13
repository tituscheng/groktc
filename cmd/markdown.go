package cmd

import "github.com/tituscheng/groktc/internal/markdown"

func init() {
	rootCmd.AddCommand(markdown.NewCommand())
}
