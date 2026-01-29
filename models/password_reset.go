package models

import "time"

// PasswordReset representa um token temporário para o fluxo de "Esqueci minha senha".
// Guardamos apenas o HASH do token (nunca o token em texto puro).
type PasswordReset struct {
	ID        int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	UserID    int64      `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"not null;index" json:"-"`
	Channel   string     `gorm:"not null;default:'whatsapp'" json:"channel"`
	ExpiresAt *time.Time `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}
