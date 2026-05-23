// Package groktc provides a high-level client for the groktc library,
// wrapping xAI API operations such as transcription, Markdown conversion,
// model catalog queries, token counting, and local STT cost estimation.
package groktc

// Model represents an xAI language model available for use.
type Model struct {
	ID                          string
	Aliases                     []string
	InputModalities             []string
	OutputModalities            []string
	Version                     string
	PromptTextTokenPriceRaw     string
	CompletionTextTokenPriceRaw string
}

// Word is a single transcribed word with its timing. Start and End are
// offsets in seconds from the beginning of the audio.
type Word struct {
	Text  string
	Start float64
	End   float64
}

// TranscriptionResult holds the output of an xAI speech-to-text call.
// Words carries the raw per-word timings and VTT is a ready-to-write,
// spec-compliant WebVTT document built from them; both come from the same
// transcription call.
type TranscriptionResult struct {
	Text     string
	Language string
	Duration float64
	Words    []Word
	VTT      string
}

// STTFileEstimate holds STT cost metadata for a single media file.
type STTFileEstimate struct {
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

// STTEstimateSummary aggregates STT cost estimates across a batch of files.
type STTEstimateSummary struct {
	Files                 int
	BillableFiles         int
	OverLimitFiles        int
	TotalDurationSeconds  float64
	TotalUploadBytes      int64
	RatePerHourUSD        float64
	EstimatedTotalCostUSD float64
}

// STTEstimateResult is the output of a local STT cost estimation run.
type STTEstimateResult struct {
	Files   []STTFileEstimate
	Summary STTEstimateSummary
}
