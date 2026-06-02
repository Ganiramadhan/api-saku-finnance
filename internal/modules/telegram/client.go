package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client interface {
	SendMessage(ctx context.Context, chatID int64, text string, opts ...SendMessageOption) error
	ClearInlineKeyboard(ctx context.Context, chatID, messageID int64) error
}

type HTTPClient struct {
	token  string
	client *http.Client
}

func NewHTTPClient(token string) Client {
	return &HTTPClient{
		token: token,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type SendMessageConfig struct {
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type SendMessageOption func(*SendMessageConfig)

func WithInlineKeyboard(markup InlineKeyboardMarkup) SendMessageOption {
	return func(cfg *SendMessageConfig) {
		cfg.ReplyMarkup = &markup
	}
}

func (c *HTTPClient) SendMessage(ctx context.Context, chatID int64, text string, opts ...SendMessageOption) error {
	if c == nil || c.token == "" {
		return nil
	}
	cfg := SendMessageConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if cfg.ReplyMarkup != nil {
		payload["reply_markup"] = cfg.ReplyMarkup
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("telegram send message failed: status %d", res.StatusCode)
	}
	return nil
}

func (c *HTTPClient) ClearInlineKeyboard(ctx context.Context, chatID, messageID int64) error {
	if c == nil || c.token == "" || chatID == 0 || messageID == 0 {
		return nil
	}
	body, err := json.Marshal(map[string]any{
		"chat_id":      chatID,
		"message_id":   messageID,
		"reply_markup": InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{}},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/editMessageReplyMarkup", c.token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("telegram clear inline keyboard failed: status %d", res.StatusCode)
	}
	return nil
}
