package router

import (
	"gateway/handler"

	"github.com/gin-gonic/gin"
)

func Setup(authHandler *handler.AuthHandler) *gin.Engine {
	r := gin.Default()
	api := r.Group("/api")
	{
		api.POST("/register", authHandler.Register)
		api.POST("/login", authHandler.Login)
		api.GET("/validate", authHandler.Validate)
	}
	return r
}
