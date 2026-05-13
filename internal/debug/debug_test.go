package debug

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tituscheng/groktc/internal/config"
)

func TestInitCreatesAndTruncatesLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// First init: create file and write something.
	if err := Init(); err != nil {
		t.Fatalf("Init() first call failed: %v", err)
	}
	slog.Error("first message")
	if err := Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	logPath := filepath.Join(tmpDir, config.DirectoryName, config.DebugLogName)
	firstContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(firstContent), "first message") {
		t.Fatalf("log should contain first message, got: %s", firstContent)
	}

	// Second init: should truncate.
	if err := Init(); err != nil {
		t.Fatalf("Init() second call failed: %v", err)
	}
	slog.Error("second message")
	if err := Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	secondContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if strings.Contains(string(secondContent), "first message") {
		t.Fatal("log should have been truncated, but still contains first message")
	}
	if !strings.Contains(string(secondContent), "second message") {
		t.Fatalf("log should contain second message, got: %s", secondContent)
	}
}

func TestInitWritesValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	slog.Error("test error", "key", "value")

	logPath := filepath.Join(tmpDir, config.DirectoryName, config.DebugLogName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("log file is empty")
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, lines[0])
	}

	if record["level"] != "ERROR" {
		t.Fatalf("expected level ERROR, got %v", record["level"])
	}
	if record["msg"] != "test error" {
		t.Fatalf("expected msg 'test error', got %v", record["msg"])
	}
	if record["key"] != "value" {
		t.Fatalf("expected key 'value', got %v", record["key"])
	}
}

func TestCloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("first Close() failed: %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("second Close() failed: %v", err)
	}
}
