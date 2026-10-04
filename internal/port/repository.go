package port

import (
	"context"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
)

type ImageRepository interface {
	CreateQueued(context.Context, *domain.Image) error
	GetByID(context.Context, string) (*domain.Image, error)
	ClaimNext(context.Context) (*domain.Image, error)
	Complete(context.Context, string, *domain.ModerationResult) error
	Fail(context.Context, string, string) error
	Requeue(context.Context, string) error
	ResetProcessing(context.Context) error
	ListPendingCleanup(context.Context, int) ([]domain.Image, error)
	MarkDeleted(context.Context, string) error
}
