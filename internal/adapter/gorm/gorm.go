package gormadapter

import (
	"fmt"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/adapter/gorm/sqlite"
	"gorm.io/gorm"
)

func Initialize(dsn string, models ...any) (*gorm.DB, error) {
	db, err := sqlite.Connect(dsn, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect sqlite through gorm: %w", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		return nil, fmt.Errorf("auto-migrate sqlite schema: %w", err)
	}
	return db, nil
}
