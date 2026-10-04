package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/port"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	images        port.ImageApplication
	maxImageBytes int64
	logger        *zap.Logger
}

func NewHandler(images port.ImageApplication, maxImageBytes int64, logger *zap.Logger) *Handler {
	return &Handler{images: images, maxImageBytes: maxImageBytes, logger: logger}
}

type uploadResponse struct {
	ImageID string        `json:"image_id"`
	Status  domain.Status `json:"status"`
}

type imageResponse struct {
	ImageID          string                   `json:"image_id"`
	OriginalFilename string                   `json:"original_filename"`
	ContentType      string                   `json:"content_type"`
	SizeBytes        int64                    `json:"size_bytes"`
	Status           domain.Status            `json:"status"`
	Outcome          string                   `json:"outcome"`
	ModerationResult *domain.ModerationResult `json:"moderation_result"`
	ProcessingError  string                   `json:"processing_error"`
	CreatedAt        time.Time                `json:"created_at"`
	ProcessedAt      *time.Time               `json:"processed_at"`
	ImageDeleted     bool                     `json:"image_deleted"`
}

func (h *Handler) Upload(c *gin.Context) {
	// Allow multipart framing overhead beyond the configured file size.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxImageBytes+(1<<20))
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		status := http.StatusBadRequest
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			status = http.StatusRequestEntityTooLarge
		}
		c.JSON(status, gin.H{"error": "invalid multipart upload"})
		return
	}
	defer file.Close()
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	image, err := h.images.Upload(c.Request.Context(), header.Filename, file)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrImageTooLarge):
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrInvalidImage):
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
		default:
			h.logger.Error("image upload failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "image upload failed"})
		}
		return
	}
	location := "/v1/images/" + image.ID
	c.Header("Location", location)
	c.JSON(http.StatusAccepted, uploadResponse{ImageID: image.ID, Status: image.Status})
}

func (h *Handler) Get(c *gin.Context) {
	image, err := h.images.Get(c.Request.Context(), c.Param("id"))
	if errors.Is(err, domain.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}
	if err != nil {
		h.logger.Error("could not load image", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load image"})
		return
	}
	outcome := ""
	if image.Status == domain.StatusProcessed {
		if image.Result != nil {
			outcome = "success"
		} else {
			outcome = "error"
		}
	}
	c.JSON(http.StatusOK, imageResponse{ImageID: image.ID, OriginalFilename: image.OriginalFilename,
		ContentType: image.ContentType, SizeBytes: image.SizeBytes, Status: image.Status, Outcome: outcome,
		ModerationResult: image.Result, ProcessingError: image.ProcessingError, CreatedAt: image.CreatedAt,
		ProcessedAt: image.ProcessedAt, ImageDeleted: image.DeletedAt != nil})
}
