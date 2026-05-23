package estimate

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/tituscheng/groktc/internal/transcribe"

	"golang.org/x/sync/errgroup"
)

const defaultConcurrency = 4

type fileResult struct {
	Path            string
	Kind            transcribe.InputKind
	DurationSec     float64
	FileSize        int64
	UploadBytes     int64
	UploadEstimated bool
	OverLimit       bool
	CostUSD         float64
}

type summaryTotals struct {
	billableFiles    int
	overLimitFiles   int
	totalDuration    float64
	totalUploadBytes int64
	totalCost        float64
	ratePerHour      float64
}

// FileEstimate holds STT cost metadata for a single media file.
type FileEstimate struct {
	Path            string
	Kind            string
	DurationSeconds float64
	FileSizeBytes   int64
	UploadBytes     int64
	UploadEstimated bool
	OverLimit       bool
	CostUSD         float64
	Billable        bool
}

// Summary aggregates STT cost estimates across a batch of files.
type Summary struct {
	Files                 int
	BillableFiles         int
	OverLimitFiles        int
	TotalDurationSeconds  float64
	TotalUploadBytes      int64
	RatePerHourUSD        float64
	EstimatedTotalCostUSD float64
}

// Result is the output of a batch STT cost estimation run.
type Result struct {
	Files   []FileEstimate
	Summary Summary
}

// Estimator probes local media files and calculates STT cost estimates.
type Estimator struct {
	ProberFactory func() transcribe.MediaProber
}

// NewEstimator returns an Estimator that uses ffprobe for local metadata probing.
func NewEstimator() *Estimator {
	return &Estimator{
		ProberFactory: func() transcribe.MediaProber {
			return transcribe.NewExecMediaProber()
		},
	}
}

// Estimate probes the given media file paths and returns per-file and total STT
// cost estimates. Paths must refer to regular .mp3 or .mp4 files.
func (e *Estimator) Estimate(ctx context.Context, paths []string) (Result, error) {
	prober := e.ProberFactory()
	if err := prober.CheckInstalled(); err != nil {
		return Result{}, err
	}

	files, err := ResolveFiles(paths)
	if err != nil {
		return Result{}, err
	}
	if len(files) == 0 {
		return Result{}, fmt.Errorf("at least one media file path is required")
	}

	estimates, err := e.estimateFiles(ctx, prober, files)
	if err != nil {
		return Result{}, err
	}

	return buildResult(estimates), nil
}

func summarizeResults(results []fileResult) summaryTotals {
	totals := summaryTotals{ratePerHour: transcribe.STTCostPerHourRESTUSD}
	for _, result := range results {
		if result.OverLimit {
			totals.overLimitFiles++
			continue
		}
		totals.billableFiles++
		totals.totalDuration += result.DurationSec
		totals.totalUploadBytes += result.UploadBytes
		totals.totalCost += result.CostUSD
	}
	return totals
}

func (e *Estimator) estimateFiles(ctx context.Context, prober transcribe.MediaProber, files []MediaFile) ([]fileResult, error) {
	results := make([]fileResult, 0, len(files))
	var (
		mu sync.Mutex
		g  errgroup.Group
	)
	g.SetLimit(defaultConcurrency)

	for _, file := range files {
		file := file
		g.Go(func() error {
			info, err := os.Stat(file.Path)
			if err != nil {
				return fmt.Errorf("stat %q: %w", file.Path, err)
			}

			probe, err := prober.ProbeMedia(ctx, file.Path)
			if err != nil {
				return fmt.Errorf("probe %q: %w", file.Path, err)
			}

			sizeCheck := transcribe.CheckSTTUploadSize(file.Kind, info.Size(), probe.DurationSeconds, probe.AudioBitrate)
			costUSD := 0.0
			if !sizeCheck.OverLimit {
				costUSD = transcribe.CalculateSTTCost(probe.DurationSeconds)
			}

			result := fileResult{
				Path:            file.Path,
				Kind:            file.Kind,
				DurationSec:     probe.DurationSeconds,
				FileSize:        info.Size(),
				UploadBytes:     sizeCheck.UploadBytes,
				UploadEstimated: sizeCheck.Estimated || file.Kind == transcribe.KindMP4,
				OverLimit:       sizeCheck.OverLimit,
				CostUSD:         costUSD,
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	return results, nil
}

func buildResult(results []fileResult) Result {
	summary := summarizeResults(results)
	files := make([]FileEstimate, 0, len(results))
	for _, result := range results {
		files = append(files, FileEstimate{
			Path:            result.Path,
			Kind:            kindName(result.Kind),
			DurationSeconds: result.DurationSec,
			FileSizeBytes:   result.FileSize,
			UploadBytes:     result.UploadBytes,
			UploadEstimated: result.UploadEstimated,
			OverLimit:       result.OverLimit,
			CostUSD:         result.CostUSD,
			Billable:        !result.OverLimit,
		})
	}

	return Result{
		Files: files,
		Summary: Summary{
			Files:                 len(results),
			BillableFiles:         summary.billableFiles,
			OverLimitFiles:        summary.overLimitFiles,
			TotalDurationSeconds:  summary.totalDuration,
			TotalUploadBytes:      summary.totalUploadBytes,
			RatePerHourUSD:        summary.ratePerHour,
			EstimatedTotalCostUSD: summary.totalCost,
		},
	}
}

func toJSONOutput(result Result) JSONOutput {
	files := make([]JSONFileResult, len(result.Files))
	for i, file := range result.Files {
		files[i] = JSONFileResult{
			Path:            file.Path,
			Kind:            file.Kind,
			DurationSeconds: file.DurationSeconds,
			FileSizeBytes:   file.FileSizeBytes,
			UploadBytes:     file.UploadBytes,
			UploadEstimated: file.UploadEstimated,
			OverLimit:       file.OverLimit,
			CostUSD:         file.CostUSD,
			Billable:        file.Billable,
		}
	}

	return JSONOutput{
		Files: files,
		Summary: JSONSummaryResult{
			Files:                 result.Summary.Files,
			BillableFiles:         result.Summary.BillableFiles,
			OverLimitFiles:        result.Summary.OverLimitFiles,
			TotalDurationSeconds:  result.Summary.TotalDurationSeconds,
			TotalUploadBytes:      result.Summary.TotalUploadBytes,
			RatePerHourUSD:        result.Summary.RatePerHourUSD,
			EstimatedTotalCostUSD: result.Summary.EstimatedTotalCostUSD,
		},
	}
}
