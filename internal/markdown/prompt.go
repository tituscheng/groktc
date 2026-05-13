package markdown

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// ResolvePrompt resolves the user's prompt from the --prompt flag, --prompt-file,
// or an interactive PromptProvider. It validates and normalizes the result.
func ResolvePrompt(ctx context.Context, prompt, promptFile string, noGUI bool, provider PromptProvider) (string, GUIPromptResult, error) {
	if strings.TrimSpace(prompt) != "" {
		p := strings.TrimSpace(prompt)
		return p, GUIPromptResult{Text: p}, nil
	}
	if strings.TrimSpace(promptFile) != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return "", GUIPromptResult{}, fmt.Errorf("read prompt file %q: %w", promptFile, err)
		}
		p := strings.TrimSpace(string(data))
		if p == "" {
			return "", GUIPromptResult{}, fmt.Errorf("prompt file %q is empty", promptFile)
		}
		return p, GUIPromptResult{Text: p}, nil
	}
	if noGUI {
		return "", GUIPromptResult{}, fmt.Errorf("prompt is required when --no-gui is set")
	}
	if provider == nil {
		return "", GUIPromptResult{}, fmt.Errorf("prompt GUI is not configured")
	}

	result, err := provider(ctx)
	if err != nil {
		return "", GUIPromptResult{}, err
	}
	result.Text = strings.TrimSpace(result.Text)
	result.CacheName = strings.TrimSpace(result.CacheName)
	if result.CacheMode == "" {
		result.CacheMode = CacheModeNone
	}
	if result.CacheMode != CacheModeNone {
		result.Cache = true
	}
	if result.Text == "" {
		return "", GUIPromptResult{}, fmt.Errorf("prompt is required")
	}
	if result.CacheMode != CacheModeNone && result.CacheName == "" {
		return "", GUIPromptResult{}, fmt.Errorf("prompt name is required when saving to library")
	}
	return result.Text, result, nil
}

// PersistPrompt saves a prompt result to the cache according to its cache mode.
func PersistPrompt(ctx context.Context, cache PromptCache, result GUIPromptResult) error {
	if cache == nil {
		return fmt.Errorf("prompt cache is not configured")
	}

	switch result.CacheMode {
	case CacheModeNone:
		return nil
	case CacheModeCreate:
		return cache.SavePrompt(ctx, result.CacheName, result.Text)
	case CacheModeUpdate:
		return cache.UpsertPrompt(ctx, result.CacheName, result.Text)
	default:
		return fmt.Errorf("unsupported prompt cache mode %q", result.CacheMode)
	}
}

// ListSavedPrompts returns all saved prompts from the cache.
func ListSavedPrompts(ctx context.Context, cache PromptCache) ([]SavedPrompt, error) {
	if cache == nil {
		return nil, fmt.Errorf("prompt cache is not configured")
	}
	return cache.ListPrompts(ctx)
}
