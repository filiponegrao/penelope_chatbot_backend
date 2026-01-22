package controllers

import (
	"net/http"
	"time"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

// ActivateUserByCode valida um código de ativação (invite) e libera o usuário.
// Rota sugerida: POST /api/user/activate/:code
func ActivateUserByCode(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		RespondError(c, "code é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var invite models.Invite
	if err := db.Where("code = ?", code).First(&invite).Error; err != nil {
		RespondError(c, "código inválido", http.StatusNotFound)
		return
	}

	// Expiração
	if invite.ExpiresAt != nil && time.Now().After(*invite.ExpiresAt) {
		_ = db.Model(&invite).Update("status", models.INVITE_STATUS_EXPIRED).Error
		RespondError(c, "código expirado", http.StatusForbidden)
		return
	}
	if invite.Status == models.INVITE_STATUS_VALIDATED {
		RespondSuccess(c, gin.H{"status": "already_validated"})
		return
	}

	// Sobe usuário
	var user models.User
	if err := db.Where("id = ?", invite.InvitedID).First(&user).Error; err != nil {
		RespondError(c, "usuário não encontrado", http.StatusNotFound)
		return
	}

	tx := db.Begin()
	if err := tx.Model(&invite).Update("status", models.INVITE_STATUS_VALIDATED).Error; err != nil {
		tx.Rollback()
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if err := tx.Model(&user).Update("status", models.USER_STATUS_AVAILABLE).Error; err != nil {
		tx.Rollback()
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	user.Password = ""
	RespondSuccess(c, gin.H{"status": "activated", "user": user})
}
