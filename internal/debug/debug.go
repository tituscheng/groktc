// Package debug provides structured logging to a debug log file for
// diagnostic purposes. The log file is reset on every subcommand invocation.
package debug

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/tituscheng/groktc/internal/config"
)

var file *os.File

// Init creates (or truncates) the debug log file and sets the default slog
// logger to write JSON records to it. Callers should defer Close().
func Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, config.DirectoryName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("create config directory %q: %w", configDir, err)
	}

	logPath := filepath.Join(configDir, config.DebugLogName)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open debug log %q: %w", logPath, err)
	}
	file = f

	handler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
	return nil
}

// Close flushes and closes the debug log file. It is safe to call multiple
// times.
func Close() error {
	if file == nil {
		return nil
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close debug log: %w", err)
	}
	file = nil
	return nil
}
