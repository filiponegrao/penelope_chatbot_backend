package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	dbpkg "penelope/db"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

// jwtClaims representa o mínimo necessário para autenticação.
// O token emitido pelo Login usa o padrão:
//   { "sub": <userId>, "email": "...", "iat": ..., "exp": ... }
// Por isso aqui extraímos o "sub" ao invés de "user_id".
type jwtClaims struct {
	Sub uint  `json:"sub"`
	Exp int64 `json:"exp"`
	Iat int64 `json:"iat"`
}

const ctxUserKey = "auth_user"

// AuthRequired validates the Bearer token and loads the user from DB into context.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			RespondError(c, "ops! wait", http.StatusUnauthorized)
			c.Abort()
			return
		}
		token := strings.TrimSpace(h[len("Bearer "):])
		claims, ok := parseAndVerifyJWT(token, getJWTSecret())
		if !ok {
			RespondError(c, "ops! wat", http.StatusUnauthorized)
			c.Abort()
			return
		}
		if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
			RespondError(c, "ops! token expired", http.StatusUnauthorized)
			c.Abort()
			return
		}

		db := dbpkg.DBInstance(c)
		if db == nil {
			RespondError(c, "db não configurado no contexto", http.StatusInternalServerError)
			c.Abort()
			return
		}
		var user models.User
		if err := db.First(&user, claims.Sub).Error; err != nil {
			RespondError(c, "user not found", http.StatusUnauthorized)
			c.Abort()
			return
		}

		c.Set(ctxUserKey, user)
		c.Next()
	}
}

// GetUserLogged returns the user loaded by AuthRequired.
func GetUserLogged(c *gin.Context) (models.User, bool) {
	v, ok := c.Get(ctxUserKey)
	if !ok {
		return models.User{}, false
	}
	user, ok := v.(models.User)
	return user, ok
}

// parseAndVerifyJWT verifies HS256 JWT signed by our Login handler.
func parseAndVerifyJWT(token string, secret string) (jwtClaims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtClaims{}, false
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := mac.Sum(nil)
	expected := base64.RawURLEncoding.EncodeToString(sig)

	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return jwtClaims{}, false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtClaims{}, false
	}

	var claims jwtClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return jwtClaims{}, false
	}

	if claims.Sub == 0 {
		return jwtClaims{}, false
	}
	return claims, true
}
