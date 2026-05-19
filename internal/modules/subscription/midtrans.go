package subscription

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type MidtransClient struct {
	serverKey    string
	isProduction bool
	http         *http.Client
}

func NewMidtransClient(serverKey string, isProduction bool) *MidtransClient {
	return &MidtransClient{
		serverKey:    serverKey,
		isProduction: isProduction,
		http:         &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MidtransClient) Enabled() bool { return strings.TrimSpace(m.serverKey) != "" }

func (m *MidtransClient) snapBaseURL() string {
	if m.isProduction {
		return "https://app.midtrans.com/snap/v1/transactions"
	}
	return "https://app.sandbox.midtrans.com/snap/v1/transactions"
}

type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

func (m *MidtransClient) CreateSnapTransaction(ctx context.Context, payload map[string]any) (*SnapResponse, error) {
	if !m.Enabled() {
		return nil, errors.New("midtrans is not configured (MIDTRANS_SERVER_KEY missing)")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.snapBaseURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	auth := base64.StdEncoding.EncodeToString([]byte(m.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := m.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("midtrans snap error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out SnapResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("midtrans snap decode: %w", err)
	}
	if out.Token == "" {
		return nil, errors.New("midtrans snap: empty token")
	}
	return &out, nil
}

func (m *MidtransClient) VerifySignature(orderID, statusCode, grossAmount, signature string) bool {
	if signature == "" {
		return false
	}
	h := sha512.Sum512([]byte(orderID + statusCode + grossAmount + m.serverKey))
	return strings.EqualFold(hex.EncodeToString(h[:]), signature)
}
