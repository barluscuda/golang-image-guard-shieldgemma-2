package http

import (
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/port"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(images port.ImageApplication, maxImageBytes int64, logger *zap.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), requestLogger(logger))
	router.GET("/health/live", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	router.GET("/health/ready", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ready"}) })
	handler := NewHandler(images, maxImageBytes, logger)
	router.POST("/v1/images", handler.Upload)
	router.GET("/v1/images/:id", handler.Get)
	return router
}

func requestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("http request", zap.String("method", c.Request.Method), zap.String("path", c.FullPath()),
			zap.Int("status", c.Writer.Status()), zap.Duration("duration", time.Since(started)))
	}
}
