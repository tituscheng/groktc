package transcribe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateSTTCost(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0.0, CalculateSTTCost(0))
	require.Equal(t, 0.0, CalculateSTTCost(-10))
	require.InDelta(t, 0.10, CalculateSTTCost(3600), 0.000001)
	require.InDelta(t, 0.05, CalculateSTTCost(1800), 0.000001)
}
