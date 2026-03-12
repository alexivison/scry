package commit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexivison/scry/internal/model"
	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const DefaultModel = "claude-sonnet-4-6"

// ClaudeProvider generates commit messages using the Anthropic Messages API.
type ClaudeProvider struct {
	client anthropic.Client
	model  anthropic.Model
}

// NewClaudeProvider creates a ClaudeProvider. If apiKey is empty, ANTHROPIC_API_KEY
// is read from the environment. Pass option.WithBaseURL for testing.
func NewClaudeProvider(apiKey string, modelOverride string, opts ...option.RequestOption) (*ClaudeProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	allOpts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)

	m := anthropic.Model(DefaultModel)
	if modelOverride != "" {
		m = anthropic.Model(modelOverride)
	}

	return &ClaudeProvider{
		client: anthropic.NewClient(allOpts...),
		model:  m,
	}, nil
}

// Generate sends the diff and file summaries to Claude and returns the commit message.
func (p *ClaudeProvider) Generate(ctx context.Context, diff string, files []model.FileSummary) (string, error) {
	prompt := BuildPrompt(diff, files)

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrProviderRequest, err)
	}

	if resp.StopReason != anthropic.StopReasonEndTurn {
		return "", fmt.Errorf("%w: unexpected stop reason %q", ErrMalformedResponse, resp.StopReason)
	}

	text := collectText(resp)
	if text == "" {
		return "", fmt.Errorf("%w: no text content in response", ErrMalformedResponse)
	}

	return strings.TrimSpace(text), nil
}

// collectText concatenates all text content blocks in the response.
func collectText(msg *anthropic.Message) string {
	var b strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}
