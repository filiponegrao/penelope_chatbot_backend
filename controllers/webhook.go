package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

type WebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				Messages []struct {
					From string `json:"from"`
					ID   string `json:"id"`
					Type string `json:"type"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
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

func extractTextMessages(payload WebhookPayload) []IncomingTextMessage {
	var out []IncomingTextMessage

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if strings.TrimSpace(change.Field) != "messages" {
				continue
			}
			for _, m := range change.Value.Messages {
				if strings.ToLower(strings.TrimSpace(m.Type)) != "text" {
					continue
				}
				body := strings.TrimSpace(m.Text.Body)
				if body == "" {
					continue
				}
				out = append(out, IncomingTextMessage{
					From: strings.TrimSpace(m.From),
					ID:   strings.TrimSpace(m.ID),
					Text: body,
				})
			}
		}
	}

	return out
}

func resolveWebhookUserID(c *gin.Context) (int64, error) {
	// /webhook/:userId
	param := strings.TrimSpace(c.Param("userId"))
	if param != "" {
		return strconv.ParseInt(param, 10, 64)
	}

	// fallback para dev (mantém /webhook funcionando localmente)
	def := strings.TrimSpace(os.Getenv("WEBHOOK_DEFAULT_USER_ID"))
	if def == "" {
		return 0, fmt.Errorf("missing userId param and WEBHOOK_DEFAULT_USER_ID not set")
	}
	return strconv.ParseInt(def, 10, 64)
}

// verifyMetaSignature validates the request body against Meta's signature header.
//
// WhatsApp/Graph Webhooks typically send: X-Hub-Signature-256: sha256=<hex>
// The secret should be your Meta App Secret (NOT the WhatsApp access token).
func verifyMetaSignature(c *gin.Context, rawBody []byte) (bool, string) {
	// Prefer a dedicated env var for webhook signature secret.
	// Keep multiple names for ops convenience.
	secret := strings.TrimSpace(os.Getenv("WEBHOOK_APP_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("WHATSAPP_APP_SECRET"))
	}
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("META_APP_SECRET"))
	}
	if secret == "" {
		return false, "missing WEBHOOK_APP_SECRET/WHATSAPP_APP_SECRET/META_APP_SECRET"
	}

	sig := strings.TrimSpace(c.GetHeader("X-Hub-Signature-256"))
	if sig == "" {
		// Some products also send X-Hub-Signature (sha1), but we enforce sha256.
		return false, "missing X-Hub-Signature-256"
	}
	if !strings.HasPrefix(sig, "sha256=") {
		return false, "invalid X-Hub-Signature-256 format"
	}

	providedHex := strings.TrimPrefix(sig, "sha256=")
	provided, err := hex.DecodeString(providedHex)
	if err != nil {
		return false, "invalid signature hex"
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(rawBody)
	expected := mac.Sum(nil)

	if !hmac.Equal(provided, expected) {
		return false, "signature mismatch"
	}

	return true, ""
}

func requireActiveUserByID(c *gin.Context, db *gorm.DB, userID int64) (*models.User, bool) {
	if userID <= 0 {
		RespondError(c, "user_id inválido", http.StatusBadRequest)
		return nil, false
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		RespondError(c, "usuário não encontrado", http.StatusNotFound)
		return nil, false
	}

	// Ajuste se seu status ativo tiver outro nome
	if user.Status != models.USER_STATUS_AVAILABLE {
		RespondError(c, "usuário não está ativo", http.StatusForbidden)
		return nil, false
	}

	return &user, true
}

// GET /webhook e GET /webhook/:userId
func WebhookVerify(c *gin.Context) {
	verifyToken := os.Getenv("WEBHOOK_VERIFY_TOKEN")
	if verifyToken == "" {
		RespondError(c, "WEBHOOK_VERIFY_TOKEN not set", http.StatusInternalServerError)
		return
	}

	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	fmt.Printf("[WA][VERIFY] path=%s mode=%s token_ok=%v challenge=%s\n",
		c.FullPath(), mode, token == verifyToken, challenge)

	if mode == "subscribe" && token == verifyToken && challenge != "" {
		c.String(http.StatusOK, "%s", challenge)
		return
	}

	RespondError(c, "forbidden", http.StatusForbidden)
}

// POST /webhook e POST /webhook/:userId
func WebhookUpdate(c *gin.Context) {
	userID, err := resolveWebhookUserID(c)
	if err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	_, ok := requireActiveUserByID(c, db, userID)
	if !ok {
		return
	}

	// Read raw body once so we can validate Meta signature.
	raw, err := c.GetRawData()
	if err != nil {
		RespondError(c, "failed to read body", http.StatusBadRequest)
		return
	}

	if ok, reason := verifyMetaSignature(c, raw); !ok {
		RespondError(c, "forbidden: "+reason, http.StatusForbidden)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		RespondError(c, "invalid json", http.StatusBadRequest)
		return
	}

	msgs := extractTextMessages(payload)

	// responde rápido pro Meta
	c.String(http.StatusOK, "EVENT_RECEIVED")

	for _, m := range msgs {
		_ = upsertDebouncedEvent(db, userID, m.From, m.ID, m.Text)
	}
}

// Debounce por (user_id + recipient)
func upsertDebouncedEvent(db *gorm.DB, userID int64, recipient string, messageID string, text string) error {
	now := time.Now()
	scheduled := now.Add(3 * time.Second)

	tx := db.Begin()

	var last models.Event
	err := tx.
		Where("user_id = ? AND recipient = ? AND status = ?", userID, recipient, models.EVENT_STATUS_PENDING).
		Where("scheduled_at IS NOT NULL AND scheduled_at > ?", now).
		Order("id desc").
		First(&last).Error

	combinedText := text
	if err == nil && last.ID > 0 {
		t := time.Now()
		_ = tx.Model(&models.Event{}).Where("id = ?", last.ID).Updates(map[string]any{
			"status":         models.EVENT_STATUS_INVALIDATED,
			"invalidated_at": &t,
		}).Error

		if strings.TrimSpace(last.Text) != "" {
			combinedText = strings.TrimSpace(last.Text) + "\n" + strings.TrimSpace(text)
		}
	}

	ev := models.Event{
		UserID:      userID,
		Recipient:   recipient,
		MessageID:   messageID,
		Text:        combinedText,
		Status:      models.EVENT_STATUS_PENDING,
		ScheduledAt: &scheduled,
	}

	if err := tx.Create(&ev).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}
