package markdown

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tituscheng/groktc/internal/config"

	"github.com/stretchr/testify/require"
)

func TestPromptCacheSavePrompt(t *testing.T) {
	cache, err := OpenPromptCache(filepath.Join(t.TempDir(), config.DirectoryName))
	require.NoError(t, err)
	defer cache.Close()

	err = cache.SavePrompt(context.Background(), "Clean Transcript", "Use headings")
	require.NoError(t, err)

	var prompt SavedPrompt
	require.NoError(t, cache.db.First(&prompt, "name = ?", "Clean Transcript").Error)
	require.Equal(t, "Use headings", prompt.Content)
	require.Equal(t, "[]", prompt.Tags)
	require.Zero(t, prompt.UsageCount)
	require.False(t, prompt.LastUsedAt.IsZero())
}

func TestPromptCacheRejectsDuplicateName(t *testing.T) {
	cache, err := OpenPromptCache(filepath.Join(t.TempDir(), config.DirectoryName))
	require.NoError(t, err)
	defer cache.Close()

	require.NoError(t, cache.SavePrompt(context.Background(), "Duplicate", "first"))
	require.Error(t, cache.SavePrompt(context.Background(), "Duplicate", "second"))
}

func TestPromptCacheValidatesInput(t *testing.T) {
	cache, err := OpenPromptCache(filepath.Join(t.TempDir(), config.DirectoryName))
	require.NoError(t, err)
	defer cache.Close()

	require.Error(t, cache.SavePrompt(context.Background(), "", "content"))
	require.Error(t, cache.SavePrompt(context.Background(), "name", ""))
}

func TestPromptCacheListPromptsOrdersByLastUsedAtThenName(t *testing.T) {
	cache, err := OpenPromptCache(filepath.Join(t.TempDir(), config.DirectoryName))
	require.NoError(t, err)
	defer cache.Close()

	require.NoError(t, cache.SavePrompt(context.Background(), "Bravo", "second"))
	require.NoError(t, cache.SavePrompt(context.Background(), "Alpha", "first"))

	now := time.Now().UTC()
	require.NoError(t, cache.db.Model(&SavedPrompt{}).Where("name = ?", "Alpha").Update("last_used_at", now).Error)
	require.NoError(t, cache.db.Model(&SavedPrompt{}).Where("name = ?", "Bravo").Update("last_used_at", now.Add(-time.Hour)).Error)

	prompts, err := cache.ListPrompts(context.Background())
	require.NoError(t, err)
	require.Len(t, prompts, 2)
	require.Equal(t, "Alpha", prompts[0].Name)
	require.Equal(t, "Bravo", prompts[1].Name)
}

func TestPromptCacheGetPromptByName(t *testing.T) {
	cache, err := OpenPromptCache(filepath.Join(t.TempDir(), config.DirectoryName))
	require.NoError(t, err)
	defer cache.Close()

	require.NoError(t, cache.SavePrompt(context.Background(), "Reusable", "content"))

	prompt, found, err := cache.GetPromptByName(context.Background(), "Reusable")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "content", prompt.Content)

	_, found, err = cache.GetPromptByName(context.Background(), "Missing")
	require.NoError(t, err)
	require.False(t, found)
}

func TestPromptCacheUpsertPromptCreatesAndUpdates(t *testing.T) {
	cache, err := OpenPromptCache(filepath.Join(t.TempDir(), config.DirectoryName))
	require.NoError(t, err)
	defer cache.Close()

	require.NoError(t, cache.UpsertPrompt(context.Background(), "Reusable", "first"))

	created, found, err := cache.GetPromptByName(context.Background(), "Reusable")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, 0, created.UsageCount)

	require.NoError(t, cache.UpsertPrompt(context.Background(), "Reusable", "second"))

	updated, found, err := cache.GetPromptByName(context.Background(), "Reusable")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "second", updated.Content)
	require.Equal(t, 1, updated.UsageCount)
	require.True(t, updated.LastUsedAt.After(created.LastUsedAt) || updated.LastUsedAt.Equal(created.LastUsedAt))
}
