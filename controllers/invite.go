package controllers

import (
	"net/http"
	"time"

	dbpkg "penelope/db"
	"penelope/models"
	"penelope/tools"

	"github.com/gin-gonic/gin"
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

// ResendActivationCode reenviará (gera outro) código de ativação para o usuário logado.
// A ideia é igual ao Venditto (ResendInvite): procura o invite PENDING do usuário e troca o code.
// Rota sugerida: POST /api/user/resend-code
//
// Obs: como ainda não tem envio por e-mail/SMS, devolvemos o activation_code no payload (para testes).
func ResendActivationCode(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	// Se o usuário já estiver ativo, você pode escolher:
	// - retornar 200 com "already_active"
	// - ou retornar erro (400/409). Aqui mantive 200 pra ficar “suave” em UX.
	if user.Status == models.USER_STATUS_AVAILABLE {
		RespondSuccess(c, gin.H{"status": "already_active"})
		return
	}

	// Busca o invite pendente do usuário
	var invite models.Invite
	if err := db.Where(
		"status = ? AND invited_id = ?",
		models.INVITE_STATUS_PENDING,
		user.ID,
	).First(&invite).Error; err != nil {
		RespondError(c, "nenhum código pendente encontrado", http.StatusNotFound)
		return
	}

	// Gera novo código e renova expiração (mantive 24h igual seu CreateInvite)
	newCode := tools.RandomString(6)
	exp := time.Now().Add(24 * time.Hour)

	invite.Code = newCode
	invite.ExpiresAt = &exp
	invite.Status = models.INVITE_STATUS_PENDING

	tx := db.Begin()
	if err := tx.Save(&invite).Error; err != nil {
		tx.Rollback()
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}

	// Em produção: aqui seria envio por e-mail/SMS/WhatsApp.
	RespondSuccess(c, gin.H{
		"status":          "resent",
		"activation_code": newCode, // <- para testes
	})
}
