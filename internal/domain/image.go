package domain

import "time"

type ModerationResult struct {
	Verdict        Verdict `json:"verdict"`
	ViolatesPolicy bool    `json:"violates_policy"`
	Model          string  `json:"model"`
}

type Image struct {
	ID               string
	OriginalFilename string
	ContentType      string
	SizeBytes        int64
	StorageKey       string
	Status           Status
	Result           *ModerationResult
	ProcessingError  string
	PolicyHash       string
	PolicyText       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ProcessedAt      *time.Time
	DeletedAt        *time.Time
}
