package transcribe

import (
	"github.com/tituscheng/groktc/pkg/runner"
)

const (
	DefaultConcurrency  = 4
	MaxAudioBytes       = 500 * 1024 * 1024 // 500 MB
	STTEndpoint         = "https://api.x.ai/v1/stt"
	XAIAPIKeyEnv        = "XAI_API_KEY"
	XAIBaseURL          = "https://api.x.ai/v1"
	STTCostPerMinuteUSD = 0.0 // placeholder — update with actual xAI STT pricing
)

type InputKind int

const (
	KindMP3 InputKind = iota
	KindMP4
	KindURL
)

type Options struct {
	Args       []string
	Force      bool
	OutputPath string
	JSON       bool
}

type FileTask struct {
	InputPath  string
	OutputPath string
	Kind       InputKind
	Skipped    bool
	SkipReason string
}

type ProcessResult = runner.Result

type Summary = runner.Summary

type STTResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
}

// TranscriptionResult is the per-task metadata returned when --json is used.
type TranscriptionResult struct {
	InputPath    string        `json:"input"`
	OutputPath   string        `json:"output"`
	Duration     float64       `json:"duration_seconds"`
	Elapsed      float64       `json:"elapsed_seconds"`
	CostEstimate float64       `json:"cost_estimate_usd"`
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
}

// RunResult wraps the runner summary with per-task transcription metadata.
type RunResult struct {
	Summary runner.Summary
	Tasks   []TranscriptionResult
}

// calculateCost returns the estimated cost in USD for a given audio duration.
func calculateCost(durationSeconds float64) float64 {
	if durationSeconds <= 0 {
		return 0
	}
	return (durationSeconds / 60.0) * STTCostPerMinuteUSD
}
