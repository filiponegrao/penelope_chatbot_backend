package workers

import (
	"context"
	"log"
	"os"
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

	replyText, err := tools.GenerateAIReply(ctx, ev.Text)
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
