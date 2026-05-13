// Package runner provides a generic concurrent task executor and summary renderer
// reused across groktc subcommands.
package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"golang.org/x/sync/errgroup"
)

// Task is the minimal information needed by the runner about a work item.
type Task struct {
	InputPath  string
	OutputPath string
	Skipped    bool
	SkipReason string
}

// Result records the outcome of a single task.
type Result struct {
	InputPath  string
	OutputPath string
	Skipped    bool
	Err        error
}

// Summary aggregates results from a batch run.
type Summary struct {
	Processed []Result
	Skipped   []Result
	Failed    []Result
	StartedAt time.Time
	EndedAt   time.Time
}

// FailedCount returns the number of failed tasks.
func (s Summary) FailedCount() int { return len(s.Failed) }

// Style holds terminal color/styling callbacks.
type Style struct {
	Header func(...interface{}) string
	Label  func(...interface{}) string
	OK     func(...interface{}) string
	Skip   func(...interface{}) string
	Fail   func(...interface{}) string
	Muted  func(...interface{}) string
	Value  func(...interface{}) string
	Total  func(...interface{}) string
}

// Options configures task execution and rendering.
type Options struct {
	Concurrency  int
	Stdout       io.Writer
	Stderr       io.Writer
	Style        Style
	SummaryTitle string
	PanicLabel   string
	TaskLabel    string
}

// ProcessTasks runs processor concurrently over tasks with panic recovery.
// Skipped tasks are added to summary.Skipped without calling processor.
func ProcessTasks[T any](ctx context.Context, tasks []T, extract func(T) Task, processor func(ctx context.Context, task T) error, opts Options) Summary {
	summary := Summary{StartedAt: time.Now().UTC()}
	var (
		mu sync.Mutex
		g  errgroup.Group
	)
	if opts.Concurrency > 0 {
		g.SetLimit(opts.Concurrency)
	}

	for _, task := range tasks {
		task := task
		info := extract(task)
		if info.Skipped {
			mu.Lock()
			summary.Skipped = append(summary.Skipped, Result{
				InputPath:  info.InputPath,
				OutputPath: info.OutputPath,
				Skipped:    true,
			})
			mu.Unlock()
			continue
		}

		g.Go(func() error {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("panic in task goroutine",
						"panic", rec,
						"stack", string(debug.Stack()),
						"label", opts.PanicLabel,
						"input_path", info.InputPath,
						"output_path", info.OutputPath,
					)
					mu.Lock()
					summary.Failed = append(summary.Failed, Result{
						InputPath:  info.InputPath,
						OutputPath: info.OutputPath,
						Err:        fmt.Errorf("panic: %v", rec),
					})
					mu.Unlock()
				}
			}()

			fmt.Fprintf(opts.Stdout, "%s %s %s\n", opts.Style.Header("[run]"), info.InputPath, opts.Style.Muted("-> "+info.OutputPath))
			err := processor(ctx, task)
			result := Result{
				InputPath:  info.InputPath,
				OutputPath: info.OutputPath,
				Err:        err,
			}

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				slog.Error("task failed",
					"label", opts.TaskLabel,
					"input_path", info.InputPath,
					"output_path", info.OutputPath,
					"error", err,
				)
				fmt.Fprintf(opts.Stderr, "%s Failed %s (%v)\n", opts.Style.Fail("[fail]"), info.InputPath, err)
				summary.Failed = append(summary.Failed, result)
				return nil
			}

			fmt.Fprintf(opts.Stdout, "%s %s\n", opts.Style.OK("[ok]"), info.OutputPath)
			summary.Processed = append(summary.Processed, result)
			return nil
		})
	}

	_ = g.Wait()
	summary.EndedAt = time.Now().UTC()
	return summary
}

// RenderSummary writes a colorized summary table to stdout.
func RenderSummary(summary Summary, opts Options) {
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintln(opts.Stdout, opts.Style.Header(opts.SummaryTitle))
	table := tabwriter.NewWriter(opts.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(table, "%s\t%s\n", opts.Style.Header("METRIC"), opts.Style.Header("VALUE"))
	fmt.Fprintf(table, "%s\t%s\n", opts.Style.Label("Processed"), opts.Style.OK(fmt.Sprintf("%d", len(summary.Processed))))
	fmt.Fprintf(table, "%s\t%s\n", opts.Style.Label("Skipped"), opts.Style.Skip(fmt.Sprintf("%d", len(summary.Skipped))))
	fmt.Fprintf(table, "%s\t%s\n", opts.Style.Label("Failed"), opts.Style.Fail(fmt.Sprintf("%d", len(summary.Failed))))
	fmt.Fprintf(table, "%s\t%s\n", opts.Style.Muted("------------------------------"), opts.Style.Muted("-----"))
	fmt.Fprintf(table, "%s\t%s\n", opts.Style.Label("Total"), opts.Style.Total(fmt.Sprintf("%d", len(summary.Processed)+len(summary.Skipped)+len(summary.Failed))))
	_ = table.Flush()

	if len(summary.Processed) > 0 {
		fmt.Fprintf(opts.Stdout, "%s %s\n", opts.Style.OK("wrote"), opts.Style.Value(strings.Join(outputPaths(summary.Processed), ", ")))
	}
	if len(summary.Skipped) > 0 {
		fmt.Fprintf(opts.Stdout, "%s %s\n", opts.Style.Skip("skipped"), opts.Style.Muted(strings.Join(outputPaths(summary.Skipped), ", ")))
	}
	if len(summary.Failed) > 0 {
		fmt.Fprintf(opts.Stdout, "%s %s\n", opts.Style.Fail("failed"), opts.Style.Muted(strings.Join(inputPaths(summary.Failed), ", ")))
	}
}

func outputPaths(results []Result) []string {
	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, r.OutputPath)
	}
	return paths
}

func inputPaths(results []Result) []string {
	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, r.InputPath)
	}
	return paths
}
