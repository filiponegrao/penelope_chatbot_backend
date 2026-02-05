package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// SendWhatsAppText sends a text message via WhatsApp Cloud API using ENV (legacy).
// The multi-tenant worker uses WhatsAppClient.SendText instead.
func SendWhatsAppText(ctx context.Context, to string, text string) error {
	token := strings.TrimSpace(os.Getenv("WHATSAPP_ACCESS_TOKEN"))
	phoneID := strings.TrimSpace(os.Getenv("WHATSAPP_PHONE_NUMBER_ID"))
	if token == "" || phoneID == "" {
		return fmt.Errorf("WHATSAPP_ACCESS_TOKEN or WHATSAPP_PHONE_NUMBER_ID not set")
	}

	client := WhatsAppClient{
		AccessToken:   token,
		ApiVersion:    "v20.0",
		PhoneNumberID: phoneID,
	}
	return client.SendText(ctx, to, text)
}

// SendText sends a text message via WhatsApp Cloud API using a tenant-aware client.
func (c WhatsAppClient) SendText(ctx context.Context, to string, text string) error {
	if strings.TrimSpace(c.AccessToken) == "" || strings.TrimSpace(c.PhoneNumberID) == "" {
		return fmt.Errorf("whatsapp client missing access_token or phone_number_id")
	}
	apiVersion := strings.TrimSpace(c.ApiVersion)
	if apiVersion == "" {
		apiVersion = "v24.0"
	}

	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", apiVersion, strings.TrimSpace(c.PhoneNumberID))

	reqBody := map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text": map[string]any{
			"body": text,
		},
	}

	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp api error: status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}
