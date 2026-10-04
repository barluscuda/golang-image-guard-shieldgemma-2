package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/port"
	"gorm.io/gorm"
)

type ImageRepository struct{ db *gorm.DB }

func NewImageRepository(db *gorm.DB) *ImageRepository { return &ImageRepository{db: db} }

var _ port.ImageRepository = (*ImageRepository)(nil)

func (r *ImageRepository) CreateQueued(ctx context.Context, image *domain.Image) error {
	model := fromDomain(image)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *ImageRepository) GetByID(ctx context.Context, id string) (*domain.Image, error) {
	var model ImageModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toDomain(model)
}

func (r *ImageRepository) ClaimNext(ctx context.Context) (*domain.Image, error) {
	var model ImageModel
	db := r.db.WithContext(ctx)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ?", domain.StatusQueued).Order("created_at ASC").First(&model).Error; err != nil {
			return err
		}
		result := tx.Model(&ImageModel{}).
			Where("id = ? AND status = ?", model.ID, domain.StatusQueued).
			Updates(map[string]any{"status": domain.StatusProcessing, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		model.Status = domain.StatusProcessing
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(model)
}

func (r *ImageRepository) Complete(ctx context.Context, id string, moderation *domain.ModerationResult) error {
	data, err := json.Marshal(moderation)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&ImageModel{}).
		Where("id = ? AND status = ?", id, domain.StatusProcessing).
		Updates(map[string]any{"status": domain.StatusProcessed, "result_json": string(data), "processing_error": "", "processed_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ImageRepository) Fail(ctx context.Context, id string, code string) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&ImageModel{}).
		Where("id = ? AND status = ?", id, domain.StatusProcessing).
		Updates(map[string]any{"status": domain.StatusProcessed, "processing_error": code, "processed_at": now, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ImageRepository) Requeue(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Model(&ImageModel{}).
		Where("id = ? AND status = ?", id, domain.StatusProcessing).
		Updates(map[string]any{"status": domain.StatusQueued, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ImageRepository) ResetProcessing(ctx context.Context) error {
	return r.db.WithContext(ctx).Model(&ImageModel{}).
		Where("status = ?", domain.StatusProcessing).
		Updates(map[string]any{"status": domain.StatusQueued, "updated_at": time.Now().UTC()}).Error
}

func (r *ImageRepository) ListPendingCleanup(ctx context.Context, limit int) ([]domain.Image, error) {
	var models []ImageModel
	if err := r.db.WithContext(ctx).Where("status = ? AND deleted_at IS NULL", domain.StatusProcessed).
		Order("processed_at ASC").Limit(limit).Find(&models).Error; err != nil {
		return nil, err
	}
	images := make([]domain.Image, 0, len(models))
	for _, model := range models {
		image, err := toDomain(model)
		if err != nil {
			return nil, err
		}
		images = append(images, *image)
	}
	return images, nil
}

func (r *ImageRepository) MarkDeleted(ctx context.Context, id string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&ImageModel{}).
		Where("id = ? AND status = ? AND deleted_at IS NULL", id, domain.StatusProcessed).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error
}

func fromDomain(image *domain.Image) ImageModel {
	var resultJSON *string
	if image.Result != nil {
		data, _ := json.Marshal(image.Result)
		value := string(data)
		resultJSON = &value
	}
	return ImageModel{ID: image.ID, OriginalFilename: image.OriginalFilename, ContentType: image.ContentType,
		SizeBytes: image.SizeBytes, StorageKey: image.StorageKey, Status: image.Status,
		ResultJSON: resultJSON, ProcessingError: image.ProcessingError, PolicyHash: image.PolicyHash,
		PolicyText: image.PolicyText,
		CreatedAt:  image.CreatedAt, UpdatedAt: image.UpdatedAt, ProcessedAt: image.ProcessedAt, DeletedAt: image.DeletedAt}
}

func toDomain(model ImageModel) (*domain.Image, error) {
	image := &domain.Image{ID: model.ID, OriginalFilename: model.OriginalFilename, ContentType: model.ContentType,
		SizeBytes: model.SizeBytes, StorageKey: model.StorageKey, Status: model.Status,
		ProcessingError: model.ProcessingError, PolicyHash: model.PolicyHash,
		PolicyText: model.PolicyText,
		CreatedAt:  model.CreatedAt, UpdatedAt: model.UpdatedAt, ProcessedAt: model.ProcessedAt, DeletedAt: model.DeletedAt}
	if model.ResultJSON != nil && *model.ResultJSON != "" {
		var result domain.ModerationResult
		if err := json.Unmarshal([]byte(*model.ResultJSON), &result); err != nil {
			return nil, err
		}
		image.Result = &result
	}
	return image, nil
}
