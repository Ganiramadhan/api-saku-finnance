package ai

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Client struct {
	sdk   anthropic.Client
	model anthropic.Model
}

func NewClient(apiKey, model string) *Client {
	sdk := anthropic.NewClient(option.WithAPIKey(apiKey))
	m := anthropic.Model(model)
	if model == "" {
		m = anthropic.ModelClaudeSonnet4_5
	}
	return &Client{sdk: sdk, model: m}
}

func (c *Client) Ask(ctx context.Context, prompt string) (string, error) {
	return c.AskWithSystem(ctx, "", prompt)
}

func (c *Client) AskWithSystem(ctx context.Context, system, prompt string) (string, error) {
	models := []anthropic.Model{c.model}

	var lastErr error
	for _, model := range models {
		params := anthropic.MessageNewParams{
			MaxTokens: 2048,
			Model:     model,
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
			},
		}
		if system != "" {
			params.System = []anthropic.TextBlockParam{{
				Text:         system,
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			}}
		}

		msg, err := c.sdk.Messages.New(ctx, params)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			continue
		}
		if len(msg.Content) == 0 {
			lastErr = fmt.Errorf("empty response from model %s", model)
			continue
		}
		return msg.Content[0].Text, nil
	}

	return "", fmt.Errorf("anthropic messages.new failed after %d attempts. last error: %w", len(models), lastErr)
}

func (c *Client) AskImage(ctx context.Context, system, prompt, mediaType, base64Data string) (string, error) {
	// Use the configured model only. The app already reads CLAUDE_MODEL from env.
	models := []anthropic.Model{c.model}

	var lastErr error
	for _, model := range models {
		params := anthropic.MessageNewParams{
			MaxTokens: 2048,
			Model:     model,
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(
					anthropic.NewImageBlockBase64(mediaType, base64Data),
					anthropic.NewTextBlock(prompt),
				),
			},
		}
		if system != "" {
			params.System = []anthropic.TextBlockParam{{
				Text:         system,
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			}}
		}

		msg, err := c.sdk.Messages.New(ctx, params)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			continue
		}
		if len(msg.Content) == 0 {
			lastErr = fmt.Errorf("empty response from model %s", model)
			continue
		}
		return msg.Content[0].Text, nil
	}

	return "", fmt.Errorf("anthropic messages.new (image) failed after %d attempts. last error: %w", len(models), lastErr)
}
