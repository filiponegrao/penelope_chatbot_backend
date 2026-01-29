package models

import "time"

// UserPlan representa o vínculo "1 usuário -> 1 plano".
// Regra: user_id é único, garantindo no máximo 1 vínculo por usuário.
type UserPlan struct {
	ID        int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	UserID    int64      `gorm:"not null;unique_index" json:"user_id"`
	PlanID    int64      `gorm:"not null;index" json:"plan_id"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}
