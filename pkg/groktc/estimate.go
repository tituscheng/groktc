package groktc

import (
	"context"

	"github.com/tituscheng/groktc/internal/estimate"
)

// EstimateSTTCost probes local .mp3 and .mp4 files with ffprobe and returns
// per-file and total xAI REST STT cost estimates. No API key is required.
func EstimateSTTCost(ctx context.Context, paths ...string) (*STTEstimateResult, error) {
	result, err := estimate.NewEstimator().Estimate(ctx, paths)
	if err != nil {
		return nil, err
	}
	return toSTTEstimateResult(result), nil
}

func toSTTEstimateResult(result estimate.Result) *STTEstimateResult {
	files := make([]STTFileEstimate, len(result.Files))
	for i, file := range result.Files {
		files[i] = STTFileEstimate{
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

	return &STTEstimateResult{
		Files: files,
		Summary: STTEstimateSummary{
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
