package groktc

import (
	"context"
	"fmt"
	"strings"

	"github.com/tituscheng/groktc/internal/markdown"
	"github.com/tituscheng/groktc/internal/transcribe"
	"github.com/tituscheng/groktc/pkg/xai"
)

// Client is a high-level client for xAI operations.
// It wraps tokenization, model catalog, transcription, and Markdown generation.
// For local STT cost estimation without an API key, use [EstimateSTTCost].
type Client struct {
	apiKey  string
	xai     *xai.Client
	chat    xai.ChatClient
}

// NewClient creates a Client using the provided xAI API key.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		xai:    xai.NewClient(apiKey),
		chat:   xai.NewChatClient(apiKey),
	}
}

// CountTokens estimates the number of tokens in text for a given model
// using the xAI tokenizer API.
func (c *Client) CountTokens(ctx context.Context, model, text string) (int, error) {
	return c.xai.CountTokens(ctx, model, text)
}

// ListModels fetches the list of available xAI language models.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	models, err := c.xai.ListLanguageModels(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]Model, 0, len(models))
	for _, m := range models {
		result = append(result, Model{
			ID:                          m.ID,
			Aliases:                     m.Aliases,
			InputModalities:             m.InputModalities,
			OutputModalities:            m.OutputModalities,
			Version:                     m.Version,
			PromptTextTokenPriceRaw:     m.PromptTextTokenPriceRaw,
			CompletionTextTokenPriceRaw: m.CompletionTextTokenPriceRaw,
		})
	}
	return result, nil
}

// ConvertToMarkdown sends raw text and a cleanup prompt to xAI and returns
// polished Markdown. By default it uses the "grok-4.3" model with medium
// reasoning effort. Use [WithModel], [WithEffort], [WithMaxTokens], or
// [WithTemperature] to override defaults.
func (c *Client) ConvertToMarkdown(ctx context.Context, text, prompt string, opts ...MarkdownOption) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		prompt = markdown.SystemPrompt
	}

	p := markdown.NewProcessor(c.chat, "", "")
	for _, opt := range opts {
		opt(p)
	}

	return p.GenerateMarkdown(ctx, prompt, text)
}

// TranscribeAudio sends an audio file (MP3, WAV, etc.) to the xAI STT
// endpoint and returns the transcript, per-word timings, and a rendered
// WebVTT document — all from a single transcription call.
func (c *Client) TranscribeAudio(ctx context.Context, audioPath string) (*TranscriptionResult, error) {
	stt := transcribe.NewHTTPSTTClient(c.apiKey)
	resp, err := stt.Transcribe(ctx, audioPath)
	if err != nil {
		return nil, fmt.Errorf("transcribe %q: %w", audioPath, err)
	}
	words := make([]Word, len(resp.Words))
	for i, w := range resp.Words {
		words[i] = Word{Text: w.Text, Start: w.Start, End: w.End}
	}
	return &TranscriptionResult{
		Text:     resp.Text,
		Language: resp.Language,
		Duration: resp.Duration,
		Words:    words,
		VTT:      transcribe.BuildVTT(resp),
	}, nil
}
