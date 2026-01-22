package controllers

import "github.com/gin-gonic/gin"

func RespondError(c *gin.Context, msg string, code int) {
	c.JSON(code, gin.H{"error": msg})
}

func RespondSuccess(c *gin.Context, payload any) {
	c.JSON(200, payload)
}
