package models

import "time"

// RefreshToken representa um token de refresh persistido.
// Guardamos apenas o hash do token (nunca o token em si) para reduzir impacto em caso de vazamento do DB.
type RefreshToken struct {
	ID        int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	UserID    int64      `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"not null;unique_index" json:"-"`
	RevokedAt *time.Time `json:"revoked_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func (rt RefreshToken) IsRevoked() bool {
	return rt.RevokedAt != nil
}

func (rt RefreshToken) IsExpired(now time.Time) bool {
	if rt.ExpiresAt == nil {
		return false
	}
	return now.After(*rt.ExpiresAt)
}
