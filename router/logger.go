package router

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger logs method, path, status and latency.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		log.Printf("%s %s -> %d (%s)", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	}
}
