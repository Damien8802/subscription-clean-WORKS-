package middleware

import (
	"github.com/gin-gonic/gin"
	"log"
	"time"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		log.Printf("[GIN] %v | %3v | %-7s | %s",
			start.Format("2006/01/02 - 15:04:05"),
			latency,
			c.Request.Method,
			path,
		)
	}
}
