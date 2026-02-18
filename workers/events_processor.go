package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"penelope/models"
	"penelope/tools"

	"github.com/jinzhu/gorm"
)

// StartEventProcessor starts a loop that processes pending events whose ScheduledAt <= now.
func StartEventProcessor(db *gorm.DB) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			processDueEvents(db)
		}
	}()
}

func processDueEvents(db *gorm.DB) {
	now := time.Now()

	var events []models.Event
	if err := db.
		Where("status = ?", models.EVENT_STATUS_PENDING).
		Where("scheduled_at IS NOT NULL AND scheduled_at <= ?", now).
		Order("scheduled_at asc, id asc").
		Limit(50).
		Find(&events).Error; err != nil {
		log.Printf("events worker: query error: %v", err)
		return
	}

	for _, ev := range events {
		// lock otimista: só processa se conseguir mudar status
		res := db.Model(&models.Event{}).
			Where("id = ? AND status = ?", ev.ID, models.EVENT_STATUS_PENDING).
			Update("status", models.EVENT_STATUS_PROCESSING)
		if res.Error != nil || res.RowsAffected == 0 {
			continue
		}

		go handleEvent(db, ev.ID)
	}
}

func handleEvent(db *gorm.DB, eventID int64) {
	var ev models.Event
	if err := db.First(&ev, eventID).Error; err != nil {
		return
	}
	if ev.Status != models.EVENT_STATUS_PROCESSING {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1) Recupera contextos (UserInputs) mais similares à pergunta para enriquecer o prompt.
	//    Se falhar por qualquer motivo (ex.: embeddings off), seguimos sem contexto.
	question := strings.TrimSpace(ev.Text)
	enrichedText := question
	if db != nil && question != "" && ev.UserID > 0 {
		ctxText, err := buildUserInputContext(ctx, db, ev.UserID, question)
		if err != nil {
			if strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG_RAG")), "true") {
				log.Printf("events worker: rag context error: %v", err)
			}
		} else if strings.TrimSpace(ctxText) != "" {
			enrichedText = ctxText
		}
	}

	replyText, err := tools.GenerateAIReply(ctx, enrichedText)
	if err != nil {
		log.Printf("events worker: openai error: %v", err)
		replyText = "Desculpe, tive um problema ao gerar a resposta."
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("POC_NO_WHATSAPP")), "true") {
		t := time.Now()
		_ = db.Model(&models.Event{}).Where("id = ?", ev.ID).Updates(map[string]any{
			"status":       models.EVENT_STATUS_DONE,
			"processed_at": &t,
			"reply_text":   replyText,
		}).Error
		return
	}

	if err := tools.SendWhatsAppText(ctx, ev.Recipient, replyText); err != nil {
		log.Printf("events worker: send whatsapp error (legacy env): %v", err)
	}

	// Multi-tenant send (preferred): uses WhatsAppConfig for ev.UserID.
	// If config is missing, we keep legacy env behavior above to avoid breaking older setups.
	if db != nil {
		var wa models.WhatsAppConfig
		if err := db.Where("user_id = ?", ev.UserID).First(&wa).Error; err == nil {
			waClient := tools.WhatsAppClient{
				AccessToken:   wa.AccessToken,
				ApiVersion:    wa.ApiVersion,
				PhoneNumberID: wa.PhoneNumberID,
			}
			if err := waClient.SendText(ctx, ev.Recipient, replyText); err != nil {
				log.Printf("events worker: send whatsapp error (tenant): %v", err)
			}
		}
	}

	t := time.Now()
	_ = db.Model(&models.Event{}).Where("id = ?", ev.ID).Updates(map[string]any{
		"status":       models.EVENT_STATUS_DONE,
		"processed_at": &t,
		"reply_text":   replyText,
	}).Error
}

type scoredUserInput struct {
	Item  models.UserInput
	Score float64
}

