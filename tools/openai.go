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

// GenerateAIReply calls OpenAI Responses API and returns assistant text.
func GenerateAIReply(ctx context.Context, userText string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}
	model := getenv("OPENAI_MODEL", "gpt-4.1-mini")

		systemPrompt := getenv(
		"OPENAI_SYSTEM_PROMPT",
		strings.TrimSpace(`Você é a Penélope, a assistente virtual do sistema/serviço "Penélope Chatbot".

Regras IMPORTANTES:
- O usuário está falando sobre o produto/serviço Penélope Chatbot (um sistema comercial), não sobre o ChatGPT nem sobre "uma IA gratuita da plataforma".
- Se o usuário perguntar sobre preço/custo/planos e essa informação estiver no CONTEXTO fornecido na mensagem, use exatamente o que está no contexto.
- Se NÃO houver informação suficiente no contexto, faça 1 pergunta objetiva para esclarecer (não invente valores, planos ou números).
- Não alucine detalhes (ex.: faixa de preço, plano VIP, ilimitado) se não estiverem explícitos no contexto.
- Se você não tiver contexto suficiente para responder com segurança, peça para o usuário reformular ou dar mais detalhes.
- Responda em português do Brasil, com tom útil, educado e direto.
`),
	)

	// Contexto global opcional (ex.: definição do produto, políticas, preço base, etc.)
	// Você pode setar isso via env OPENAI_GLOBAL_CONTEXT para reduzir ambiguidade sem redeploy.
	if gc := strings.TrimSpace(os.Getenv("OPENAI_GLOBAL_CONTEXT")); gc != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n" + gc)
	}

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

// EmbedText calls OpenAI Embeddings API and returns a JSON string array of floats.
func EmbedText(ctx context.Context, text string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}
	model := getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")

	reqBody := map[string]any{
		"model": model,
		"input": text,
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.openai.com/v1/embeddings",
		bytes.NewReader(b),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai embeddings error %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return "", fmt.Errorf("empty embedding")
	}

	outBytes, _ := json.Marshal(parsed.Data[0].Embedding)
	return string(outBytes), nil
}

func getenv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}
