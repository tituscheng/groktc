package estimate

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tituscheng/groktc/internal/transcribe"
)

type fakeProber struct {
	durations map[string]float64
	bitrates  map[string]int64
}

func (f *fakeProber) CheckInstalled() error { return nil }

func (f *fakeProber) ProbeMedia(_ context.Context, path string) (transcribe.MediaProbe, error) {
	duration := f.durations[path]
	bitrate := f.bitrates[path]
	if duration == 0 {
		duration = f.durations[filepath.Base(path)]
		bitrate = f.bitrates[filepath.Base(path)]
	}
	return transcribe.MediaProbe{
		DurationSeconds: duration,
		AudioBitrate:    bitrate,
	}, nil
}

func TestRunnerRunExplicitFiles(t *testing.T) {
	dir := t.TempDir()
	okFile := filepath.Join(dir, "ok.mp3")
	overFile := filepath.Join(dir, "big.mp4")
	if err := os.WriteFile(okFile, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("write ok file: %v", err)
	}
	if err := os.WriteFile(overFile, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("write over-limit file: %v", err)
	}

	var stdout bytes.Buffer
	runner := &Runner{
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		Estimator: &Estimator{
			ProberFactory: func() transcribe.MediaProber {
				return &fakeProber{
					durations: map[string]float64{
						"ok.mp3":  3600,
						"big.mp4": 7200,
					},
					bitrates: map[string]int64{
						"big.mp4": 640000,
					},
				}
			},
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(_ io.Reader, _ io.Writer, _ []string) (bool, error) {
			return true, nil
		},
	}

	err := withWorkingDir(dir, func() error {
		return runner.Run(context.Background(), Options{
			Args: []string{"ok.mp3", "big.mp4"},
		})
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "ok.mp3") || !strings.Contains(output, "big.mp4") {
		t.Fatalf("expected file names in output, got %q", output)
	}
	if !strings.Contains(output, "OVER LIMIT") {
		t.Fatalf("expected over-limit status in output, got %q", output)
	}
	if !strings.Contains(output, "$0.100000") {
		t.Fatalf("expected billable file cost in output, got %q", output)
	}
	if !strings.Contains(output, "Estimated total cost") {
		t.Fatalf("expected total cost line in output, got %q", output)
	}
}

func TestSummarizeResultsExcludesOverLimit(t *testing.T) {
	t.Parallel()

	result := buildResult([]fileResult{
		{DurationSec: 3600, UploadBytes: 1024, CostUSD: 0.10},
		{OverLimit: true, DurationSec: 7200, UploadBytes: 501 * 1024 * 1024},
	})

	if result.Summary.BillableFiles != 1 {
		t.Fatalf("expected 1 billable file, got %d", result.Summary.BillableFiles)
	}
	if result.Summary.OverLimitFiles != 1 {
		t.Fatalf("expected 1 over-limit file, got %d", result.Summary.OverLimitFiles)
	}
	if result.Summary.EstimatedTotalCostUSD != 0.10 {
		t.Fatalf("expected total cost 0.10, got %f", result.Summary.EstimatedTotalCostUSD)
	}
}

func withWorkingDir(dir string, fn func() error) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	defer os.Chdir(cwd)
	return fn()
}
