package groktc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSTTEstimateResultType(t *testing.T) {
	result := STTEstimateResult{
		Files: []STTFileEstimate{{
			Path:            "a.mp3",
			Kind:            "mp3",
			DurationSeconds: 60,
			CostUSD:         0.001667,
			Billable:        true,
		}},
		Summary: STTEstimateSummary{
			Files:                 1,
			BillableFiles:         1,
			EstimatedTotalCostUSD: 0.001667,
		},
	}
	require.Equal(t, "a.mp3", result.Files[0].Path)
	require.Equal(t, 1, result.Summary.BillableFiles)
}

func TestEstimateSTTCostRequiresPaths(t *testing.T) {
	t.Parallel()

	_, err := EstimateSTTCost(t.Context())
	require.Error(t, err)
}
