package main

import (
	"log"
	"os"

	"github.com/RigelNana/arkstudy/gateway/handler"
	"github.com/RigelNana/arkstudy/gateway/router"
	"github.com/RigelNana/arkstudy/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 启动 Prometheus metrics 服务器
	metrics.StartMetricsServer("2112")
	log.Printf("Prometheus metrics server started on :2112")

	authClient := handler.NewAuthServiceClient()
	userClient := handler.NewUserServiceClient()
	materialClient := handler.NewMaterialServiceClient()
	llmClient := handler.NewLLMServiceClient()

	authHandler := handler.NewAuthHandler(authClient, userClient)
	userHandler := handler.NewUserHandler(userClient)
	materialHandler := handler.NewMaterialHandler(materialClient)
	llmHandler := handler.NewLLMHandler(llmClient)

	r := router.Setup(authHandler, userHandler, materialHandler, llmHandler)

	// 添加 /metrics 端点到主服务器
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Gateway listening on %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("gateway failed: %v", err)
	}
}
