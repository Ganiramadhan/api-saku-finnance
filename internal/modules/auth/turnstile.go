package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type turnstileVerifier struct {
	secret string
	client *http.Client
}

func newTurnstileVerifier(secret string) *turnstileVerifier {
	return &turnstileVerifier{
		secret: strings.TrimSpace(secret),
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (v *turnstileVerifier) Enabled() bool {
	return v != nil && v.secret != ""
}

func (v *turnstileVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if !v.Enabled() {
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("%w: captcha verification is required", domain.ErrInvalidInput)
	}
	form := url.Values{}
	form.Set("secret", v.secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("captcha verification failed: %w", err)
	}
	defer res.Body.Close()
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return fmt.Errorf("captcha verification failed: %w", err)
	}
	if !out.Success {
		return fmt.Errorf("%w: captcha verification failed", domain.ErrInvalidInput)
	}
	return nil
}
