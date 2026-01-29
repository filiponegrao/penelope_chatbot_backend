package models

import "time"

// Plan representa um plano comercial que habilita um conjunto de módulos.
type Plan struct {
	ID          int64  `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	Name        string `gorm:"not null;unique" json:"name" form:"name"`
	Description string `gorm:"type:text" json:"description" form:"description"`
	PriceCents  int64  `gorm:"not null;default:0" json:"price_cents" form:"price_cents"`

	// MonthlyMessageLimit define quantas mensagens/processamentos o usuário pode consumir por mês neste plano.
	// 0 significa "sem limite" (ou limite não configurado).
	MonthlyMessageLimit int64 `gorm:"not null;default:0" json:"monthly_message_limit" form:"monthly_message_limit"`

	Currency  string     `gorm:"not null;default:'BRL'" json:"currency" form:"currency"`
	Interval  string     `gorm:"not null;default:'monthly'" json:"interval" form:"interval"` // monthly|yearly|one_time
	IsActive  bool       `gorm:"not null;default:true" json:"is_active" form:"is_active"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}
