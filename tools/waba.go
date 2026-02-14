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

// WabaClient is a thin client for WABA-level Graph API operations.
// Example: /{waba_id}/subscribed_apps
type WabaClient struct {
	AccessToken string
	ApiVersion  string // e.g. v24.0
	WabaID      string
}

func (c WabaClient) post(ctx context.Context, path string, body any) error {
	apiVersion := strings.TrimSpace(c.ApiVersion)
	if apiVersion == "" {
		apiVersion = "v24.0"
	}
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/%s", apiVersion, strings.TrimSpace(c.WabaID), strings.TrimPrefix(path, "/"))

	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}

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
		return WhatsAppAPIError{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	return nil
}

// SubscribeApp subscribes the current app to receive webhook updates for this WABA.
func (c WabaClient) SubscribeApp(ctx context.Context) error {
	if strings.TrimSpace(c.WabaID) == "" {
		return fmt.Errorf("waba_id é obrigatório")
	}
	return c.post(ctx, "subscribed_apps", nil)
}
