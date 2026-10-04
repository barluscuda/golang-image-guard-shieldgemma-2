package shieldgemma

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/barluscuda/golang-image-guard-shieldgemma-2/internal/domain"
	"go.uber.org/zap"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestModerateRequiresModelResponse(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		callErr error
		verdict domain.Verdict
	}{
		{name: "allowed", status: 200, body: `{"choices":[{"message":{"content":"No"}}]}`, verdict: domain.VerdictAllowed},
		{name: "blocked", status: 200, body: `{"choices":[{"message":{"content":"Yes"}}]}`, verdict: domain.VerdictBlocked},
		{name: "unavailable", callErr: errors.New("connection refused")},
		{name: "http error", status: 503, body: `{"choices":[{"message":{"content":"No"}}]}`},
		{name: "empty choices", status: 200, body: `{"choices":[]}`},
		{name: "missing decision", status: 200, body: `{"choices":[{"message":{}}]}`},
		{name: "invalid decision", status: 200, body: `{"choices":[{"message":{"content":"Maybe"}}]}`},
		{name: "invalid json", status: 200, body: `invalid`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := New("http://model.test/v1/chat/completions", "shieldgemma", time.Second, 1024, zap.NewNop())
			calls := 0
			client.http.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != http.MethodPost || req.URL.String() != client.endpoint || req.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("unexpected model request: %s %s, headers %v", req.Method, req.URL, req.Header)
				}
				var request struct {
					Model    string `json:"model"`
					Messages []struct {
						Role    string        `json:"role"`
						Content []contentPart `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if request.Model != "shieldgemma" || len(request.Messages) != 1 || request.Messages[0].Role != "user" || len(request.Messages[0].Content) != 2 {
					t.Fatalf("unexpected model payload: %#v", request)
				}
				parts := request.Messages[0].Content
				if !strings.Contains(parts[0].Text, "test policy") || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,aW1hZ2U=" {
					t.Fatalf("policy or image missing from model request: %#v", parts)
				}
				if tc.callErr != nil {
					return nil, tc.callErr
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})
			result, err := client.Moderate(context.Background(), strings.NewReader("image"), "image/png", "test policy")
			if calls != 1 {
				t.Fatalf("model requests = %d, want 1", calls)
			}
			if tc.verdict == "" {
				if err == nil || result != nil {
					t.Fatalf("invalid model response produced result %#v, error %v", result, err)
				}
			} else if err != nil || result == nil || result.Verdict != tc.verdict {
				t.Fatalf("result = %#v, error = %v, want %q", result, err, tc.verdict)
			}
		})
	}
}
