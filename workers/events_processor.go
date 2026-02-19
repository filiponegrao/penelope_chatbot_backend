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

var (
	chatHistoryWindowMin int
	chatHistoryMaxEvents int
)

// Config obrigatório: se não estiver setado corretamente, o backend deve falhar rápido.
// (Você pediu "all-in": sem fallback silencioso.)
func init() {
	chatHistoryWindowMin = mustEnvInt("CHAT_HISTORY_WINDOW_MIN")
	chatHistoryMaxEvents = mustEnvInt("CHAT_HISTORY_MAX_EVENTS")
	if chatHistoryWindowMin <= 0 || chatHistoryWindowMin > 24*60 {
		log.Fatalf("CHAT_HISTORY_WINDOW_MIN inválido: %d (esperado 1..1440)", chatHistoryWindowMin)
	}
	if chatHistoryMaxEvents <= 0 || chatHistoryMaxEvents > 50 {
		log.Fatalf("CHAT_HISTORY_MAX_EVENTS inválido: %d (esperado 1..50)", chatHistoryMaxEvents)
	}
}

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
	var hadRagContext bool

	if db != nil && question != "" && ev.UserID > 0 {
		ctxText, err := buildUserInputContext(ctx, db, ev.UserID, question)
		if err != nil {
			if strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG_RAG")), "true") {
				log.Printf("events worker: rag context error: %v", err)
			}
		} else if strings.TrimSpace(ctxText) != "" {
			enrichedText = ctxText
			hadRagContext = true
		}
	}

	if db != nil && strings.TrimSpace(ev.Recipient) != "" && ev.UserID > 0 {
		hist := buildConversationHistory(db, ev.UserID, ev.Recipient, ev.ID)
		if strings.TrimSpace(hist) != "" {
			enrichedText = strings.TrimSpace(hist + "\n\n" + enrichedText)
		}
	}

	if !hadRagContext && looksBusinessSpecific(question) {
		replyText := "Entendi em partes, consegue me explicar com um pouco mais de detalhe? :)"
		finalizeEvent(db, &ev, replyText)
		return
	}

	replyText, err := tools.GenerateAIReply(ctx, enrichedText)
	if err != nil {
		log.Printf("events worker: openai error: %v", err)
		replyText = "Hmmm, vou precisar confirmar aqui no sistema. Consegue voltar em 30 segundos?"
	}

	finalizeEvent(db, &ev, replyText)
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
	if strings.EqualFold(strings.TrimSpace(os.Getenv("RAG_DEBUG")), "1") {
		max := 3
		if len(scored) < max {
			max = len(scored)
		}
		for i := 0; i < max; i++ {
			log.Printf("RAG debug: rank=%d user_input_id=%d score=%.4f", i+1, scored[i].Item.ID, scored[i].Score)
		}
	}

	// Heurísticas simples: topK e threshold para evitar "contexto aleatório".
	// Observação: perguntas curtas (ex.: "Quanto custa o produto?") tendem a ter score menor,
	// então o threshold padrão precisa ser menos agressivo.
	k := 4
	threshold := 0.55
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

	// Fallback pequeno e seguro para perguntas de preço/custo:
	// se nada passou no threshold, mas a pergunta é claramente de preço e o top item parece conter preço,
	// incluímos o top1 (desde que tenha um score mínimo).
	if len(selected) == 0 && len(scored) > 0 {
		q := strings.ToLower(question)
		isPriceQ := strings.Contains(q, "preço") || strings.Contains(q, "preco") || strings.Contains(q, "custo") || strings.Contains(q, "custa") || strings.Contains(q, "valor") || strings.Contains(q, "quanto")
		if isPriceQ {
			top := scored[0]
			ctx := strings.ToLower(strings.TrimSpace(top.Item.Content))
			looksPriceCtx := strings.Contains(ctx, "r$") || strings.Contains(ctx, "reais") || strings.Contains(ctx, "custo") || strings.Contains(ctx, "preço") || strings.Contains(ctx, "preco") || strings.Contains(ctx, "valor")
			if looksPriceCtx && top.Score >= 0.40 {
				selected = append(selected, top)
			}
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

func finalizeEvent(db *gorm.DB, ev *models.Event, replyText string) {
	// Envio WhatsApp:
	// Preferir config multi-tenant (whats_app_configs). Se não existir, usar legacy env.
	sent := false
	if db != nil {
		var wa models.WhatsAppConfig
		if err := db.Where("user_id = ?", ev.UserID).First(&wa).Error; err == nil {
			waClient := tools.WhatsAppClient{
				AccessToken:   wa.AccessToken,
				ApiVersion:    wa.ApiVersion,
				PhoneNumberID: wa.PhoneNumberID,
			}
			if err := waClient.SendText(context.Background(), ev.Recipient, replyText); err != nil {
				log.Printf("events worker: send whatsapp error (tenant): %v", err)
			} else {
				sent = true
			}
		}
	}
	if !sent {
		if err := tools.SendWhatsAppText(context.Background(), ev.Recipient, replyText); err != nil {
			log.Printf("events worker: send whatsapp error (legacy env): %v", err)
		}
	}

	t := time.Now()
	_ = db.Model(&models.Event{}).Where("id = ?", ev.ID).Updates(map[string]any{
		"status":       models.EVENT_STATUS_DONE,
		"processed_at": &t,
		"reply_text":   replyText,
	}).Error
}

// buildConversationHistory monta um histórico curto (user/assistant) para dar continuidade.
// Pega as últimas interações DONE do mesmo (user_id + recipient) dentro de um intervalo de tempo.
func buildConversationHistory(db *gorm.DB, userID int64, recipient string, currentEventID int64) string {
	since := time.Now().Add(-time.Duration(chatHistoryWindowMin) * time.Minute)

	var events []models.Event
	q := db.
		Where("user_id = ? AND recipient = ?", userID, recipient).
		Where("status = ?", models.EVENT_STATUS_DONE).
		Where("processed_at IS NOT NULL AND processed_at >= ?", since).
		Order("processed_at desc, id desc").
		Limit(chatHistoryMaxEvents)

	if currentEventID > 0 {
		q = q.Where("id <> ?", currentEventID)
	}

	if err := q.Find(&events).Error; err != nil {
		return ""
	}
	if len(events) == 0 {
		return ""
	}

	// Está em ordem desc; invertendo para ficar cronológico.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	var b strings.Builder
	b.WriteString("Histórico recente desta conversa (WhatsApp):\n")
	for _, e := range events {
		ut := strings.TrimSpace(e.Text)
		at := strings.TrimSpace(e.ReplyText)
		if ut != "" {
			b.WriteString("- Usuário: ")
			b.WriteString(limitText(ut, 800))
			b.WriteString("\n")
		}
		if at != "" {
			b.WriteString("- Assistente: ")
			b.WriteString(limitText(at, 800))
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func limitText(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func looksBusinessSpecific(question string) bool {
	q := strings.ToLower(strings.TrimSpace(question))
	if q == "" {
		return false
	}
	keywords := []string{
		"penelope", "penélope", "chatbot", "plano", "planos", "bot", "chat bot",
		"preço", "preco", "custo", "custa", "valor", "mensal", "mensalidade", "assinatura",
	}
	for _, k := range keywords {
		if strings.Contains(q, k) {
			return true
		}
	}
	return false
}

func mustEnvInt(key string) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("%s não definido no ambiente", key)
	}
	n, err := atoiSafe(v)
	if err != nil {
		log.Fatalf("%s inválido: %q", key, v)
	}
	return n
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
