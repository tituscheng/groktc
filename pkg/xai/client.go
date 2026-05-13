package xai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const (
	DefaultBaseURL = "https://api.x.ai"
)

type TokenizerClient interface {
	CountTokens(ctx context.Context, model string, text string) (int, error)
}

type LanguageModelCatalogClient interface {
	ListLanguageModels(ctx context.Context) ([]LanguageModel, error)
}

// ChatClient is the subset of the OpenAI client used for chat completions.
type ChatClient interface {
	CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// NewChatClient creates an OpenAI-compatible client configured for the xAI API.
func NewChatClient(apiKey string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = DefaultBaseURL + "/v1"
	return openai.NewClientWithConfig(cfg)
}

type LanguageModel struct {
	ID                          string
	Aliases                     []string
	InputModalities             []string
	OutputModalities            []string
	Version                     string
	PromptTextTokenPriceRaw     string
	CompletionTextTokenPriceRaw string
	RawJSON                     string
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type tokenizeRequest struct {
	Model string `json:"model"`
	Text  string `json:"text"`
}

type tokenizeResponse struct {
	TokenIDs []json.RawMessage `json:"token_ids"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		APIKey:  apiKey,
	}
}

func (c *Client) CountTokens(ctx context.Context, model string, text string) (int, error) {
	if strings.TrimSpace(model) == "" {
		return 0, fmt.Errorf("model is required")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return 0, fmt.Errorf("xAI API key is required")
	}

	body, err := json.Marshal(tokenizeRequest{
		Model: model,
		Text:  text,
	})
	if err != nil {
		return 0, fmt.Errorf("marshal tokenize request: %w", err)
	}

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/tokenize-text", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create tokenize request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("call xAI tokenize API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := decodeAPIError(resp)
		slog.Error("xAI tokenize API error",
			"status_code", resp.StatusCode,
			"error", err,
		)
		return 0, err
	}

	var payload tokenizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, fmt.Errorf("decode tokenize response: %w", err)
	}
	if payload.TokenIDs == nil {
		return 0, fmt.Errorf("decode tokenize response: missing token_ids")
	}

	return len(payload.TokenIDs), nil
}

func (c *Client) ListLanguageModels(ctx context.Context) ([]LanguageModel, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("xAI API key is required")
	}

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/language-models", nil)
	if err != nil {
		return nil, fmt.Errorf("create list language models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call xAI language models API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := decodeAPIError(resp)
		slog.Error("xAI language models API error",
			"status_code", resp.StatusCode,
			"error", err,
		)
		return nil, err
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read language models response: %w", err)
	}

	modelMaps, err := extractModelItems(body)
	if err != nil {
		return nil, err
	}

	models := make([]LanguageModel, 0, len(modelMaps))
	for _, modelMap := range modelMaps {
		model, err := parseLanguageModel(modelMap)
		if err != nil {
			return nil, err
		}
		if model.ID == "" {
			continue
		}
		models = append(models, model)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	return models, nil
}

func extractModelItems(data []byte) ([]map[string]any, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("decode language models response: %w", err)
	}

	switch typed := root.(type) {
	case []any:
		return toModelMapSlice(typed), nil
	case map[string]any:
		for _, key := range []string{"data", "models", "language_models"} {
			if raw, ok := typed[key]; ok {
				if list, ok := raw.([]any); ok {
					return toModelMapSlice(list), nil
				}
			}
		}

		if _, hasID := typed["id"]; hasID {
			return []map[string]any{typed}, nil
		}
	}

	return nil, fmt.Errorf("decode language models response: unsupported payload shape")
}

func toModelMapSlice(input []any) []map[string]any {
	output := make([]map[string]any, 0, len(input))
	for _, item := range input {
		if mapped, ok := item.(map[string]any); ok {
			output = append(output, mapped)
		}
	}
	return output
}

func parseLanguageModel(modelMap map[string]any) (LanguageModel, error) {
	id := extractString(modelMap, "id", "name", "model", "model_id")
	if id == "" {
		return LanguageModel{}, fmt.Errorf("decode language model: missing id")
	}

	rawJSON, err := json.Marshal(modelMap)
	if err != nil {
		return LanguageModel{}, fmt.Errorf("marshal model %q raw JSON: %w", id, err)
	}

	pricingMap := extractNestedMap(modelMap, "pricing", "token_pricing")

	model := LanguageModel{
		ID:                          id,
		Aliases:                     dedupeAndSortStrings(extractStringSlice(modelMap, "aliases", "alias")),
		InputModalities:             dedupeAndSortStrings(extractStringSlice(modelMap, "input_modalities", "inputModalities")),
		OutputModalities:            dedupeAndSortStrings(extractStringSlice(modelMap, "output_modalities", "outputModalities")),
		Version:                     extractString(modelMap, "version"),
		PromptTextTokenPriceRaw:     coalesceString(extractString(modelMap, "prompt_text_token_price", "promptTextTokenPrice"), extractString(pricingMap, "prompt_text_token_price", "promptTextTokenPrice")),
		CompletionTextTokenPriceRaw: coalesceString(extractString(modelMap, "completion_text_token_price", "completionTextTokenPrice"), extractString(pricingMap, "completion_text_token_price", "completionTextTokenPrice")),
		RawJSON:                     string(rawJSON),
	}

	return model, nil
}

func extractNestedMap(source map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if nested, ok := source[key].(map[string]any); ok {
			return nested
		}
	}
	return map[string]any{}
}

func extractString(source map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := source[key]
		if !ok || value == nil {
			continue
		}

		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case json.Number:
			return typed.String()
		}
	}
	return ""
}

func extractStringSlice(source map[string]any, keys ...string) []string {
	for _, key := range keys {
		raw, ok := source[key]
		if !ok || raw == nil {
			continue
		}

		switch typed := raw.(type) {
		case []any:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				switch itemTyped := item.(type) {
				case string:
					if strings.TrimSpace(itemTyped) != "" {
						out = append(out, itemTyped)
					}
				}
			}
			return out
		case []string:
			return append([]string(nil), typed...)
		case string:
			if strings.TrimSpace(typed) != "" {
				return []string{typed}
			}
		}
	}
	return nil
}

func dedupeAndSortStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, value := range input {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func coalesceString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func decodeAPIError(resp *http.Response) error {
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if readErr != nil {
		return fmt.Errorf("xAI tokenize API returned status %d", resp.StatusCode)
	}

	var payload errorResponse
	if err := json.Unmarshal(data, &payload); err == nil && strings.TrimSpace(payload.Error.Message) != "" {
		return fmt.Errorf("xAI tokenize API returned status %d: %s", resp.StatusCode, payload.Error.Message)
	}

	message := strings.TrimSpace(string(data))
	if message == "" {
		return fmt.Errorf("xAI tokenize API returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("xAI tokenize API returned status %d: %s", resp.StatusCode, message)
}
