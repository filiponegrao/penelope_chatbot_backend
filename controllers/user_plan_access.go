package controllers

import (
	"penelope/models"

	"github.com/jinzhu/gorm"
)

// getUserPlanID retorna o plan_id do usuário via tabela user_plans.
// Se não existir vínculo, retorna (nil, nil).
func getUserPlanID(db *gorm.DB, userID int64) (*int64, error) {
	var link models.UserPlan
	if err := db.Where("user_id = ?", userID).First(&link).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return &link.PlanID, nil
}

// getUserPlanID retorna o plan_id do usuário via tabela user_plans.
// Se não existir vínculo, retorna (nil, nil).
func getUserPlans(db *gorm.DB, userID int64) ([]models.UserPlan, error) {
	var links []models.UserPlan
	if err := db.Where("user_id = ?", userID).Find(&links).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return links, nil
}
