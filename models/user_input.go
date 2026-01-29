package models

import "time"

// UserInput armazena o conteúdo fornecido pelo usuário para um determinado Input.
// Regra: um usuário só pode ter 1 UserInput por Input (unique(user_id, input_id)).
type UserInput struct {
	ID        int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	UserID    int64      `gorm:"not null;index;unique_index:ux_user_input" json:"user_id"`
	InputID   int64      `gorm:"not null;index;unique_index:ux_user_input" json:"input_id"`
	Content   string     `gorm:"type:text" json:"content" form:"content"`
	Embedding string     `gorm:"type:text" json:"embedding"` // JSON array (ex: [0.1,0.2,...])
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}
