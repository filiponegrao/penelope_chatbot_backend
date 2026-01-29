package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Me(c *gin.Context) {
	user, ok := GetUserLogged(c)
	if !ok {
		RespondError(c, "unauthorized", http.StatusUnauthorized)
		return
	}
	user.Password = ""
	c.JSON(http.StatusOK, gin.H{"user": user})
}
