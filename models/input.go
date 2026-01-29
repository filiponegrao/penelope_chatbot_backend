package models

import "time"

// Input representa um "tipo de informação" configurável (cardápio, horário de atendimento etc).
type Input struct {
	ID          int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	Key         string     `gorm:"not null;unique" json:"key" form:"key"`
	Type        string     `gorm:"not null" json:"type" form:"type"` // ex: text, url, json
	Description string     `gorm:"type:text" json:"description" form:"description"`
	CreatedAt   *time.Time `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}
