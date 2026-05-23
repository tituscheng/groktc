package estimate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/tituscheng/groktc/internal/confirm"
	"github.com/tituscheng/groktc/internal/transcribe"
)

const estimateWarningNote = "Note: Durations and upload sizes come from ffprobe metadata; MP4 upload size is estimated from audio bitrate. Actual billed usage may differ. Over-limit files will fail groktc transcribe until re-encoded or split."

type Runner struct {
	Stdout          io.Writer
	Stderr          io.Writer
	Stdin           io.Reader
	Estimator       *Estimator
	TerminalChecker func() bool
	Prompt          func(io.Reader, io.Writer, []string) (bool, error)
}

type JSONOutput struct {
	Files   []JSONFileResult  `json:"files"`
	Summary JSONSummaryResult `json:"summary"`
}

type JSONFileResult struct {
	Path            string  `json:"path"`
	Kind            string  `json:"kind"`
	DurationSeconds float64 `json:"duration_seconds"`
	FileSizeBytes   int64   `json:"file_size_bytes"`
	UploadBytes     int64   `json:"upload_bytes"`
	UploadEstimated bool    `json:"upload_estimated"`
	OverLimit       bool    `json:"over_limit"`
	CostUSD         float64 `json:"cost_usd"`
	Billable        bool    `json:"billable"`
}

type JSONSummaryResult struct {
	Files                 int     `json:"files"`
	BillableFiles         int     `json:"billable_files"`
	OverLimitFiles        int     `json:"over_limit_files"`
	TotalDurationSeconds  float64 `json:"total_duration_seconds"`
	TotalUploadBytes      int64   `json:"total_upload_bytes"`
	RatePerHourUSD        float64 `json:"rate_per_hour_usd"`
	EstimatedTotalCostUSD float64 `json:"estimated_total_cost_usd"`
}

func NewRunner() *Runner {
	return &Runner{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Stdin:     os.Stdin,
		Estimator: NewEstimator(),
		TerminalChecker: isTerminal,
		Prompt:          confirm.Prompt,
	}
}

func (r *Runner) Run(ctx context.Context, opts Options) error {
	files, err := r.resolveFiles(opts)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no media files were selected for estimation")
	}

	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}

	estimator := r.Estimator
	if estimator == nil {
		estimator = NewEstimator()
	}

	result, err := estimator.Estimate(ctx, paths)
	if err != nil {
		return err
	}

	if opts.JSON {
		return writeJSON(toJSONOutput(result))
	}

	r.renderReport(result)
	return nil
}

func (r *Runner) resolveFiles(opts Options) ([]MediaFile, error) {
	if len(opts.Args) > 0 {
		return ResolveFiles(opts.Args)
	}

	files, err := ResolveFiles(nil)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no media files were found in %s", filepath.Clean("."))
	}

	if opts.Yes {
		return files, nil
	}

	if !r.TerminalChecker() {
		fmt.Fprintln(r.Stdout, "Discovered media files:")
		for _, file := range files {
			fmt.Fprintf(r.Stdout, "  - %s\n", file.Path)
		}
		return nil, fmt.Errorf("interactive confirmation requires a TTY; rerun with explicit file paths or --yes")
	}

	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}

	confirmed, err := r.Prompt(r.Stdin, r.Stdout, paths)
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return nil, fmt.Errorf("estimation canceled")
	}

	return files, nil
}

func writeJSON(output JSONOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func (r *Runner) renderReport(result Result) {
	summary := result.Summary

	tw := tabwriter.NewWriter(r.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(
		tw,
		"%s\t%s\t%s\t%s\t%s\n",
		colorHeader("FILE"),
		colorHeader("DURATION"),
		colorHeader("UPLOAD SIZE"),
		colorHeader("STATUS"),
		colorHeader("EST. COST"),
	)
	for _, file := range result.Files {
		status := colorOK("OK")
		cost := colorValue(formatUSD(file.CostUSD))
		if file.OverLimit {
			status = colorWarning("OVER LIMIT")
			cost = colorMuted("-")
		}

		fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			file.Path,
			formatDuration(file.DurationSeconds),
			formatUploadSize(file.UploadBytes, file.UploadEstimated),
			status,
			cost,
		)
	}
	_ = tw.Flush()

	fmt.Fprintln(r.Stdout)
	summaryTable := tabwriter.NewWriter(r.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(summaryTable, "%s\t%s\n", colorHeader("METRIC"), colorHeader("VALUE"))
	fmt.Fprintf(summaryTable, "%s\t%s\n", colorLabel("Files"), colorValue(fmt.Sprintf("%d", summary.Files)))
	fmt.Fprintf(summaryTable, "%s\t%s\n", colorLabel("Billable files"), colorValue(fmt.Sprintf("%d", summary.BillableFiles)))
	fmt.Fprintf(summaryTable, "%s\t%s\n", colorLabel("Over limit (>500 MB upload)"), colorWarning(fmt.Sprintf("%d", summary.OverLimitFiles)))
	fmt.Fprintf(summaryTable, "%s\t%s\n", colorLabel("Total duration (billable)"), colorValue(formatDuration(summary.TotalDurationSeconds)))
	fmt.Fprintf(summaryTable, "%s\t%s\n", colorLabel("Total upload size (billable)"), colorValue(formatBytes(summary.TotalUploadBytes)))
	fmt.Fprintf(summaryTable, "%s\t%s\n", colorLabel("Rate"), colorValue(fmt.Sprintf("$%.2f / hr (REST)", summary.RatePerHourUSD)))
	_ = summaryTable.Flush()

	fmt.Fprintln(r.Stdout, colorMuted("------------------------------------------------------------"))
	totalSummary := tabwriter.NewWriter(r.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(totalSummary, "%s\t%s\n", colorLabel("Estimated total cost"), colorTotal(formatUSD(summary.EstimatedTotalCostUSD)))
	_ = totalSummary.Flush()
	fmt.Fprintf(r.Stdout, "\n%s\n", colorWarning(estimateWarningNote))
}

func kindName(kind transcribe.InputKind) string {
	switch kind {
	case transcribe.KindMP3:
		return "mp3"
	case transcribe.KindMP4:
		return "mp4"
	default:
		return "unknown"
	}
}

func formatUSD(amount float64) string {
	return fmt.Sprintf("$%.6f", amount)
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return "0s"
	}

	total := int(seconds + 0.5)
	hours := total / 3600
	minutes := (total % 3600) / 60
	secs := total % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

func formatBytes(n int64) string {
	switch {
	case n >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func formatUploadSize(bytes int64, estimated bool) string {
	formatted := formatBytes(bytes)
	if estimated {
		return "~" + formatted
	}
	return formatted
}

func isTerminal() bool {
	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdoutInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (stdinInfo.Mode()&os.ModeCharDevice) != 0 && (stdoutInfo.Mode()&os.ModeCharDevice) != 0
}
