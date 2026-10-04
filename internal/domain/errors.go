package domain

import "errors"

var (
	ErrNotFound        = errors.New("image not found")
	ErrInvalidImage    = errors.New("invalid or unsupported image")
	ErrImageTooLarge   = errors.New("image exceeds size limit")
	ErrPolicyEmpty     = errors.New("moderation policy is empty")
	ErrInvalidDecision = errors.New("moderation response did not contain a Yes or No decision")
)
