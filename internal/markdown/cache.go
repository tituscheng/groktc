package markdown

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tituscheng/groktc/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type PromptCache interface {
	SavePrompt(ctx context.Context, name string, content string) error
	ListPrompts(ctx context.Context) ([]SavedPrompt, error)
	GetPromptByName(ctx context.Context, name string) (SavedPrompt, bool, error)
	UpsertPrompt(ctx context.Context, name string, content string) error
	DeletePrompt(ctx context.Context, name string) error
	Close() error
}

type SavedPrompt struct {
	ID          uint      `gorm:"primaryKey"`
	Name        string    `gorm:"uniqueIndex;size:255;not null"`
	Content     string    `gorm:"type:text;not null"`
	Description string    `gorm:"type:text"`
	Tags        string    `gorm:"type:json;not null;default:'[]'"`
	UsageCount  int       `gorm:"not null;default:0"`
	LastUsedAt  time.Time `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SavedPrompt) TableName() string {
	return "saved_prompts"
}

type SQLitePromptCache struct {
	db *gorm.DB
}

func OpenDefaultPromptCache(homeResolver func() (string, error)) (*SQLitePromptCache, error) {
	homeDir, err := homeResolver()
	if err != nil {
		slog.Error("resolve home directory failed", "error", err)
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return OpenPromptCache(filepath.Join(homeDir, config.DirectoryName))
}

func OpenPromptCache(configDir string) (*SQLitePromptCache, error) {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		slog.Error("create config directory failed", "config_dir", configDir, "error", err)
		return nil, fmt.Errorf("create config directory %q: %w", configDir, err)
	}

	dbPath := filepath.Join(configDir, config.DatabaseName)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		slog.Error("open sqlite database failed", "db_path", dbPath, "error", err)
		return nil, fmt.Errorf("open sqlite database %q: %w", dbPath, err)
	}
	if err := db.AutoMigrate(&SavedPrompt{}); err != nil {
		slog.Error("migrate saved prompts schema failed", "db_path", dbPath, "error", err)
		return nil, fmt.Errorf("migrate saved prompts schema: %w", err)
	}

	return &SQLitePromptCache{db: db}, nil
}

func (c *SQLitePromptCache) SavePrompt(ctx context.Context, name string, content string) error {
	record, err := newSavedPromptRecord(name, content)
	if err != nil {
		return err
	}

	err = c.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoNothing: true,
		}).
		Create(&record).
		Error
	if err != nil {
		slog.Error("save prompt failed", "name", name, "error", err)
		return fmt.Errorf("save prompt %q: %w", name, err)
	}
	if record.ID == 0 {
		slog.Error("prompt already exists", "name", name)
		return fmt.Errorf("prompt %q already exists", name)
	}

	return nil
}

func (c *SQLitePromptCache) ListPrompts(ctx context.Context) ([]SavedPrompt, error) {
	var prompts []SavedPrompt
	if err := c.db.WithContext(ctx).
		Order("last_used_at DESC").
		Order("name ASC").
		Find(&prompts).Error; err != nil {
		slog.Error("list saved prompts failed", "error", err)
		return nil, fmt.Errorf("list saved prompts: %w", err)
	}
	return prompts, nil
}

func (c *SQLitePromptCache) GetPromptByName(ctx context.Context, name string) (SavedPrompt, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		slog.Error("prompt name is required")
		return SavedPrompt{}, false, fmt.Errorf("prompt name is required")
	}

	var prompt SavedPrompt
	err := c.db.WithContext(ctx).Where("name = ?", name).Take(&prompt).Error
	if err == nil {
		return prompt, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return SavedPrompt{}, false, nil
	}
	slog.Error("get prompt by name failed", "name", name, "error", err)
	return SavedPrompt{}, false, fmt.Errorf("get prompt %q: %w", name, err)
}

func (c *SQLitePromptCache) UpsertPrompt(ctx context.Context, name string, content string) error {
	record, err := newSavedPromptRecord(name, content)
	if err != nil {
		return err
	}

	err = c.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "name"}},
			DoUpdates: clause.Assignments(map[string]any{
				"content":      record.Content,
				"last_used_at": record.LastUsedAt,
				"updated_at":   record.UpdatedAt,
				"usage_count":  gorm.Expr("usage_count + 1"),
			}),
		}).
		Create(&record).
		Error
	if err != nil {
		slog.Error("upsert prompt failed", "name", record.Name, "error", err)
		return fmt.Errorf("upsert prompt %q: %w", record.Name, err)
	}
	return nil
}

func (c *SQLitePromptCache) DeletePrompt(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		slog.Error("prompt name is required")
		return fmt.Errorf("prompt name is required")
	}

	result := c.db.WithContext(ctx).Delete(&SavedPrompt{}, "name = ?", name)
	if result.Error != nil {
		slog.Error("delete prompt failed", "name", name, "error", result.Error)
		return fmt.Errorf("delete prompt %q: %w", name, result.Error)
	}
	if result.RowsAffected == 0 {
		slog.Error("prompt not found", "name", name)
		return fmt.Errorf("prompt %q not found", name)
	}
	return nil
}

func (c *SQLitePromptCache) Close() error {
	sqlDB, err := c.db.DB()
	if err != nil {
		slog.Error("access sql db handle failed", "error", err)
		return fmt.Errorf("access sql db handle: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		slog.Error("close sql db handle failed", "error", err)
		return fmt.Errorf("close sql db handle: %w", err)
	}
	return nil
}

func newSavedPromptRecord(name string, content string) (SavedPrompt, error) {
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	if name == "" {
		return SavedPrompt{}, fmt.Errorf("prompt name is required")
	}
	if len(name) > 255 {
		return SavedPrompt{}, fmt.Errorf("prompt name must be 255 characters or fewer")
	}
	if content == "" {
		return SavedPrompt{}, fmt.Errorf("prompt content is required")
	}

	tags, err := json.Marshal([]string{})
	if err != nil {
		return SavedPrompt{}, fmt.Errorf("marshal default prompt tags: %w", err)
	}

	now := time.Now().UTC()
	return SavedPrompt{
		Name:       name,
		Content:    content,
		Tags:       string(tags),
		UsageCount: 0,
		LastUsedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}
