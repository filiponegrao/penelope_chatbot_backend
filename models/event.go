package models

import "time"

/************************************************
/**** MARK: EVENT STATUS ****/
/************************************************/
const EVENT_STATUS_PENDING = "pending"
const EVENT_STATUS_PROCESSING = "processing"
const EVENT_STATUS_DONE = "done"
const EVENT_STATUS_INVALIDATED = "invalidated"

// Event representa um evento recebido no webhook (mensagem inbound).
// Ele entra como "pending" e é processado após uma janela de debounce (3s) para agregação.
type Event struct {
	ID            int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	UserID        int64      `gorm:"not null;default:0;index" json:"user_id"`
	Recipient     string     `gorm:"not null;index" json:"recipient"` // ex: telefone do remetente (from)
	MessageID     string     `gorm:"default:''" json:"message_id"`
	Text          string     `gorm:"type:text" json:"text"`
	Status        string     `gorm:"not null;default:'pending';index" json:"status"`
	ScheduledAt   *time.Time `gorm:"index" json:"scheduled_at"`
	ProcessedAt   *time.Time `json:"processed_at"`
	InvalidatedAt *time.Time `json:"invalidated_at"`
	ReplyText     string     `gorm:"type:text" json:"reply_text"`
	CreatedAt     *time.Time `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at"`
}
