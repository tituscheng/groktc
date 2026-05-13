package markdown

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
)

type fakeChatClient struct {
	requests []openai.ChatCompletionRequest
	results  []openai.ChatCompletionResponse
	errs     []error
}

func (f *fakeChatClient) CreateChatCompletion(_ context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	f.requests = append(f.requests, request)
	index := len(f.requests) - 1
	if index < len(f.errs) && f.errs[index] != nil {
		return openai.ChatCompletionResponse{}, f.errs[index]
	}
	if index < len(f.results) {
		return f.results[index], nil
	}
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "# Clean"}}},
	}, nil
}

func TestProcessorBuildsExpectedMessages(t *testing.T) {
	client := &fakeChatClient{}
	processor := NewProcessor(client, "grok-4.3", "medium")

	got, err := processor.GenerateMarkdown(context.Background(), "Use sections", "Speaker: hello")
	require.NoError(t, err)
	require.Equal(t, "# Clean", got)
	require.Len(t, client.requests, 1)

	request := client.requests[0]
	require.Equal(t, "grok-4.3", request.Model)
	require.Equal(t, "medium", request.ReasoningEffort)
	require.Equal(t, DefaultMaxTokens, request.MaxCompletionTokens)
	require.Len(t, request.Messages, 3)
	require.Equal(t, openai.ChatMessageRoleSystem, request.Messages[0].Role)
	require.Equal(t, SystemPrompt, request.Messages[0].Content)
	require.Equal(t, "Use sections", request.Messages[1].Content)
	require.Equal(t, "Here is the full transcript:\n\nSpeaker: hello", request.Messages[2].Content)
}

func TestProcessorRetriesTransientErrors(t *testing.T) {
	client := &fakeChatClient{
		errs: []error{
			&openai.APIError{HTTPStatusCode: 500, Message: "server"},
			nil,
		},
		results: []openai.ChatCompletionResponse{
			{},
			{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "# Retried"}}}},
		},
	}
	processor := NewProcessor(client, "grok-4.3", "medium")
	processor.Sleep = func(context.Context, time.Duration) error { return nil }

	got, err := processor.GenerateMarkdown(context.Background(), "Prompt", "Transcript")
	require.NoError(t, err)
	require.Equal(t, "# Retried", got)
	require.Len(t, client.requests, 2)
}

func TestProcessorDoesNotRetryValidationErrors(t *testing.T) {
	client := &fakeChatClient{errs: []error{errors.New("bad request")}}
	processor := NewProcessor(client, "grok-4.3", "medium")

	_, err := processor.GenerateMarkdown(context.Background(), "Prompt", "Transcript")
	require.Error(t, err)
	require.Len(t, client.requests, 1)
}

func TestProcessorEmptyResponseFails(t *testing.T) {
	client := &fakeChatClient{
		results: []openai.ChatCompletionResponse{
			{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "   "}}}},
		},
	}
	processor := NewProcessor(client, "grok-4.3", "medium")

	_, err := processor.GenerateMarkdown(context.Background(), "Prompt", "Transcript")
	require.Error(t, err)
}

func TestProcessorWritesMarkdownAtomically(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "raw.txt")
	output := filepath.Join(dir, "raw.md")
	require.NoError(t, os.WriteFile(input, []byte("raw transcript"), 0o644))

	client := &fakeChatClient{
		results: []openai.ChatCompletionResponse{
			{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "# Cleaned"}}}},
		},
	}
	processor := NewProcessor(client, "grok-4.3", "medium")

	err := processor.ProcessFile(context.Background(), FileTask{InputPath: input, OutputPath: output}, "Prompt")
	require.NoError(t, err)
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, "# Cleaned", string(data))
}
