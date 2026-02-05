package tools

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

// WhatsAppClient is a thin client for WhatsApp Cloud API calls that are tenant-specific.
type WhatsAppClient struct {
	AccessToken   string
	ApiVersion    string // e.g. v24.0
	PhoneNumberID string
}

func (c WhatsAppClient) post(ctx context.Context, path string, body any) error {
	apiVersion := strings.TrimSpace(c.ApiVersion)
	if apiVersion == "" {
		apiVersion = "v24.0"
	}
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/%s", apiVersion, strings.TrimSpace(c.PhoneNumberID), path)

	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp api error: status=%d body=%s", resp.StatusCode, string(raw))
	}
	return nil
}

// RequestCode requests a verification code via SMS/VOICE.
func (c WhatsAppClient) RequestCode(ctx context.Context, method string, language string) error {
	if strings.TrimSpace(method) == "" {
		method = "SMS"
	}
	if strings.TrimSpace(language) == "" {
		language = "pt_BR"
	}
	return c.post(ctx, "request_code", map[string]any{
		"code_method": method,
		"language":    language,
	})
}

// Register registers the phone number in Cloud API using the PIN received by the user.
func (c WhatsAppClient) Register(ctx context.Context, pin string) error {
	return c.post(ctx, "register", map[string]any{
		"messaging_product": "whatsapp",
		"pin":               pin,
	})
}
