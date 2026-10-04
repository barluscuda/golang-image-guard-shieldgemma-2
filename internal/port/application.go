package port

import (
	"context"
	"io"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
)

type ImageApplication interface {
	Upload(context.Context, string, io.Reader) (*domain.Image, error)
	Get(context.Context, string) (*domain.Image, error)
}

type ModerationApplication interface {
	ProcessNext(context.Context) (bool, error)
	Cleanup(context.Context) error
}
