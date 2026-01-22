package controllers

import (
	"time"

	"penelope/models"

	"github.com/jinzhu/gorm"
)

// Mantém assinatura do Venditto que seu CreateUser está chamando:
// CreateInvite(c, tx, code, user, "")
func CreateInvite(_ any, tx *gorm.DB, code string, user models.User, _ string) (*models.Invite, error) {
	exp := time.Now().Add(24 * time.Hour)

	invite := models.Invite{
		InviterID: user.ID,
		InvitedID: user.ID,
		Code:      code,
		Status:    models.INVITE_STATUS_PENDING,
		ExpiresAt: &exp,
	}

	if err := tx.Create(&invite).Error; err != nil {
		return nil, err
	}
	return &invite, nil
}
