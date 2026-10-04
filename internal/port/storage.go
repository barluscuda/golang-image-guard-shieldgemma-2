package port

import (
	"context"
	"io"
)

type StoredImage struct {
	StorageKey  string
	ContentType string
	SizeBytes   int64
}

type ImageStorage interface {
	Store(context.Context, string, io.Reader, int64) (StoredImage, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}
