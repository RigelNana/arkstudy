package main

import (
	"log"
	"os"

	"github.com/RigelNana/arkstudy/gateway/handler"
	"github.com/RigelNana/arkstudy/gateway/router"
	"github.com/RigelNana/arkstudy/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
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

	// 初始化 Quiz Handler
	logger := logrus.New()
	quizServiceAddr := os.Getenv("QUIZ_SERVICE_ADDR")
	if quizServiceAddr == "" {
		// 检查K8s环境变量
		quizHost := os.Getenv("ARKSTUDY_QUIZ_SERVICE_SERVICE_HOST")
		quizPort := os.Getenv("ARKSTUDY_QUIZ_SERVICE_SERVICE_PORT")
		if quizHost != "" && quizPort != "" {
			quizServiceAddr = quizHost + ":" + quizPort
		} else {
			quizServiceAddr = "quiz-service:50056" // 使用正确的端口
		}
	}
	log.Printf("Quiz service address: %s", quizServiceAddr)
	quizHandler := handler.NewQuizHandler(quizServiceAddr, logger)

	// 初始化 ASR Handler
	asrServiceAddr := os.Getenv("ASR_SERVICE_ADDR")
	if asrServiceAddr == "" {
		// 检查K8s环境变量
		asrHost := os.Getenv("ARKSTUDY_ASR_SERVICE_SERVICE_HOST")
		asrPort := os.Getenv("ARKSTUDY_ASR_SERVICE_SERVICE_PORT")
		if asrHost != "" && asrPort != "" {
			asrServiceAddr = asrHost + ":" + asrPort
		} else {
			asrServiceAddr = "asr-service:50057"
		}
	}
	log.Printf("ASR service address: %s", asrServiceAddr)
	asrHandler := handler.NewASRHandler(asrServiceAddr, logger)

	// 初始化 OCR Handler
	ocrHandler := handler.NewOCRHandler()

	r := router.Setup(authHandler, userHandler, materialHandler, llmHandler, quizHandler, asrHandler, ocrHandler)

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
