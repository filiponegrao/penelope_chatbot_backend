package controllers

import (
	"net/http"
	"time"

	dbpkg "penelope/db"
	"penelope/models"
	"penelope/tools"

	"github.com/gin-gonic/gin"
)

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken        string `json:"access_token"`
	AccessExpiresAt    int64  `json:"access_expires_at"`     // unix seconds
	AccessExpiresAtISO string `json:"access_expires_at_iso"` // RFC3339
	RefreshToken       string `json:"refresh_token"`
}

// Refresh troca um refresh token válido por um novo par (access+refresh).
// Regras de segurança:
// - Não armazenamos o token em texto no DB (apenas hash)
// - Rotação: ao usar, revogamos tokens anteriores e emitimos um novo
// - Sessão única: revoga TODOS os refresh tokens ativos do usuário (incluindo o atual)
func Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if req.RefreshToken == "" {
		RespondError(c, "refresh_token é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	hash := tools.EncryptTextSHA512(req.RefreshToken)

	var stored models.RefreshToken
	if err := db.Where("token_hash = ?", hash).First(&stored).Error; err != nil {
		RespondError(c, "refresh token inválido", http.StatusUnauthorized)
		return
	}

	if stored.IsRevoked() || stored.IsExpired(now) {
		RespondError(c, "refresh token expirado", http.StatusUnauthorized)
		return
	}

	// ✅ Sessão única + rotação: revoga todos os refresh tokens ativos deste usuário.
	if err := revokeAllUserRefreshTokens(db, stored.UserID, now); err != nil {
		RespondError(c, "erro ao revogar sessões anteriores", http.StatusInternalServerError)
		return
	}

	secret := getJWTSecret()
	accessTTLMinutes := getenvInt("JWT_ACCESS_TTL_MINUTES", 24*60)
	accessExp := now.Add(time.Duration(accessTTLMinutes) * time.Minute)

	accessToken, err := signHS256JWT(secret, map[string]any{
		"sub": stored.UserID,
		"iat": now.Unix(),
		"exp": accessExp.Unix(),
	})
	if err != nil {
		RespondError(c, "erro ao assinar token", http.StatusInternalServerError)
		return
	}

	newRefresh, err := issueRefreshToken(db, stored.UserID, now)
	if err != nil {
		RespondError(c, "erro ao gerar refresh token", http.StatusInternalServerError)
		return
	}

	RespondSuccess(c, RefreshResponse{
		AccessToken:        accessToken,
		AccessExpiresAt:    accessExp.Unix(),
		AccessExpiresAtISO: accessExp.UTC().Format(time.RFC3339),
		RefreshToken:       newRefresh,
	})
}
