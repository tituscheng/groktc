package model

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tituscheng/groktc/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type Store interface {
	SetSelectedModel(ctx context.Context, modelID string) error
	GetSelectedModel(ctx context.Context) (string, error)
	UpsertModelCache(ctx context.Context, models []CatalogModel) error
	ListModelCache(ctx context.Context) ([]CatalogModel, error)
	GetModelByID(ctx context.Context, modelID string) (CatalogModel, bool, error)
	Close() error
}

type HomeDirResolver func() (string, error)

type SQLiteStore struct {
	db *gorm.DB
}

type settingRecord struct {
	Key       string    `gorm:"primaryKey;size:191"`
	Value     string    `gorm:"not null;default:''"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (settingRecord) TableName() string { return "settings" }

type modelCacheRecord struct {
	ModelID                     string    `gorm:"primaryKey;size:191"`
	AliasesJSON                 string    `gorm:"not null;default:'[]'"`
	InputModalitiesJSON         string    `gorm:"not null;default:'[]'"`
	OutputModalitiesJSON        string    `gorm:"not null;default:'[]'"`
	Version                     string    `gorm:"not null;default:''"`
	PromptTextTokenPriceRaw     string    `gorm:"not null;default:''"`
	CompletionTextTokenPriceRaw string    `gorm:"not null;default:''"`
	BestFor                     string    `gorm:"not null;default:''"`
	RetirementStatus            string    `gorm:"not null;default:''"`
	ReplacementModel            string    `gorm:"not null;default:''"`
	LastFetchedAt               time.Time `gorm:"not null"`
	RawJSON                     string    `gorm:"not null;default:''"`
	UpdatedAt                   time.Time `gorm:"not null"`
}

func (modelCacheRecord) TableName() string { return "model_cache" }

func OpenDefaultStore(homeResolver HomeDirResolver) (*SQLiteStore, error) {
	homeDir, err := homeResolver()
	if err != nil {
		slog.Error("resolve home directory failed", "error", err)
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return OpenStore(filepath.Join(homeDir, config.DirectoryName))
}

func OpenStore(configDir string) (*SQLiteStore, error) {
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

	if err := db.AutoMigrate(&settingRecord{}, &modelCacheRecord{}); err != nil {
		slog.Error("migrate sqlite schema failed", "db_path", dbPath, "error", err)
		return nil, fmt.Errorf("migrate sqlite schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	sqlDB, err := s.db.DB()
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

func (s *SQLiteStore) SetSelectedModel(ctx context.Context, modelID string) error {
	record := settingRecord{
		Key:       SelectedModelSettingKey,
		Value:     modelID,
		UpdatedAt: time.Now().UTC(),
	}

	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).
		Create(&record).
		Error
}

func (s *SQLiteStore) GetSelectedModel(ctx context.Context) (string, error) {
	var record settingRecord
	err := s.db.WithContext(ctx).First(&record, "key = ?", SelectedModelSettingKey).Error
	if err == nil {
		return record.Value, nil
	}
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	slog.Error("query selected model setting failed", "error", err)
	return "", fmt.Errorf("query selected model setting: %w", err)
}

func (s *SQLiteStore) UpsertModelCache(ctx context.Context, models []CatalogModel) error {
	now := time.Now().UTC()
	rows := make([]modelCacheRecord, 0, len(models))
	for _, item := range models {
		aliasesJSON, err := marshalStringSlice(item.Aliases)
		if err != nil {
			slog.Error("marshal aliases failed", "model_id", item.ID, "error", err)
			return err
		}
		inputJSON, err := marshalStringSlice(item.InputModalities)
		if err != nil {
			slog.Error("marshal input modalities failed", "model_id", item.ID, "error", err)
			return err
		}
		outputJSON, err := marshalStringSlice(item.OutputModalities)
		if err != nil {
			slog.Error("marshal output modalities failed", "model_id", item.ID, "error", err)
			return err
		}

		lastFetchedAt := item.LastFetchedAt
		if lastFetchedAt.IsZero() {
			lastFetchedAt = now
		}

		rows = append(rows, modelCacheRecord{
			ModelID:                     item.ID,
			AliasesJSON:                 aliasesJSON,
			InputModalitiesJSON:         inputJSON,
			OutputModalitiesJSON:        outputJSON,
			Version:                     item.Version,
			PromptTextTokenPriceRaw:     item.PromptTextTokenPriceRaw,
			CompletionTextTokenPriceRaw: item.CompletionTextTokenPriceRaw,
			BestFor:                     item.BestFor,
			RetirementStatus:            string(item.RetirementStatus),
			ReplacementModel:            item.ReplacementModel,
			LastFetchedAt:               lastFetchedAt,
			RawJSON:                     item.RawJSON,
			UpdatedAt:                   now,
		})
	}

	if len(rows) == 0 {
		return nil
	}

	if err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "model_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"aliases_json",
				"input_modalities_json",
				"output_modalities_json",
				"version",
				"prompt_text_token_price_raw",
				"completion_text_token_price_raw",
				"best_for",
				"retirement_status",
				"replacement_model",
				"last_fetched_at",
				"raw_json",
				"updated_at",
			}),
		}).
		Create(&rows).
		Error; err != nil {
		slog.Error("upsert model cache failed", "error", err)
		return err
	}
	return nil
}

func (s *SQLiteStore) ListModelCache(ctx context.Context) ([]CatalogModel, error) {
	var rows []modelCacheRecord
	if err := s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		slog.Error("query model cache failed", "error", err)
		return nil, fmt.Errorf("query model cache: %w", err)
	}

	models := make([]CatalogModel, 0, len(rows))
	for _, row := range rows {
		model, err := rowToCatalogModel(row)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	return models, nil
}

func (s *SQLiteStore) GetModelByID(ctx context.Context, modelID string) (CatalogModel, bool, error) {
	var row modelCacheRecord
	err := s.db.WithContext(ctx).First(&row, "model_id = ?", modelID).Error
	if err == gorm.ErrRecordNotFound {
		return CatalogModel{}, false, nil
	}
	if err != nil {
		slog.Error("query model from cache failed", "model_id", modelID, "error", err)
		return CatalogModel{}, false, fmt.Errorf("query model %q from cache: %w", modelID, err)
	}

	model, err := rowToCatalogModel(row)
	if err != nil {
		return CatalogModel{}, false, err
	}
	return model, true, nil
}

func rowToCatalogModel(row modelCacheRecord) (CatalogModel, error) {
	aliases, err := unmarshalStringSlice(row.AliasesJSON)
	if err != nil {
		return CatalogModel{}, err
	}
	input, err := unmarshalStringSlice(row.InputModalitiesJSON)
	if err != nil {
		return CatalogModel{}, err
	}
	output, err := unmarshalStringSlice(row.OutputModalitiesJSON)
	if err != nil {
		return CatalogModel{}, err
	}

	return CatalogModel{
		ID:                          row.ModelID,
		Aliases:                     aliases,
		InputModalities:             input,
		OutputModalities:            output,
		Version:                     row.Version,
		PromptTextTokenPriceRaw:     row.PromptTextTokenPriceRaw,
		CompletionTextTokenPriceRaw: row.CompletionTextTokenPriceRaw,
		BestFor:                     row.BestFor,
		RetirementStatus:            RetirementStatus(row.RetirementStatus),
		ReplacementModel:            row.ReplacementModel,
		LastFetchedAt:               row.LastFetchedAt,
		RawJSON:                     row.RawJSON,
	}, nil
}

func marshalStringSlice(values []string) (string, error) {
	if len(values) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal string slice: %w", err)
	}
	return string(data), nil
}

func unmarshalStringSlice(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("unmarshal string slice: %w", err)
	}
	return values, nil
}
