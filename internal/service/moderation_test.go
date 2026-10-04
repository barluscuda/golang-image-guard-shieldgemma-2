package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	gormadapter "github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/adapter/gorm"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/adapter/storage"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/repository"
	"go.uber.org/zap"
)

type allowModerator struct{}

func (allowModerator) Moderate(_ context.Context, _ io.Reader, _, policy string) (*domain.ModerationResult, error) {
	if policy != "test policy" {
		return nil, errors.New("wrong policy passed to moderator")
	}
	return &domain.ModerationResult{Verdict: domain.VerdictAllowed, Model: "test-model"}, nil
}

func TestUploadModeratePersistAndDelete(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := gormadapter.Initialize(filepath.Join(root, "test.db")+"?_busy_timeout=5000", &repository.ImageModel{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	repo := repository.NewImageRepository(db)
	files, err := storage.NewLocal(filepath.Join(root, "images"))
	if err != nil {
		t.Fatal(err)
	}
	var pngData bytes.Buffer
	fixture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	fixture.Set(0, 0, color.RGBA{R: 40, G: 80, B: 120, A: 255})
	if err := png.Encode(&pngData, fixture); err != nil {
		t.Fatal(err)
	}
	images := NewImageService(repo, files, "test policy", "test-policy-hash", 1024)
	queued, err := images.Upload(ctx, "fixture.png", bytes.NewReader(pngData.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if queued.Status != domain.StatusQueued {
		t.Fatalf("upload status = %q, want queued", queued.Status)
	}
	storedPath := filepath.Join(root, "images", queued.StorageKey)
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("uploaded file was not stored: %v", err)
	}

	moderation := NewModerationService(repo, files, allowModerator{}, zap.NewNop())
	worked, err := moderation.ProcessNext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !worked {
		t.Fatal("worker did not claim queued image")
	}
	processed, err := images.Get(ctx, queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if processed.Status != domain.StatusProcessed {
		t.Fatalf("status = %q, want processed", processed.Status)
	}
	if processed.Result == nil || processed.Result.Verdict != domain.VerdictAllowed {
		t.Fatalf("unexpected moderation result: %#v", processed.Result)
	}
	if processed.DeletedAt == nil {
		t.Fatal("database does not record image deletion")
	}
	if _, err := os.Stat(storedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("image file still exists or stat failed: %v", err)
	}
}
