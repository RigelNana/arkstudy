package main

import (
	"log"
	"os"

	"github.com/RigelNana/arkstudy/gateway/handler"
	"github.com/RigelNana/arkstudy/gateway/router"
)

func main() {
	authClient := handler.NewAuthServiceClient()
	userClient := handler.NewUserServiceClient()
	materialClient := handler.NewMaterialServiceClient()

	authHandler := handler.NewAuthHandler(authClient, userClient)
	userHandler := handler.NewUserHandler(userClient)
	materialHandler := handler.NewMaterialHandler(materialClient)

	r := router.Setup(authHandler, userHandler, materialHandler)
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Gateway listening on %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("gateway failed: %v", err)
	}
}
