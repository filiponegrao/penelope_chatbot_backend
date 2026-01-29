package models

import "time"

// ModuleInput liga inputs a módulos (N:N).
type ModuleInput struct {
	ID        int64      `gorm:"primary_key;AUTO_INCREMENT" json:"id"`
	ModuleID  int64      `gorm:"not null;index;unique_index:ux_module_input" json:"module_id"`
	InputID   int64      `gorm:"not null;index;unique_index:ux_module_input" json:"input_id"`
	CreatedAt *time.Time `json:"created_at"`
}
