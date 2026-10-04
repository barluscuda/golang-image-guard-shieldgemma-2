package storage

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/port"
)

type Local struct{ root string }

func NewLocal(root string) (*Local, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create image directory: %w", err)
	}
	return &Local{root: root}, nil
}

var _ port.ImageStorage = (*Local)(nil)

func (s *Local) Store(ctx context.Context, id string, src io.Reader, maxBytes int64) (port.StoredImage, error) {
	if err := ctx.Err(); err != nil {
		return port.StoredImage{}, err
	}
	tmp, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return port.StoredImage{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	n, copyErr := io.Copy(tmp, io.LimitReader(src, maxBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return port.StoredImage{}, copyErr
	}
	if closeErr != nil {
		return port.StoredImage{}, closeErr
	}
	if n > maxBytes {
		return port.StoredImage{}, domain.ErrImageTooLarge
	}
	if n == 0 {
		return port.StoredImage{}, domain.ErrInvalidImage
	}

	file, err := os.Open(tmpName)
	if err != nil {
		return port.StoredImage{}, err
	}
	config, format, decodeErr := image.DecodeConfig(file)
	closeErr = file.Close()
	if decodeErr != nil || closeErr != nil || config.Width <= 0 || config.Height <= 0 {
		return port.StoredImage{}, domain.ErrInvalidImage
	}
	if format != "jpeg" && format != "png" {
		return port.StoredImage{}, domain.ErrInvalidImage
	}
	if config.Width > 12000 || config.Height > 12000 || int64(config.Width)*int64(config.Height) > 40000000 {
		return port.StoredImage{}, domain.ErrInvalidImage
	}

	ext := ".jpg"
	contentType := "image/jpeg"
	if format == "png" {
		ext = ".png"
		contentType = "image/png"
	}
	key := id + ext
	if err := os.Rename(tmpName, filepath.Join(s.root, key)); err != nil {
		return port.StoredImage{}, err
	}
	return port.StoredImage{StorageKey: key, ContentType: contentType, SizeBytes: n}, nil
}

func (s *Local) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if filepath.Base(key) != key || strings.ContainsAny(key, `/\`) {
		return nil, errors.New("invalid storage key")
	}
	return os.Open(filepath.Join(s.root, key))
}

func (s *Local) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if filepath.Base(key) != key || strings.ContainsAny(key, `/\`) {
		return errors.New("invalid storage key")
	}
	err := os.Remove(filepath.Join(s.root, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
