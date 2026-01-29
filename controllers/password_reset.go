package controllers

import (
	"context"
	"log"
	"strings"
	"time"

	dbpkg "penelope/db"
	"penelope/models"
	"penelope/tools"

	"github.com/gin-gonic/gin"
)

// POST /api/password/forgot (public)
// Body: { "email": "...", "channel": "whatsapp|sms|email" }
// Retorna sempre true (anti enumeração).
func ForgotPasswordSendCode(c *gin.Context) {
	type Request struct {
		Email   string `json:"email" form:"email"`
		Channel string `json:"channel" form:"channel"`
	}

	var req Request
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Email) == "" {
		// anti-enumeração: sempre true
		RespondSuccess(c, true)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		// ainda assim, anti-enumeração
		RespondSuccess(c, true)
		return
	}

	channel := strings.ToLower(strings.TrimSpace(req.Channel))
	if channel == "" {
		channel = "whatsapp"
	}

	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// anti-enumeração: sempre true
		RespondSuccess(c, true)
		return
	}

	// Mantém 1 token ativo por usuário (opcional, mas ajuda)
	_ = db.Where("user_id = ? AND used_at IS NULL", user.ID).Delete(&models.PasswordReset{}).Error

	// Token numérico (6 dígitos)
	tokenText := tools.RandomNumbers(6)
	tokenHash := tools.EncryptTextSHA512(tokenText)

	exp := time.Now().Add(15 * time.Minute)
	reset := models.PasswordReset{
		UserID:    user.ID,
		TokenHash: tokenHash,
		Channel:   channel,
		ExpiresAt: &exp,
	}

	if err := db.Create(&reset).Error; err != nil {
		// anti-enumeração: sempre true
		RespondSuccess(c, true)
		return
	}

	// Logística preparada pros 3 canais
	msg := "Seu código de recuperação é: " + tokenText

	switch channel {
	case "whatsapp":
		// best-effort; se não tiver credenciais ainda, não quebra o fluxo
		to := strings.TrimSpace(user.Phone1)
		if to != "" {
			// _, _ = tools.SendWhatsAppText(requestCtx(c), to, msg)
			log.Println(msg)
		}
	case "sms":
		// TODO: integrar SMS (Twilio, Zenvia, etc.)
	case "email":
		// TODO: integrar e-mail (SMTP/SES/etc.)
	default:
		// fallback: não faz nada
	}

	RespondSuccess(c, true)
}

// POST /api/password/check-token (public)
// Body: { "email": "...", "token": "123456" }
// Retorna true/false (não consome o token).
func CheckResetToken(c *gin.Context) {
	type Request struct {
		Email string `json:"email" form:"email"`
		Token string `json:"token" form:"token"`
	}

	var req Request
	if err := c.Bind(&req); err != nil {
		RespondSuccess(c, false)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Token = strings.TrimSpace(req.Token)
	if req.Email == "" || req.Token == "" {
		RespondSuccess(c, false)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondSuccess(c, false)
		return
	}

	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		RespondSuccess(c, false)
		return
	}

	tokenHash := tools.EncryptTextSHA512(req.Token)

	var reset models.PasswordReset
	err := db.
		Where("user_id = ? AND token_hash = ? AND used_at IS NULL AND expires_at > ?", user.ID, tokenHash, time.Now()).
		Order("id desc").
		First(&reset).Error
	if err != nil {
		RespondSuccess(c, false)
		return
	}

	RespondSuccess(c, true)
}

// POST /api/password/reset (public)
// Body: { "email": "...", "token": "123456", "new_password": "..." }
// Retorna true/false. Consome o token e revoga refresh tokens.
func ResetPassword(c *gin.Context) {
	type Request struct {
		Email       string `json:"email" form:"email"`
		Token       string `json:"token" form:"token"`
		NewPassword string `json:"new_password" form:"new_password"`
	}

	var req Request
	if err := c.Bind(&req); err != nil {
		RespondSuccess(c, false)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Token = strings.TrimSpace(req.Token)
	req.NewPassword = strings.TrimSpace(req.NewPassword)

	if req.Email == "" || req.Token == "" || req.NewPassword == "" {
		RespondSuccess(c, false)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondSuccess(c, false)
		return
	}

	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		RespondSuccess(c, false)
		return
	}

	tokenHash := tools.EncryptTextSHA512(req.Token)

	var reset models.PasswordReset
	err := db.
		Where("user_id = ? AND token_hash = ? AND used_at IS NULL AND expires_at > ?", user.ID, tokenHash, time.Now()).
		Order("id desc").
		First(&reset).Error
	if err != nil {
		RespondSuccess(c, false)
		return
	}

	// Atualiza senha NO MESMO PADRÃO DO PROJETO (sha512 + email salt)
	passwordEncode := tools.EncryptTextSHA512(req.NewPassword)
	passwordEncode = user.Email + ":" + passwordEncode
	passwordEncode = tools.EncryptTextSHA512(passwordEncode)

	tx := db.Begin()

	if err := tx.Model(&user).Update("password", passwordEncode).Error; err != nil {
		tx.Rollback()
		RespondSuccess(c, false)
		return
	}

	now := time.Now()
	if err := tx.Model(&reset).Update("used_at", &now).Error; err != nil {
		tx.Rollback()
		RespondSuccess(c, false)
		return
	}

	// Revoga refresh tokens do usuário (força novo login)
	if err := tx.Where("user_id = ?", user.ID).Delete(&models.RefreshToken{}).Error; err != nil {
		tx.Rollback()
		RespondSuccess(c, false)
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		RespondSuccess(c, false)
		return
	}

	RespondSuccess(c, true)
}

func requestCtx(c *gin.Context) context.Context {
	if c != nil && c.Request != nil {
		return c.Request.Context()
	}
	return context.Background()
}
