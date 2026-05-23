package transcribe

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tituscheng/groktc/pkg/runner"
	"github.com/tituscheng/groktc/internal/style"
)

type Runner struct {
	Stdout               io.Writer
	Stderr               io.Writer
	LookupEnv            func(string) string
	STTFactory           func(apiKey string) STTClient
	ConverterFactory     func() FFmpegConverter
	URLDownloaderFactory func() URLDownloader
}

func NewRunner() *Runner {
	return &Runner{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		LookupEnv: func(key string) string {
			return os.Getenv(key)
		},
		STTFactory: func(apiKey string) STTClient {
			return NewHTTPSTTClient(apiKey)
		},
		ConverterFactory: func() FFmpegConverter {
			return NewExecFFmpegConverter()
		},
		URLDownloaderFactory: func() URLDownloader {
			return NewYtgoDownloader()
		},
	}
}

func (r *Runner) Run(ctx context.Context, opts Options) (RunResult, error) {
	startedAt := time.Now().UTC()
	result := RunResult{
		Summary: runner.Summary{StartedAt: startedAt},
	}

	apiKey := strings.TrimSpace(r.LookupEnv(XAIAPIKeyEnv))
	if apiKey == "" {
		return result, fmt.Errorf(
			"%s is not set — set it in your shell profile (e.g. export %s=...) and re-run",
			XAIAPIKeyEnv, XAIAPIKeyEnv,
		)
	}

	tasks, err := ResolveTasks(opts.Args, opts.Force, opts.OutputPath)
	if err != nil {
		return result, err
	}
	if len(tasks) == 0 {
		return result, fmt.Errorf("no media files to process")
	}

	// Check ffmpeg if any MP4 tasks exist.
	if hasMp4Tasks(tasks) {
		if err := r.ConverterFactory().CheckInstalled(); err != nil {
			return result, err
		}
	}

	// Resolve titles and output paths for URL tasks.
	tasks, err = r.resolveURLTasks(ctx, tasks, opts.Force)
	if err != nil {
		return result, err
	}

	stt := r.STTFactory(apiKey)
	converter := r.ConverterFactory()
	downloader := r.URLDownloaderFactory()
	pipeline := newFilePipeline(stt, converter, downloader)

	// Route pipeline progress spinners to the runner's stdout.
	pipeline.SetStdout(r.Stdout)

	var mu sync.Mutex
	result.Tasks = make([]TranscriptionResult, 0, len(tasks))

	result.Summary = runner.ProcessTasks(ctx, tasks, func(t FileTask) runner.Task {
		return runner.Task{InputPath: t.InputPath, OutputPath: t.OutputPath, Skipped: t.Skipped, SkipReason: t.SkipReason}
	}, func(ctx context.Context, task FileTask) error {
		start := time.Now()
		duration, err := pipeline.Run(ctx, task)
		elapsed := time.Since(start)

		tr := TranscriptionResult{
			InputPath:     task.InputPath,
			OutputPath:    task.OutputPath,
			VTTOutputPath: vttPath(task.OutputPath),
			Duration:      duration,
			Elapsed:       elapsed.Seconds(),
			CostEstimate:  CalculateSTTCost(duration),
			Success:       err == nil,
		}
		if err != nil {
			tr.Error = err.Error()
		}

		mu.Lock()
		result.Tasks = append(result.Tasks, tr)
		mu.Unlock()

		return err
	}, runner.Options{
		Concurrency:  DefaultConcurrency,
		Stdout:       r.Stdout,
		Stderr:       r.Stderr,
		Style:        runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		SummaryTitle: "Transcribe Summary",
		PanicLabel:   "transcribe",
		TaskLabel:    "transcribe",
	})
	runner.RenderSummary(result.Summary, runner.Options{
		Stdout:       r.Stdout,
		Stderr:       r.Stderr,
		Style:        runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		SummaryTitle: "Transcribe Summary",
	})

	if result.Summary.FailedCount() > 0 {
		return result, fmt.Errorf("%d file(s) failed", result.Summary.FailedCount())
	}
	return result, nil
}

func (r *Runner) resolveURLTasks(ctx context.Context, tasks []FileTask, force bool) ([]FileTask, error) {
	if !hasURLTasks(tasks) {
		return tasks, nil
	}
	downloader := r.URLDownloaderFactory()
	if err := downloader.CheckInstalled(); err != nil {
		return nil, err
	}

	for i := range tasks {
		if tasks[i].Kind != KindURL || tasks[i].Skipped {
			continue
		}
		title, id, err := downloader.GetTitleAndID(ctx, tasks[i].InputPath)
		if err != nil {
			// Fall back to the placeholder slug derived from the URL.
			title = urlSlug(tasks[i].InputPath)
			id = ""
		}
		title = sanitizeFilename(title)
		if title == "" {
			title = urlSlug(tasks[i].InputPath)
		}
		var outName string
		if id != "" {
			outName = fmt.Sprintf("%s.%s.txt", title, id)
		} else {
			outName = title + ".txt"
		}
		tasks[i].OutputPath = filepath.Clean(outName)
		if shouldSkipOutputs(tasks[i].OutputPath, force) {
			tasks[i].Skipped = true
			tasks[i].SkipReason = "matching non-empty transcript and VTT files already exist"
		}
	}
	return tasks, nil
}

func hasMp4Tasks(tasks []FileTask) bool {
	for _, t := range tasks {
		if !t.Skipped && t.Kind == KindMP4 {
			return true
		}
	}
	return false
}

func hasURLTasks(tasks []FileTask) bool {
	for _, t := range tasks {
		if !t.Skipped && t.Kind == KindURL {
			return true
		}
	}
	return false
}
