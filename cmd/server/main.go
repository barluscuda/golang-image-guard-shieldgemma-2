package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/adapter/gorm"
	httpadapter "github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/adapter/http"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/adapter/policy"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/adapter/shieldgemma"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/adapter/storage"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/config"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/repository"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/service"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/worker"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger, err := newLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	defer logger.Sync()

	policySnapshot, err := policy.Load(cfg.PolicyPath)
	if err != nil {
		return err
	}
	db, err := gormadapter.Initialize(cfg.DatabaseDSN, &repository.ImageModel{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sqlite connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()

	repo := repository.NewImageRepository(db)
	if err := repo.ResetProcessing(context.Background()); err != nil {
		return fmt.Errorf("recover interrupted images: %w", err)
	}
	fileStorage, err := storage.NewLocal(cfg.StorageDirectory)
	if err != nil {
		return err
	}
	imageService := service.NewImageService(repo, fileStorage, policySnapshot.Text, policySnapshot.Hash, cfg.MaxImageBytes)
	moderator := shieldgemma.New(cfg.ModelEndpoint, cfg.ModelName, cfg.ModelTimeout, cfg.MaxImageBytes, logger)
	moderationService := service.NewModerationService(repo, fileStorage, moderator, logger)
	backgroundWorker := worker.New(moderationService, cfg.WorkerPollInterval, logger)
	router := httpadapter.NewRouter(imageService, cfg.MaxImageBytes, logger)
	server := &http.Server{Addr: cfg.Address, Handler: router, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		if err := backgroundWorker.Run(ctx); err != nil {
			logger.Error("worker stopped", zap.Error(err))
		}
	}()
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", zap.String("address", cfg.Address), zap.String("policy_hash", policySnapshot.Hash))
		serverErr <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", zap.Error(err))
		}
		stop()
	case <-workerDone:
		logger.Error("worker stopped unexpectedly")
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
	}
	stop()
	select {
	case <-workerDone:
	case <-shutdownCtx.Done():
	}
	return nil
}

func newLogger(level string) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	if err := config.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}
	return config.Build()
}
