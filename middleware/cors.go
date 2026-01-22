package middleware

import "github.com/gin-gonic/gin"

// CORSMiddleware libera CORS básico (útil para testes locais e integração com front).
// Se/Quando precisar endurecer isso, troque para uma lista de origens permitidas.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", "*")
		header.Set("Access-Control-Allow-Credentials", "true")
		header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Application-Version")
		header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
