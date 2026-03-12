package commit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexivison/scry/internal/model"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// --- NewClaudeProvider tests ---

func TestNewClaudeProvider_missingAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := NewClaudeProvider("", "")
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("error = %v, want ErrMissingAPIKey", err)
	}
}

func TestNewClaudeProvider_envFallback(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-from-env")

	p, err := NewClaudeProvider("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}

func TestNewClaudeProvider_validKey(t *testing.T) {
	t.Parallel()

	p, err := NewClaudeProvider("sk-test-key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}

// --- ClaudeProvider.Generate tests ---

func claudeResponse(text string) []byte {
	resp := map[string]any{
		"id":   "msg_test",
		"type": "message",
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
		"model":       "claude-sonnet-4-6",
		"stop_reason": "end_turn",
		"usage":       map[string]any{"input_tokens": 100, "output_tokens": 50},
	}
	b, _ := json.Marshal(resp)
	return b
}

func claudeError(status int, errType, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		resp := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    errType,
				"message": message,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}
}

func TestClaudeProvider_Generate_success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(claudeResponse("feat: add logging\n\nAdd structured logging to the main module."))
	}))
	defer srv.Close()

	p, err := NewClaudeProvider("sk-test-key", "", option.WithBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("NewClaudeProvider: %v", err)
	}

	files := []model.FileSummary{
		{Path: "main.go", Status: model.StatusModified, Additions: 5, Deletions: 1},
	}

	msg, err := p.Generate(context.Background(), fixtureDiff, files)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if msg == "" {
		t.Error("generated message is empty")
	}
	if msg != "feat: add logging\n\nAdd structured logging to the main module." {
		t.Errorf("msg = %q, want expected commit message", msg)
	}
}

func TestClaudeProvider_Generate_authFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(claudeError(401, "authentication_error", "Invalid API key"))
	defer srv.Close()

	p, err := NewClaudeProvider("sk-bad-key", "", option.WithBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("NewClaudeProvider: %v", err)
	}

	_, err = p.Generate(context.Background(), fixtureDiff, fixtureFiles)
	if err == nil {
		t.Fatal("expected error for auth failure, got nil")
	}
	if !errors.Is(err, ErrProviderRequest) {
		t.Errorf("error = %v, want ErrProviderRequest", err)
	}
}

func TestClaudeProvider_Generate_rateLimitError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(claudeError(429, "rate_limit_error", "Rate limited"))
	defer srv.Close()

	p, err := NewClaudeProvider("sk-test-key", "", option.WithBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("NewClaudeProvider: %v", err)
	}

	_, err = p.Generate(context.Background(), fixtureDiff, fixtureFiles)
	if err == nil {
		t.Fatal("expected error for rate limit, got nil")
	}
	if !errors.Is(err, ErrProviderRequest) {
		t.Errorf("error = %v, want ErrProviderRequest", err)
	}
}

func TestClaudeProvider_Generate_truncated(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":   "msg_test",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "feat: partial message that got cut"},
			},
			"model":       "claude-sonnet-4-6",
			"stop_reason": "max_tokens",
			"usage":       map[string]any{"input_tokens": 100, "output_tokens": 1024},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewClaudeProvider("sk-test-key", "", option.WithBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("NewClaudeProvider: %v", err)
	}

	_, err = p.Generate(context.Background(), fixtureDiff, fixtureFiles)
	if err == nil {
		t.Fatal("expected error for max_tokens stop reason, got nil")
	}
	if !errors.Is(err, ErrMalformedResponse) {
		t.Errorf("error = %v, want ErrMalformedResponse", err)
	}
}

func TestClaudeProvider_Generate_malformedResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Response with no text content blocks.
		resp := map[string]any{
			"id":          "msg_test",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]any{},
			"model":       "claude-sonnet-4-6",
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 100, "output_tokens": 0},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewClaudeProvider("sk-test-key", "", option.WithBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("NewClaudeProvider: %v", err)
	}

	_, err = p.Generate(context.Background(), fixtureDiff, fixtureFiles)
	if err == nil {
		t.Fatal("expected error for malformed response, got nil")
	}
	if !errors.Is(err, ErrMalformedResponse) {
		t.Errorf("error = %v, want ErrMalformedResponse", err)
	}
}