// buildUserInputContext busca os UserInputs do usuário, escolhe os mais próximos da pergunta via cosine similarity
// e devolve um texto "enriquecido" para mandar ao OpenAI.
func buildUserInputContext(ctx context.Context, db *gorm.DB, userID int64, question string) (string, error) {
	// Embedding da pergunta
	qEmbStr, err := tools.EmbedText(ctx, question)
	if err != nil {
		return "", fmt.Errorf("embed question: %w", err)
	}
	qEmb, err := parseEmbedding(qEmbStr)
	if err != nil {
		return "", fmt.Errorf("parse question embedding: %w", err)
	}

	// Carrega user inputs com embedding
	var items []models.UserInput
	if err := db.
		Where("user_id = ? AND embedding IS NOT NULL AND embedding != ''", userID).
		Find(&items).Error; err != nil {
		return "", fmt.Errorf("load user_inputs: %w", err)
	}
	if len(items) == 0 {
		return question, nil
	}

	// Score por similaridade
	scored := make([]scoredUserInput, 0, len(items))
	for _, it := range items {
		emb, err := parseEmbedding(it.Embedding)
		if err != nil {
			continue
		}
		s, ok := cosineSimilarity(qEmb, emb)
		if !ok {
			continue
		}
		scored = append(scored, scoredUserInput{Item: it, Score: s})
	}

	if len(scored) == 0 {
		return question, nil
	}

	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })

	// Heurísticas simples: topK e threshold para evitar "contexto aleatório".
	k := 4
	threshold := 0.75
	if v := strings.TrimSpace(os.Getenv("RAG_TOP_K")); v != "" {
		if n, err := atoiSafe(v); err == nil && n > 0 && n <= 20 {
			k = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("RAG_MIN_SCORE")); v != "" {
		if f, err := atofSafe(v); err == nil && f >= -1 && f <= 1 {
			threshold = f
		}
	}

	selected := make([]scoredUserInput, 0, k)
	for _, s := range scored {
		if len(selected) >= k {
			break
		}
		if s.Score >= threshold {
			selected = append(selected, s)
		}
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG_RAG")), "true") {
		log.Printf("events worker: rag candidates=%d selected=%d threshold=%.2f", len(scored), len(selected), threshold)
		for i := 0; i < len(scored) && i < 5; i++ {
			log.Printf("events worker: rag top%d score=%.4f input_id=%d user_input_id=%d", i+1, scored[i].Score, scored[i].Item.InputID, scored[i].Item.ID)
		}
	}

	if len(selected) == 0 {
		return question, nil
	}

	// Monta prompt enriquecido (bem explícito e curto)
	var b strings.Builder
	b.WriteString("Use as informações abaixo como contexto quando forem relevantes.\n")
	b.WriteString("Se alguma informação parecer não relacionada à pergunta, ignore.\n\n")
	b.WriteString("Contexto (anotações do usuário):\n")
	for _, s := range selected {
		c := strings.TrimSpace(s.Item.Content)
		if c == "" {
			continue
		}
		// limitador simples para evitar explodir tokens
		if len(c) > 600 {
			c = c[:600] + "..."
		}
		b.WriteString("- ")
		b.WriteString(c)
		b.WriteString("\n")
	}
	b.WriteString("\nPergunta do usuário:\n")
	b.WriteString(question)

	return b.String(), nil
}

func parseEmbedding(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty embedding string")
	}
	var arr []float64
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil, err
	}
	// remove NaNs/Infs
	out := make([]float64, 0, len(arr))
	for _, v := range arr {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("invalid embedding value")
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty embedding array")
	}
	return out, nil
}

func cosineSimilarity(a, b []float64) (float64, bool) {
	if len(a) == 0 || len(b) == 0 {
		return 0, false
	}
	// só computa até o menor tamanho (defensivo)
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0, false
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb)), true
}

func atoiSafe(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func atofSafe(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
	if err != nil {
		return 0, err
	}
	return f, nil
}
