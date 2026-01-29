package models

import "time"

// PlanModule liga módulos a planos (N:N).
type PlanModule struct {
	ID        int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	PlanID    int64      `gorm:"not null;index;unique_index:ux_plan_module" json:"plan_id"`
	ModuleID  int64      `gorm:"not null;index;unique_index:ux_plan_module" json:"module_id"`
	CreatedAt *time.Time `json:"created_at"`
}
