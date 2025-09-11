package main

import (
	"log"
	"os"

	"gateway/handler"
	"gateway/router"
)

func main() {
	authClient := handler.NewAuthServiceClient()
	userClient := handler.NewUserServiceClient()
	authHandler := handler.NewAuthHandler(authClient, userClient)
	r := router.Setup(authHandler)
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Gateway listening on %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("gateway failed: %v", err)
	}
}
