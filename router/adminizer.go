package router

import (
	"net/http"

	"penelope/controllers"

	"github.com/gin-gonic/gin"
)

// Adminizer blocks access when user is not admin.
func Adminizer() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := controllers.GetUserLogged(c)
		if !ok {
			controllers.RespondError(c, "unauthorized", http.StatusUnauthorized)
			c.Abort()
			return
		}
		if !user.Admin {
			controllers.RespondError(c, "admin required", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}
