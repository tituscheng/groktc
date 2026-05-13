package markdown

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tituscheng/groktc/pkg/runner"
	"github.com/tituscheng/groktc/internal/style"
	"github.com/tituscheng/groktc/pkg/xai"
)

type PromptProvider func(context.Context) (GUIPromptResult, error)

type ProcessorInterface interface {
	ProcessFile(ctx context.Context, task FileTask, prompt string) error
}

type Runner struct {
	Stdout           io.Writer
	Stderr           io.Writer
	LookupEnv        func(string) string
	PromptProvider   PromptProvider
	CacheFactory     func() (PromptCache, error)
	ProcessorFactory func(apiKey string, model string, effort string) ProcessorInterface
}

func NewRunner() *Runner {
	return &Runner{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		LookupEnv: func(key string) string {
			return os.Getenv(key)
		},
		PromptProvider: nil,
		CacheFactory: func() (PromptCache, error) {
			return OpenDefaultPromptCache(os.UserHomeDir)
		},
		ProcessorFactory: func(apiKey string, model string, effort string) ProcessorInterface {
			return NewProcessor(xai.NewChatClient(apiKey), model, effort)
		},
	}
}

func (r *Runner) Run(ctx context.Context, opts Options) (Summary, error) {
	startedAt := time.Now().UTC()
	summary := Summary{StartedAt: startedAt}

	prompt, guiResult, err := ResolvePrompt(ctx, opts.Prompt, opts.PromptFile, opts.NoGUI, r.PromptProvider)
	if err != nil {
		return summary, err
	}
	if guiResult.Cache {
		cache, err := r.CacheFactory()
		if err != nil {
			return summary, err
		}
		defer cache.Close()
		if err := PersistPrompt(ctx, cache, guiResult); err != nil {
			return summary, err
		}
	}

	apiKey := strings.TrimSpace(r.LookupEnv(XAIAPIKeyEnv))
	if apiKey == "" {
		return summary, fmt.Errorf("%s is required", XAIAPIKeyEnv)
	}

	tasks, err := ResolveTasks(opts.Args, opts.Force)
	if err != nil {
		return summary, err
	}
	if len(tasks) == 0 {
		return summary, fmt.Errorf("no transcript files to process")
	}

	processor := r.ProcessorFactory(apiKey, opts.Model, opts.Effort)
	summary = runner.ProcessTasks(ctx, tasks, func(t FileTask) runner.Task { return t }, func(ctx context.Context, task FileTask) error {
		return processor.ProcessFile(ctx, task, prompt)
	}, runner.Options{
		Concurrency:  DefaultConcurrency,
		Stdout:       r.Stdout,
		Stderr:       r.Stderr,
		Style:        runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		SummaryTitle: "Markdown Summary",
		PanicLabel:   "markdown",
		TaskLabel:    "markdown",
	})
	runner.RenderSummary(summary, runner.Options{
		Stdout:       r.Stdout,
		Stderr:       r.Stderr,
		Style:        runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		SummaryTitle: "Markdown Summary",
	})

	if summary.FailedCount() > 0 {
		return summary, fmt.Errorf("%d transcript file(s) failed", summary.FailedCount())
	}
	return summary, nil
}
