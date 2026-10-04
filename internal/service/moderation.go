package service

import (
	"context"
	"fmt"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/port"
	"go.uber.org/zap"
)

type ModerationService struct {
	repository port.ImageRepository
	storage    port.ImageStorage
	moderator  port.ImageModerator
	logger     *zap.Logger
}

func NewModerationService(repository port.ImageRepository, storage port.ImageStorage, moderator port.ImageModerator, logger *zap.Logger) *ModerationService {
	return &ModerationService{repository: repository, storage: storage, moderator: moderator, logger: logger}
}

var _ port.ModerationApplication = (*ModerationService)(nil)

func (s *ModerationService) ProcessNext(ctx context.Context) (bool, error) {
	image, err := s.repository.ClaimNext(ctx)
	if err != nil {
		return false, err
	}
	if image == nil {
		return false, s.Cleanup(ctx)
	}
	fields := []zap.Field{zap.String("image_id", image.ID)}
	file, err := s.storage.Open(ctx, image.StorageKey)
	if err != nil {
		return true, s.failAndCleanup(ctx, image, "image_read_failed", err, fields)
	}
	result, moderationErr := s.moderator.Moderate(ctx, file, image.ContentType, image.PolicyText)
	closeErr := file.Close()
	if moderationErr == nil && closeErr != nil {
		moderationErr = fmt.Errorf("close image after moderation: %w", closeErr)
	}
	if moderationErr != nil {
		return true, s.failAndCleanup(ctx, image, "moderation_failed", moderationErr, fields)
	}
	if err := s.repository.Complete(ctx, image.ID, result); err != nil {
		if requeueErr := s.repository.Requeue(context.WithoutCancel(ctx), image.ID); requeueErr != nil {
			s.logger.Error("could not requeue image after result persistence failure", append(fields, zap.Error(requeueErr))...)
		}
		return true, fmt.Errorf("save moderation result: %w", err)
	}
	s.logger.Info("image moderation completed", append(fields, zap.String("verdict", string(result.Verdict)))...)
	return true, s.cleanupImage(ctx, image, fields)
}

func (s *ModerationService) failAndCleanup(ctx context.Context, image *domain.Image, code string, cause error, fields []zap.Field) error {
	if ctx.Err() != nil {
		if err := s.repository.Requeue(context.WithoutCancel(ctx), image.ID); err != nil {
			return fmt.Errorf("moderation canceled (%v) and requeue failed: %w", cause, err)
		}
		return nil
	}
	if err := s.repository.Fail(ctx, image.ID, code); err != nil {
		return fmt.Errorf("save terminal processing error after %v: %w", cause, err)
	}
	s.logger.Warn("image processing failed", append(fields, zap.String("error_code", code), zap.Error(cause))...)
	return s.cleanupImage(ctx, image, fields)
}

func (s *ModerationService) Cleanup(ctx context.Context) error {
	images, err := s.repository.ListPendingCleanup(ctx, 50)
	if err != nil {
		return err
	}
	var firstErr error
	for _, image := range images {
		if err := s.cleanupImage(ctx, &image, []zap.Field{zap.String("image_id", image.ID)}); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *ModerationService) cleanupImage(ctx context.Context, image *domain.Image, fields []zap.Field) error {
	if err := s.storage.Delete(ctx, image.StorageKey); err != nil {
		s.logger.Warn("image file deletion will be retried", append(fields, zap.Error(err))...)
		return err
	}
	if err := s.repository.MarkDeleted(ctx, image.ID); err != nil {
		s.logger.Warn("could not record image file deletion; cleanup will retry", append(fields, zap.Error(err))...)
		return err
	}
	return nil
}
