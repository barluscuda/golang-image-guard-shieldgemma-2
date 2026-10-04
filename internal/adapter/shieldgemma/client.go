package shieldgemma

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/port"
)

type Client struct {
	endpoint string
	model    string
	http     *http.Client
	maxBytes int64
}

func New(endpoint, model string, timeout time.Duration, maxBytes int64) *Client {
	return &Client{endpoint: endpoint, model: model, maxBytes: maxBytes, http: &http.Client{Timeout: timeout}}
}

var _ port.ImageModerator = (*Client)(nil)

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}
type imageURL struct {
	URL string `json:"url"`
}
type requestMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}
type requestBody struct {
	Model       string           `json:"model"`
	Messages    []requestMessage `json:"messages"`
	Temperature float64          `json:"temperature"`
	MaxTokens   int              `json:"max_tokens"`
	Stream      bool             `json:"stream"`
}
type responseBody struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) Moderate(ctx context.Context, imageReader io.Reader, contentType, policy string) (*domain.ModerationResult, error) {
	imageBytes, err := io.ReadAll(io.LimitReader(imageReader, c.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read image for moderation: %w", err)
	}
	if len(imageBytes) == 0 || int64(len(imageBytes)) > c.maxBytes {
		return nil, domain.ErrInvalidImage
	}
	dataURL := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(imageBytes)
	prompt := "You are a policy expert determining whether an image violates the specific safety policy below. Follow only the policy language.\n\n<BEGIN_SAFETY_POLICY>\n" + policy + "\n</END_SAFETY_POLICY>\n\nDoes the image violate this policy? Start your answer with Yes or No."
	body := requestBody{Model: c.model, Temperature: 0, MaxTokens: 8, Stream: false,
		Messages: []requestMessage{{Role: "user", Content: []contentPart{{Type: "text", Text: prompt}, {Type: "image_url", ImageURL: &imageURL{URL: dataURL}}}}}}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call llama.cpp: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llama.cpp returned status %d", resp.StatusCode)
	}
	var decoded responseBody
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode llama.cpp response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return nil, domain.ErrInvalidDecision
	}
	answer := strings.TrimSpace(decoded.Choices[0].Message.Content)
	words := strings.Fields(answer)
	if len(words) == 0 {
		return nil, domain.ErrInvalidDecision
	}
	first := strings.ToLower(strings.Trim(words[0], " \t\r\n.,:;!\"'`"))
	violates := false
	switch first {
	case "yes":
		violates = true
	case "no":
	default:
		return nil, domain.ErrInvalidDecision
	}
	verdict := domain.VerdictAllowed
	if violates {
		verdict = domain.VerdictBlocked
	}
	return &domain.ModerationResult{Verdict: verdict, ViolatesPolicy: violates, Model: c.model}, nil
}
