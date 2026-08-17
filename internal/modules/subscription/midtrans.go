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

func (m *MidtransClient) coreBaseURL() string {
	if m.isProduction {
		return "https://api.midtrans.com/v2"
	}
	return "https://api.sandbox.midtrans.com/v2"
}

type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

// QRISChargeResponse is the Core API /v2/charge response for payment_type=qris.
type QRISChargeResponse struct {
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	PaymentType       string `json:"payment_type"`
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	ExpiryTime        string `json:"expiry_time"`
	Acquirer          string `json:"acquirer"`
	QRString          string `json:"qr_string"`
	Actions           []struct {
		Name   string `json:"name"`
		Method string `json:"method"`
		URL    string `json:"url"`
	} `json:"actions"`
}

type TransactionStatusResponse struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	PaymentType       string `json:"payment_type"`
	TransactionID     string `json:"transaction_id"`
	TransactionTime   string `json:"transaction_time"`
	ExpiryTime        string `json:"expiry_time"`
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

// ChargeQRIS creates a QRIS-only Core API transaction (bypasses Snap entirely,
// so the caller can render its own payment UI from the returned qr_string/actions).
func (m *MidtransClient) ChargeQRIS(ctx context.Context, payload map[string]any) (*QRISChargeResponse, error) {
	if !m.Enabled() {
		return nil, errors.New("midtrans is not configured (MIDTRANS_SERVER_KEY missing)")
	}
	payload["payment_type"] = "qris"
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.coreBaseURL()+"/charge", bytes.NewReader(body))
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
		return nil, fmt.Errorf("midtrans qris charge error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out QRISChargeResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("midtrans qris charge decode: %w", err)
	}
	if out.QRString == "" && out.QRImageURL() == "" {
		return nil, errors.New("midtrans qris charge: response has neither qr_string nor a QR image action")
	}
	return &out, nil
}

// QRImageURL returns the "generate-qr-code" action URL, i.e. the Midtrans-hosted
// QR image link. This is what Midtrans's own sandbox Payment Simulator expects
// pasted into it — the raw qr_string is a different, unrelated input there and
// gets rejected as "QR inputted unparsable".
func (r *QRISChargeResponse) QRImageURL() string {
	for _, action := range r.Actions {
		if action.Name == "generate-qr-code" && action.URL != "" {
			return action.URL
		}
	}
	if len(r.Actions) > 0 {
		return r.Actions[0].URL
	}
	return ""
}

func (m *MidtransClient) CancelTransaction(ctx context.Context, orderID string) error {
	if !m.Enabled() {
		return errors.New("midtrans is not configured (MIDTRANS_SERVER_KEY missing)")
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return errors.New("midtrans cancel: empty order id")
	}
	url := fmt.Sprintf("%s/%s/cancel", m.coreBaseURL(), orderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	auth := base64.StdEncoding.EncodeToString([]byte(m.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		body := strings.TrimSpace(string(raw))
		lower := strings.ToLower(body)
		if resp.StatusCode == http.StatusNotFound ||
			strings.Contains(lower, "already expired") ||
			strings.Contains(lower, "already canceled") ||
			strings.Contains(lower, "transaction doesn't exist") {
			return nil
		}
		return fmt.Errorf("midtrans cancel error %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (m *MidtransClient) GetTransactionStatus(ctx context.Context, orderID string) (*TransactionStatusResponse, error) {
	if !m.Enabled() {
		return nil, errors.New("midtrans is not configured (MIDTRANS_SERVER_KEY missing)")
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, errors.New("midtrans status: empty order id")
	}
	url := fmt.Sprintf("%s/%s/status", m.coreBaseURL(), orderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("midtrans status error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out TransactionStatusResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("midtrans status decode: %w", err)
	}
	if out.OrderID == "" {
		out.OrderID = orderID
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
