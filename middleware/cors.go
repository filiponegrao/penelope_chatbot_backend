package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// NOTE:
// - CORS é relevante apenas para chamadas de BROWSER.
// - Webhooks do WhatsApp/Meta são server-to-server; CORS não protege webhook.
//
// Política atual (como combinado):
// - permitir apenas os nossos domínios oficiais.
func CORSMiddleware() gin.HandlerFunc {
	allowed := map[string]bool{
		"https://penelope.filiponegrao.com.br":      true,
		"https://test.penelope.filiponegrao.com.br": true,
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" && allowed[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Application-Version")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
