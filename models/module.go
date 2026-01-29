package models

import "time"

// Module representa um módulo funcional do sistema (Triagem, Catálogo, Suporte, Atendimento Informativo).
type Module struct {
	ID          int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	Key         string     `gorm:"not null;unique" json:"key" form:"key"` // ex: triage, catalog, support, info
	Name        string     `gorm:"not null" json:"name" form:"name"`
	Description string     `gorm:"type:text" json:"description" form:"description"`
	CreatedAt   *time.Time `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}
