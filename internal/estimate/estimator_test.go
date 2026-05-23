package estimate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tituscheng/groktc/internal/transcribe"
	"github.com/stretchr/testify/require"
)

func TestEstimatorEstimate(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.mp3")
	overPath := filepath.Join(dir, "big.mp4")
	require.NoError(t, os.WriteFile(okPath, []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(overPath, []byte("x"), 0o644))

	estimator := &Estimator{
		ProberFactory: func() transcribe.MediaProber {
			return &fakeProber{
				durations: map[string]float64{
					filepath.Base(okPath):   3600,
					filepath.Base(overPath): 7200,
				},
				bitrates: map[string]int64{
					filepath.Base(overPath): 640000,
				},
			}
		},
	}

	result, err := estimator.Estimate(context.Background(), []string{okPath, overPath})
	require.NoError(t, err)
	require.Len(t, result.Files, 2)
	require.Equal(t, 1, result.Summary.BillableFiles)
	require.Equal(t, 1, result.Summary.OverLimitFiles)
	require.InDelta(t, 0.10, result.Summary.EstimatedTotalCostUSD, 0.000001)
}

func TestEstimatorEstimateRequiresPaths(t *testing.T) {
	t.Parallel()

	_, err := NewEstimator().Estimate(context.Background(), nil)
	require.Error(t, err)
}
