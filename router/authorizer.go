package router

import (
	"net/http"

	"penelope/controllers"
	"penelope/models"

	"github.com/gin-gonic/gin"
)

// Authorizer blocks access to protected routes when user is not active.
func Authorizer() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := controllers.GetUserLogged(c)
		if !ok {
			controllers.RespondError(c, "unauthorized", http.StatusUnauthorized)
			c.Abort()
			return
		}

		if user.Status == models.USER_STATUS_PENDING {
			controllers.RespondError(c, "necessário confirmar a conta", http.StatusForbidden)
			c.Abort()
			return
		}
		if user.Status == models.USER_STATUS_BLOCKED {
			controllers.RespondError(c, "sem acesso ao aplicativo", http.StatusForbidden)
			c.Abort()
			return
		}

		c.Next()
	}
}
