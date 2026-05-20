package groktc

import (
	"github.com/tituscheng/groktc/internal/markdown"
)

// MarkdownOption customizes the behaviour of [Client.ConvertToMarkdown].
type MarkdownOption func(*markdown.Processor)

// WithModel sets the xAI model used for Markdown conversion.
// The default is "grok-4.3".
func WithModel(model string) MarkdownOption {
	return func(p *markdown.Processor) {
		p.Model = model
	}
}

// WithEffort sets the reasoning effort sent to xAI.
// Valid values are "low", "medium", or "high". The default is "medium".
func WithEffort(effort string) MarkdownOption {
	return func(p *markdown.Processor) {
		p.Effort = effort
	}
}

// WithMaxTokens sets the maximum completion tokens.
// The default is 8192.
func WithMaxTokens(n int) MarkdownOption {
	return func(p *markdown.Processor) {
		p.MaxTokens = n
	}
}

// WithTemperature sets the sampling temperature.
// The default is 0.2.
func WithTemperature(t float32) MarkdownOption {
	return func(p *markdown.Processor) {
		p.Temperature = t
	}
}
