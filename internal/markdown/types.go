package markdown

import (
	"time"

	"github.com/tituscheng/groktc/pkg/runner"
)

const (
	DefaultModel           = "grok-4.3"
	DefaultReasoningEffort = "medium"
	DefaultMaxTokens       = 8192
	DefaultTemperature     = 0.2
	DefaultConcurrency     = 4
	XAIBaseURL             = "https://api.x.ai/v1"
	XAIAPIKeyEnv           = "XAI_API_KEY"
)

const SystemPrompt = "You are an expert technical transcriber. Convert the provided raw transcript into clean, professional Markdown. Rules: - Use proper headings (#, ##, ###) - Bullet points and numbered lists where appropriate - Preserve speaker names if present (e.g. **Speaker:**) - Add timestamps only if explicitly requested - Output ONLY the Markdown — no explanations, no code fences, no backticks."

type Options struct {
	Args       []string
	Prompt     string
	PromptFile string
	NoGUI      bool
	Force      bool
	Model      string
	Effort     string
	JSON       bool
}

type CacheMode string

const (
	CacheModeNone   CacheMode = "none"
	CacheModeCreate CacheMode = "create"
	CacheModeUpdate CacheMode = "update"
)

type GUIPromptResult struct {
	Text      string
	Cache     bool
	CacheName string
	CacheMode CacheMode
}

type FileTask = runner.Task

type ProcessResult = runner.Result

type Summary = runner.Summary

type MarkdownJSONResult struct {
	InputPath  string `json:"input"`
	OutputPath string `json:"output"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type MarkdownJSONOutput struct {
	Results []MarkdownJSONResult `json:"results"`
	Summary struct {
		Processed int       `json:"processed"`
		Skipped   int       `json:"skipped"`
		Failed    int       `json:"failed"`
		Total     int       `json:"total"`
		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"ended_at"`
	} `json:"summary"`
}
