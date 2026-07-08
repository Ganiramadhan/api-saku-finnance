package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client interface {
	SendMessage(ctx context.Context, chatID int64, text string, opts ...SendMessageOption) error
	ClearInlineKeyboard(ctx context.Context, chatID, messageID int64) error
	GetFile(ctx context.Context, fileID string) (File, error)
	DownloadFile(ctx context.Context, filePath string) ([]byte, string, error)
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

type telegramAPIResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description,omitempty"`
}

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

func (c *HTTPClient) GetFile(ctx context.Context, fileID string) (File, error) {
	if c == nil || c.token == "" || strings.TrimSpace(fileID) == "" {
		return File{}, nil
	}
	body, err := json.Marshal(map[string]any{"file_id": fileID})
	if err != nil {
		return File{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/getFile", c.token), bytes.NewReader(body))
	if err != nil {
		return File{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return File{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return File{}, fmt.Errorf("telegram get file failed: status %d", res.StatusCode)
	}
	var out telegramAPIResponse[File]
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return File{}, err
	}
	if !out.OK {
		if out.Description != "" {
			return File{}, fmt.Errorf("telegram get file failed: %s", out.Description)
		}
		return File{}, fmt.Errorf("telegram get file failed")
	}
	return out.Result, nil
}

func (c *HTTPClient) DownloadFile(ctx context.Context, filePath string) ([]byte, string, error) {
	if c == nil || c.token == "" || strings.TrimSpace(filePath) == "" {
		return nil, "", nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", c.token, strings.TrimLeft(filePath, "/")), nil)
	if err != nil {
		return nil, "", err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, "", fmt.Errorf("telegram download file failed: status %d", res.StatusCode)
	}
	const maxTelegramFileBytes = int64(8 << 20)
	data, err := io.ReadAll(io.LimitReader(res.Body, maxTelegramFileBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxTelegramFileBytes {
		return nil, "", fmt.Errorf("telegram file exceeds 8MB limit")
	}
	mediaType := strings.TrimSpace(strings.Split(res.Header.Get("Content-Type"), ";")[0])
	return data, mediaType, nil
}
