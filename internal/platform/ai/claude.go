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
	params := anthropic.MessageNewParams{
		MaxTokens: 2048,
		Model:     c.model,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	msg, err := c.sdk.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("anthropic messages.new: %w", err)
	}
	if len(msg.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}
	return msg.Content[0].Text, nil
}

func (c *Client) AskImage(ctx context.Context, system, prompt, mediaType, base64Data string) (string, error) {
	params := anthropic.MessageNewParams{
		MaxTokens: 2048,
		Model:     c.model,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewImageBlockBase64(mediaType, base64Data),
				anthropic.NewTextBlock(prompt),
			),
		},
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	msg, err := c.sdk.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("anthropic messages.new (image): %w", err)
	}
	if len(msg.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}
	return msg.Content[0].Text, nil
}
