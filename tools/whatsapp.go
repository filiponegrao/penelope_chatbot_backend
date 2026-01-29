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

// SendWhatsAppText sends a text message via WhatsApp Cloud API.
func SendWhatsAppText(ctx context.Context, to string, text string) error {
	token := strings.TrimSpace(os.Getenv("WHATSAPP_ACCESS_TOKEN"))
	phoneID := strings.TrimSpace(os.Getenv("WHATSAPP_PHONE_NUMBER_ID"))
	if token == "" || phoneID == "" {
		return fmt.Errorf("WHATSAPP_ACCESS_TOKEN or WHATSAPP_PHONE_NUMBER_ID not set")
	}

	url := fmt.Sprintf("https://graph.facebook.com/v20.0/%s/messages", phoneID)

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
	req.Header.Set("Authorization", "Bearer "+token)
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
