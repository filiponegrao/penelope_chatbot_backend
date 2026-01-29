package controllers

import (
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

	verifyToken := os.Getenv("WEBHOOK_VERIFY_TOKEN")
	if verifyToken == "" {
		RespondError(c, "WEBHOOK_VERIFY_TOKEN not set", http.StatusInternalServerError)
		return
	}

	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == verifyToken {
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

	var payload WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
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
