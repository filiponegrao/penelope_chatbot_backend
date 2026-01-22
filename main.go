package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// =====================
// ENV esperadas
// =====================
//
// Server
// - PORT                          (ex: 8080)
// - WEBHOOK_VERIFY_TOKEN          (string que você configura no painel do WhatsApp para verificação do webhook)
// - WEBHOOK_SECRET                (App Secret para validar X-Hub-Signature-256)  [opcional mas recomendado]
//
// WhatsApp Cloud API (Meta)
// - WHATSAPP_ACCESS_TOKEN         (token permanente/sistema)
// - WHATSAPP_PHONE_NUMBER_ID      (ID do telefone para envio de mensagens)
// - POC_NO_WHATSAPP 			   (se true, não envia pro WhatsApp e devolve no HTTP)
//
// OpenAI
// - OPENAI_API_KEY
// - OPENAI_MODEL                  (ex: gpt-4.1-mini ou outro que você quiser)
// - OPENAI_SYSTEM_PROMPT          (opcional)
//
// =====================

func main() {
	port := getenv("PORT", "8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/webhook", webhookHandler)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Penelope listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleWebhookVerify(w, r)
		return
	case http.MethodPost:
		handleWebhookUpdate(w, r)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

// =====================
// 1) VERIFICAÇÃO DO WEBHOOK (GET)
// =====================
//
// O WhatsApp (Meta) chama algo como:
// GET /webhook?hub.mode=subscribe&hub.verify_token=...&hub.challenge=...
//
func handleWebhookVerify(w http.ResponseWriter, r *http.Request) {
	verifyToken := os.Getenv("WEBHOOK_VERIFY_TOKEN")
	if verifyToken == "" {
		http.Error(w, "WEBHOOK_VERIFY_TOKEN not set", http.StatusInternalServerError)
		return
	}

	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && subtle.ConstantTimeCompare([]byte(token), []byte(verifyToken)) == 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge))
		return
	}

	http.Error(w, "forbidden", http.StatusForbidden)
}

// =====================
// 2) RECEBENDO UPDATE (POST)
// =====================
func handleWebhookUpdate(w http.ResponseWriter, r *http.Request) {
	// (Opcional, mas recomendado) validar assinatura do Meta
	if secret := os.Getenv("WEBHOOK_SECRET"); secret != "" {
		bodyBytes, err := readAndRestoreBody(r)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		if !validateMetaSignature(r, bodyBytes, secret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Extrai mensagens texto
	msgs := extractTextMessages(payload)

	// Se estamos em POC sem WhatsApp, devolvemos a resposta no HTTP
	if strings.EqualFold(getenv("POC_NO_WHATSAPP", "false"), "true") {
		type item struct {
			From  string `json:"from"`
			ID    string `json:"id"`
			Text  string `json:"text"`
			Reply string `json:"reply"`
		}
		out := make([]item, 0, len(msgs))

		for _, m := range msgs {
			log.Printf("Incoming message: from=%s text=%q msg_id=%s", m.From, m.Text, m.ID)

			replyText, err := generateAIReply(r.Context(), m.Text)
			if err != nil {
				log.Printf("OpenAI error: %v", err)
				replyText = "Erro ao gerar resposta (OpenAI)."
			}

			log.Printf("AI reply: to=%s msg_id=%s reply=%q", m.From, m.ID, replyText)

			out = append(out, item{
				From:  m.From,
				ID:    m.ID,
				Text:  m.Text,
				Reply: replyText,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode":     "POC_NO_WHATSAPP",
			"messages": out,
		})
		return
	}

	// Modo normal (WhatsApp ativo): responde rápido e processa
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("EVENT_RECEIVED"))

	for _, m := range msgs {
		log.Printf("Incoming message: from=%s text=%q msg_id=%s", m.From, m.Text, m.ID)

		replyText, err := generateAIReply(r.Context(), m.Text)
		if err != nil {
			log.Printf("OpenAI error: %v (fallback to echo)", err)
			replyText = "Você disse: " + m.Text
		}

		if err := sendWhatsAppText(r.Context(), m.From, replyText); err != nil {
			log.Printf("Send WhatsApp error: %v", err)
		}
	}
}


// =====================
// MODELOS (Webhook)
// =====================

// Estrutura mínima para capturar mensagens texto
type WebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				MessagingProduct string `json:"messaging_product"`
				Metadata         struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberID      string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WaID string `json:"wa_id"`
				} `json:"contacts"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Text      *struct {
						Body string `json:"body"`
					} `json:"text,omitempty"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

type IncomingTextMessage struct {
	From string
	ID   string
	Text string
}

func extractTextMessages(p WebhookPayload) []IncomingTextMessage {
	var out []IncomingTextMessage
	for _, e := range p.Entry {
		for _, c := range e.Changes {
			for _, msg := range c.Value.Messages {
				if strings.ToLower(msg.Type) == "text" && msg.Text != nil {
					out = append(out, IncomingTextMessage{
						From: msg.From,
						ID:   msg.ID,
						Text: msg.Text.Body,
					})
				}
			}
		}
	}
	return out
}

// =====================
// ENVIO DE MENSAGEM (WhatsApp Cloud API)
// =====================
func sendWhatsAppText(ctx context.Context, to string, text string) error {
	token := os.Getenv("WHATSAPP_ACCESS_TOKEN")
	phoneID := os.Getenv("WHATSAPP_PHONE_NUMBER_ID")
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

	resp, err := http.DefaultClient.Do(req)
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

// =====================
// OpenAI (Responses API via openai-go)
// =====================
func generateAIReply(ctx context.Context, userText string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	model := getenv("OPENAI_MODEL", "gpt-4.1-mini")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	systemPrompt := getenv(
		"OPENAI_SYSTEM_PROMPT",
		"Você é a Penélope, um chatbot útil, educado e direto. Responda em português do Brasil.",
	)

	// Request no formato recomendado: instructions + input(string)
	reqBody := map[string]any{
		"model":        model,
		"instructions": systemPrompt,
		"input":        userText,
	}

	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.openai.com/v1/responses",
		bytes.NewReader(b),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// (Opcional, mas bom) cliente com timeout
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai error %d: %s", resp.StatusCode, string(body))
	}

	// Parse do "output" (no REST é aqui que vem o texto).
	var parsed struct {
		Output []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, item := range parsed.Output {
		if item.Type == "message" && item.Role == "assistant" {
			for _, c := range item.Content {
				if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
					if sb.Len() > 0 {
						sb.WriteString("\n")
					}
					sb.WriteString(c.Text)
				}
			}
		}
	}

	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", fmt.Errorf("empty response from model (no output_text items found)")
	}

	return out, nil
}

// =====================
// Assinatura do Meta (opcional)
// =====================

func readAndRestoreBody(r *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return bodyBytes, nil
}

func validateMetaSignature(r *http.Request, body []byte, secret string) bool {
	// Implementação completa (HMAC SHA256) eu deixo pronta na próxima etapa se você quiser,
	// porque você pode testar primeiro sem isso (setando WEBHOOK_SECRET vazio).
	//
	// Header: X-Hub-Signature-256: sha256=<hex>
	_ = r
	_ = body
	_ = secret
	return true
}

// =====================
// Helpers
// =====================
func getenv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}
