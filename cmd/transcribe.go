package cmd

import "github.com/tituscheng/groktc/internal/transcribe"

func init() {
	rootCmd.AddCommand(transcribe.NewCommand())
}
