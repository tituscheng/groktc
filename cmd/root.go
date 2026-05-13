package cmd

import (
	"log/slog"
	"os"
	"runtime/debug"

	appdebug "github.com/tituscheng/groktc/internal/debug"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "groktc",
	Short:         "A groktc cmd utility",
	Long:          "A groktc cmd utility",
	SilenceErrors: true,
}

func Execute() {
	if err := appdebug.Init(); err != nil {
		color.Red("debug log init failed: %v", err)
		os.Exit(1)
	}
	defer appdebug.Close()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered", "panic", r, "stack", string(debug.Stack()))
			appdebug.Close()
			panic(r)
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		slog.Error("command execution failed", "error", err)
		appdebug.Close()
		color.Red(err.Error())
		os.Exit(1)
	}
}
