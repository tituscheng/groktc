package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/tituscheng/groktc/internal/markdown"
	"github.com/tituscheng/groktc/internal/prompt/webui"

	"github.com/spf13/cobra"
)

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Manage saved prompts",
	Long:  "Open an interactive webview to create, edit, and delete saved prompts.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPromptManager(cmd.Context())
	},
}

type promptStore struct {
	cache markdown.PromptCache
}

func (s *promptStore) SavePrompt(ctx context.Context, name, content string) error {
	return s.cache.SavePrompt(ctx, name, content)
}

func (s *promptStore) UpsertPrompt(ctx context.Context, name, content string) error {
	return s.cache.UpsertPrompt(ctx, name, content)
}

func (s *promptStore) DeletePrompt(ctx context.Context, name string) error {
	return s.cache.DeletePrompt(ctx, name)
}

func (s *promptStore) ListPrompts(ctx context.Context) ([]webui.SavedPromptOption, error) {
	prompts, err := s.cache.ListPrompts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]webui.SavedPromptOption, len(prompts))
	for i, p := range prompts {
		out[i] = webui.SavedPromptOption{Name: p.Name, Content: p.Content}
	}
	return out, nil
}

func (s *promptStore) Close() error {
	return s.cache.Close()
}

func runPromptManager(ctx context.Context) error {
	cache, err := markdown.OpenDefaultPromptCache(os.UserHomeDir)
	if err != nil {
		return fmt.Errorf("open prompt cache: %w", err)
	}
	defer cache.Close()

	saved, err := markdown.ListSavedPrompts(ctx, cache)
	if err != nil {
		return fmt.Errorf("list saved prompts: %w", err)
	}

	store := &promptStore{cache: cache}
	return webui.Manage(ctx, "Prompt Manager", "Create, edit, and delete saved prompts.", store, toWebuiSaved(saved))
}

func toWebuiSaved(prompts []markdown.SavedPrompt) []webui.SavedPromptOption {
	out := make([]webui.SavedPromptOption, len(prompts))
	for i, p := range prompts {
		out[i] = webui.SavedPromptOption{Name: p.Name, Content: p.Content}
	}
	return out
}

func init() {
	rootCmd.AddCommand(promptCmd)
}
