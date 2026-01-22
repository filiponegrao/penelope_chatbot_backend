package controllers

import (
	"net/http"
	"os"
	"time"

	dbpkg "penelope/db"
	"penelope/models"
	"penelope/tools"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		RespondError(c, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		RespondError(c, "email e password são obrigatórios", http.StatusBadRequest)
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

	// valida senha (mesma regra usada no CreateUser)
	passwordEncode := tools.EncryptTextSHA512(req.Password)
	passwordEncode = user.Email + ":" + passwordEncode
	passwordEncode = tools.EncryptTextSHA512(passwordEncode)
	if user.Password != passwordEncode {
		RespondError(c, "usuário ou senha inválidos", http.StatusUnauthorized)
		return
	}

	if user.Status == models.USER_STATUS_PENDING {
		RespondError(c, "usuário pendente de ativação", http.StatusForbidden)
		return
	}
	if user.Status == models.USER_STATUS_BLOCKED {
		RespondError(c, "usuário bloqueado", http.StatusForbidden)
		return
	}

	secret := getenv("JWT_SECRET", "")
	if secret == "" {
		// tenta ler do config.json via env injetada; se não tiver, usa fallback de config.json
		// OBS: como o config é carregado no main, preferimos expor via env se quiser trocar sem rebuild.
		secret = getenv("PENELOPE_JWT_SECRET", "")
	}
	if secret == "" {
		// último fallback: config.json (valor default CHANGE_ME)
		// Para não acoplar controllers<->config, aceitamos o risco em dev.
		secret = "CHANGE_ME"
	}

	signed, err := signHS256JWT(secret, map[string]any{
		"sub":   user.ID,
		"email": user.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		RespondError(c, "erro ao assinar token", http.StatusInternalServerError)
		return
	}

	user.Password = ""
	RespondSuccess(c, LoginResponse{Token: signed, User: user})
}

func signHS256JWT(secret string, claims map[string]any) (string, error) {
	// Header
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	headB, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	// Payload
	payloadB, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	enc := base64.RawURLEncoding
	unsigned := enc.EncodeToString(headB) + "." + enc.EncodeToString(payloadB)

	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(unsigned))
	sig := enc.EncodeToString(h.Sum(nil))
	return unsigned + "." + sig, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
