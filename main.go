package main

import (
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/tituscheng/groktc/cmd"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("fatal panic", "panic", r, "stack", string(debug.Stack()))
			os.Exit(1)
		}
	}()

	cmd.Execute()
}
