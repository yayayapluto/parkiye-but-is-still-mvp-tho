package payment

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"parkieee/pkg/config"
)

type midtransClient struct {
	serverKey string
	baseURL   string
}

func newMidtransClient(cfg config.MidtransConfig) *midtransClient {
	base := "https://api.sandbox.midtrans.com"
	if cfg.Env == "production" {
		base = "https://api.midtrans.com"
	}
	return &midtransClient{serverKey: cfg.ServerKey, baseURL: base}
}

func (m *midtransClient) authHeader() string {
	encoded := base64.StdEncoding.EncodeToString([]byte(m.serverKey + ":"))
	return "Basic " + encoded
}

type qrisChargeResponse struct {
	TransactionID string `json:"transaction_id"`
	OrderID       string `json:"order_id"`
	QRString      string `json:"qr_string"`
	Actions       []struct {
		Name   string `json:"name"`
		Method string `json:"method"`
		URL    string `json:"url"`
	} `json:"actions"`
	TransactionStatus string `json:"transaction_status"`
	StatusCode        string `json:"status_code"`
	StatusMessage     string `json:"status_message"`
}

func (m *midtransClient) chargeQRIS(orderID string, grossAmount int) (*qrisChargeResponse, error) {
	body := map[string]any{
		"payment_type": "qris",
		"transaction_details": map[string]any{
			"order_id":     orderID,
			"gross_amount": grossAmount,
		},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal charge request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, m.baseURL+"/v2/charge", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create charge request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", m.authHeader())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute charge request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read charge response: %w", err)
	}

	var result qrisChargeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal charge response: %w", err)
	}

	if result.StatusCode != "201" {
		return nil, fmt.Errorf("midtrans charge failed: [%s] %s", result.StatusCode, result.StatusMessage)
	}

	return &result, nil
}

func (m *midtransClient) refund(midtransTransactionID, refundKey string, amount int, reason string) error {
	body := map[string]any{
		"refund_key": refundKey,
		"amount":     amount,
		"reason":     reason,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal refund request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, m.baseURL+"/v2/"+midtransTransactionID+"/refund", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create refund request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", m.authHeader())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute refund request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("midtrans refund failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
