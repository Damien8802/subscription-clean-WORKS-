package middleware

import (
	"subscription-system/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupCORS(cfg *config.Config) gin.HandlerFunc {
	corsConfig := cors.DefaultConfig()
	if len(cfg.AllowedOrigins) == 0 || (len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*") {
		corsConfig.AllowAllOrigins = true
	} else {
		corsConfig.AllowOrigins = cfg.AllowedOrigins
	}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	corsConfig.AllowHeaders = []string{
		"Origin", "Content-Type", "Content-Length", "Accept-Encoding",
		"X-CSRF-Token", "Authorization", "Accept", "Cache-Control", "X-Requested-With",
	}
	corsConfig.AllowCredentials = true
	corsConfig.MaxAge = 12 * 60 * 60
	return cors.New(corsConfig)
}
