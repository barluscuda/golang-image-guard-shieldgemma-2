package repository

import (
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
)

type ImageModel struct {
	ID               string        `gorm:"primaryKey;size:64"`
	OriginalFilename string        `gorm:"not null;size:255"`
	ContentType      string        `gorm:"not null;size:100"`
	SizeBytes        int64         `gorm:"not null"`
	StorageKey       string        `gorm:"not null;uniqueIndex;size:128"`
	Status           domain.Status `gorm:"not null;index;size:24"`
	ResultJSON       *string       `gorm:"type:text"`
	ProcessingError  string        `gorm:"size:128"`
	PolicyHash       string        `gorm:"not null;size:64"`
	PolicyText       string        `gorm:"not null;type:text"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ProcessedAt      *time.Time
	DeletedAt        *time.Time
}

func (ImageModel) TableName() string { return "images" }
