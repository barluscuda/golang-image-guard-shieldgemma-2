package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/port"
)

type ImageService struct {
	repository    port.ImageRepository
	storage       port.ImageStorage
	policyText    string
	policyHash    string
	maxImageBytes int64
}

func NewImageService(repository port.ImageRepository, storage port.ImageStorage, policyText, policyHash string, maxImageBytes int64) *ImageService {
	return &ImageService{repository: repository, storage: storage, policyText: policyText, policyHash: policyHash, maxImageBytes: maxImageBytes}
}

var _ port.ImageApplication = (*ImageService)(nil)

func (s *ImageService) Upload(ctx context.Context, filename string, src io.Reader) (*domain.Image, error) {
	var rawID [16]byte
	if _, err := rand.Read(rawID[:]); err != nil {
		return nil, fmt.Errorf("generate image id: %w", err)
	}
	id := hex.EncodeToString(rawID[:])
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "image"
	}
	if len(filename) > 255 {
		filename = filename[:255]
	}
	stored, err := s.storage.Store(ctx, id, src, s.maxImageBytes)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	image := &domain.Image{ID: id, OriginalFilename: filename, ContentType: stored.ContentType,
		SizeBytes: stored.SizeBytes, StorageKey: stored.StorageKey, Status: domain.StatusQueued,
		PolicyHash: s.policyHash, PolicyText: s.policyText, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.CreateQueued(ctx, image); err != nil {
		_ = s.storage.Delete(context.WithoutCancel(ctx), stored.StorageKey)
		return nil, fmt.Errorf("save queued image: %w", err)
	}
	return image, nil
}

func (s *ImageService) Get(ctx context.Context, id string) (*domain.Image, error) {
	return s.repository.GetByID(ctx, id)
}
