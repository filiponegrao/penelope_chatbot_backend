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

type LoginRequest struct {
	Email    string `json:"email" form:"email"`
	Token    string `json:"token" form:"token"`
	Password string `json:"password" form:"password"`
}

// LoginResponse não devolve dados do usuário.
// O consumidor recebe o access token + refresh token e a data de expiração do access token,
// para conseguir antecipar o refresh antes de tomar 401.
type LoginResponse struct {
	AccessToken        string `json:"access_token"`
	AccessExpiresAt    int64  `json:"access_expires_at"`     // unix seconds
	AccessExpiresAtISO string `json:"access_expires_at_iso"` // RFC3339
	RefreshToken       string `json:"refresh_token"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		RespondError(c, "email (ou username) é obrigatório", http.StatusBadRequest)
		return
	}
	if req.Token == "" && req.Password == "" {
		RespondError(c, "token (ou password) é obrigatório", http.StatusBadRequest)
		return
	}

	db := dbpkg.DBInstance(c)
	if db == nil {
		RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
		return
	}

	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		RespondError(c, "usuário ou senha inválidos", http.StatusUnauthorized)
		return
	}

	// Venditto-style: compara o token enviado. Se vier password (legado), gera o token.
	providedToken := req.Token
	if providedToken == "" {
		passwordEncode := tools.EncryptTextSHA512(req.Password)
		passwordEncode = user.Email + ":" + passwordEncode
		passwordEncode = tools.EncryptTextSHA512(passwordEncode)
		providedToken = passwordEncode
	}
	if user.Password != providedToken {
		RespondError(c, "usuário ou senha inválidos", http.StatusUnauthorized)
		return
	}

	now := time.Now()

	// ✅ Sessão única: revoga todos os refresh tokens ativos do usuário antes de emitir um novo.
	if err := revokeAllUserRefreshTokens(db, user.ID, now); err != nil {
		RespondError(c, "erro ao revogar sessões anteriores", http.StatusInternalServerError)
		return
	}

	secret := getJWTSecret()
	accessTTLMinutes := getenvInt("JWT_ACCESS_TTL_MINUTES", 24*60) // default: 24h (mantém compatibilidade)
	accessExp := now.Add(time.Duration(accessTTLMinutes) * time.Minute)

	accessToken, err := signHS256JWT(secret, map[string]any{
		"sub":   user.ID,
		"email": user.Email,
		"iat":   now.Unix(),
		"exp":   accessExp.Unix(),
	})
	if err != nil {
		RespondError(c, "erro ao assinar token", http.StatusInternalServerError)
		return
	}

	refreshToken, err := issueRefreshToken(db, user.ID, now)
	if err != nil {
		RespondError(c, "erro ao gerar refresh token", http.StatusInternalServerError)
		return
	}

	RespondSuccess(c, LoginResponse{
		AccessToken:        accessToken,
		AccessExpiresAt:    accessExp.Unix(),
		AccessExpiresAtISO: accessExp.UTC().Format(time.RFC3339),
		RefreshToken:       refreshToken,
	})
}

func issueRefreshToken(db *gorm.DB, userID int64, now time.Time) (string, error) {
	refreshTTLDays := getenvInt("JWT_REFRESH_TTL_DAYS", 30) // default: 30 dias
	expiresAt := now.Add(time.Duration(refreshTTLDays) * 24 * time.Hour)

	raw := tools.RandomString(64)
	hash := tools.EncryptTextSHA512(raw)

	rt := models.RefreshToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: &expiresAt,
	}
	if err := db.Create(&rt).Error; err != nil {
		return "", err
	}
	return raw, nil
}

func revokeAllUserRefreshTokens(db *gorm.DB, userID int64, now time.Time) error {
	// Revoga qualquer refresh token ainda ativo (não revogado e não expirado).
	// Isso garante sessão única.
	return db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", userID, now).
		Update("revoked_at", now).Error
}
