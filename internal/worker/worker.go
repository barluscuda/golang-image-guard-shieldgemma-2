package worker

import (
	"context"
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/port"
	"go.uber.org/zap"
)

type Worker struct {
	moderation port.ModerationApplication
	interval   time.Duration
	logger     *zap.Logger
}

func New(moderation port.ModerationApplication, interval time.Duration, logger *zap.Logger) *Worker {
	return &Worker{moderation: moderation, interval: interval, logger: logger}
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.moderation.Cleanup(ctx); err != nil {
		w.logger.Warn("initial image cleanup failed", zap.Error(err))
	}
	for {
		worked, err := w.moderation.ProcessNext(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			w.logger.Error("image processing cycle failed", zap.Error(err))
		}
		if worked && err == nil {
			continue
		}
		timer := time.NewTimer(w.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
