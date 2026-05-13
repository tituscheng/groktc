package markdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tituscheng/groktc/pkg/retry"
	"github.com/tituscheng/groktc/pkg/xai"

	openai "github.com/sashabaranov/go-openai"
)

type Processor struct {
	Client      xai.ChatClient
	Model       string
	Effort      string
	MaxTokens   int
	Temperature float32
	Sleep       func(context.Context, time.Duration) error
}

func NewProcessor(client xai.ChatClient, model string, effort string) *Processor {
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}
	if strings.TrimSpace(effort) == "" {
		effort = DefaultReasoningEffort
	}
	return &Processor{
		Client:      client,
		Model:       model,
		Effort:      effort,
		MaxTokens:   DefaultMaxTokens,
		Temperature: DefaultTemperature,
		Sleep:       retry.Sleep,
	}
}

func (p *Processor) ProcessFile(ctx context.Context, task FileTask, prompt string) error {
	txtContent, err := os.ReadFile(task.InputPath)
	if err != nil {
		return fmt.Errorf("read transcript %q: %w", task.InputPath, err)
	}

	markdown, err := p.GenerateMarkdown(ctx, prompt, string(txtContent))
	if err != nil {
		return err
	}
	if strings.TrimSpace(markdown) == "" {
		return fmt.Errorf("generated Markdown is empty")
	}

	if err := WriteFileAtomic(task.OutputPath, []byte(markdown)); err != nil {
		return fmt.Errorf("write Markdown %q: %w", task.OutputPath, err)
	}
	return nil
}

func (p *Processor) GenerateMarkdown(ctx context.Context, prompt string, txtContent string) (string, error) {
	if p.Client == nil {
		return "", fmt.Errorf("chat client is required")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}
	if strings.TrimSpace(txtContent) == "" {
		return "", fmt.Errorf("transcript content is required")
	}

	request := p.buildRequest(prompt, txtContent)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		response, err := p.Client.CreateChatCompletion(ctx, request)
		if err == nil {
			if len(response.Choices) == 0 {
				slog.Error("xAI returned no choices")
				return "", fmt.Errorf("xAI returned no choices")
			}
			content := strings.TrimSpace(response.Choices[0].Message.Content)
			if content == "" {
				slog.Error("xAI returned empty Markdown")
				return "", fmt.Errorf("xAI returned empty Markdown")
			}
			return content, nil
		}

		lastErr = err
		if !isRetryableError(err) || attempt == 3 {
			break
		}
		slog.Warn("chat completion attempt failed, retrying",
			"attempt", attempt,
			"max_attempts", 3,
			"error", err,
		)
		if sleepErr := p.sleep(ctx, retry.Backoff(250*time.Millisecond, attempt)); sleepErr != nil {
			return "", sleepErr
		}
	}

	slog.Error("generate Markdown with xAI failed after retries", "error", lastErr)
	return "", fmt.Errorf("generate Markdown with xAI: %w", lastErr)
}

func (p *Processor) buildRequest(prompt string, txtContent string) openai.ChatCompletionRequest {
	maxTokens := p.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	temperature := p.Temperature
	if math.IsNaN(float64(temperature)) {
		temperature = DefaultTemperature
	}

	return openai.ChatCompletionRequest{
		Model:               p.Model,
		MaxCompletionTokens: maxTokens,
		Temperature:         temperature,
		ReasoningEffort:     p.Effort,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: SystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
			{Role: openai.ChatMessageRoleUser, Content: "Here is the full transcript:\n\n" + txtContent},
		},
	}
}

func (p *Processor) sleep(ctx context.Context, duration time.Duration) error {
	if p.Sleep != nil {
		return p.Sleep(ctx, duration)
	}
	return retry.Sleep(ctx, duration)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode == 429 || apiErr.HTTPStatusCode >= 500
	}

	var requestErr *openai.RequestError
	if errors.As(err, &requestErr) {
		return requestErr.HTTPStatusCode == 429 || requestErr.HTTPStatusCode >= 500
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}
