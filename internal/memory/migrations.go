package memory

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// CreateTables creates all necessary tables for the memory service
func CreateTables(ctx context.Context, db *bun.DB) error {
	models := []interface{}{
		(*UserSchema)(nil),
		(*SessionSchema)(nil),
		(*MessageSchema)(nil),
		(*FactSchema)(nil),
		(*MessageEmbeddingSchema)(nil),
		(*FactEmbeddingSchema)(nil),
	}

	for _, model := range models {
		_, err := db.NewCreateTable().
			Model(model).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create table for model %T: %w", model, err)
		}
	}

	return nil
}

// CreateIndexes creates all necessary indexes for the memory service
func CreateIndexes(ctx context.Context, db *bun.DB) error {
	allIndexes := append(SessionIndexes, UserIndexes...)
	allIndexes = append(allIndexes, MessageIndexes...)
	allIndexes = append(allIndexes, FactIndexes...)
	allIndexes = append(allIndexes, EmbeddingIndexes...)

	for _, indexSQL := range allIndexes {
		_, err := db.ExecContext(ctx, indexSQL)
		if err != nil {
			return fmt.Errorf("failed to create index with SQL %q: %w", indexSQL, err)
		}
	}

	return nil
}

// MigrateSchema performs any necessary schema migrations
func MigrateSchema(ctx context.Context, db *bun.DB) error {
	// Add rating column to facts table if it doesn't exist
	_, err := db.ExecContext(ctx, `
		ALTER TABLE facts 
		ADD COLUMN IF NOT EXISTS rating float8 NOT NULL DEFAULT 0.0
	`)
	if err != nil {
		return fmt.Errorf("failed to add rating column to facts table: %w", err)
	}

	return nil
}
