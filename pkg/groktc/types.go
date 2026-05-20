// Package groktc provides a high-level client for the groktc library,
// wrapping xAI API operations such as transcription, Markdown conversion,
// model catalog queries, and token counting.
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
// Words carries the raw per-word timings and VTT is a ready-to-write WebVTT
// document built from them; both come from the same transcription call.
type TranscriptionResult struct {
	Text     string
	Language string
	Duration float64
	Words    []Word
	VTT      string
}
