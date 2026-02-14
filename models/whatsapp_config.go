package models

import "time"

const (
	WHATSAPP_STATUS_PENDING    = "pending"
	WHATSAPP_STATUS_REGISTERED = "registered"
)

// WhatsAppConfig stores tenant-specific WhatsApp Cloud API credentials.
// One row per user (multi-tenant).
type WhatsAppConfig struct {
	ID            int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	UserID        int64      `gorm:"not null;unique_index" json:"user_id"`
	WabaID        string     `gorm:"column:waba_id" json:"waba_id"`
	PhoneNumberID string     `gorm:"column:phone_number_id;not null" json:"phone_number_id"`
	AccessToken   string     `gorm:"column:access_token;not null" json:"access_token"`
	ApiVersion    string     `gorm:"column:api_version;not null;default:'v24.0'" json:"api_version"`
	Status        string     `gorm:"column:status;not null;default:'pending'" json:"status"`
	CreatedAt     *time.Time `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at"`
}
