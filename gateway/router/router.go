package router

import (
	"github.com/RigelNana/arkstudy/gateway/docs"
	"github.com/RigelNana/arkstudy/gateway/handler"
	"github.com/RigelNana/arkstudy/gateway/middleware"
	ginMetrics "github.com/RigelNana/arkstudy/pkg/metrics/gin"

	"github.com/gin-gonic/gin"
)

func Setup(authHandler *handler.AuthHandler, userHandler *handler.UserHandler, materialHandler *handler.MaterialHandler, llmHandler *handler.LLMHandler, quizHandler *handler.QuizHandler, asrHandler *handler.ASRHandler, ocrHandler *handler.OCRHandler) *gin.Engine {
	r := gin.Default()

	// 添加 Prometheus 中间件
	r.Use(ginMetrics.PrometheusMiddleware("gateway"))

	// 创建认证中间件
	authValidator := middleware.NewAuthValidator()

	// 文档与 OpenAPI 路由
	docs.RegisterRoutes(r)

	api := r.Group("/api")
	{
		// 公开的认证相关路由（无需认证）
		api.POST("/register", authHandler.Register)
		api.POST("/login", authHandler.Login)
		api.GET("/validate", authHandler.Validate)

		// 需要认证的路由组
		protected := api.Group("")
		protected.Use(authValidator.JWTAuth()) // 应用 JWT 认证中间件
		{
			// 用户相关路由（需要认证）
			protected.GET("/users", userHandler.ListUsers)
			protected.GET("/users/:id", userHandler.GetUserByID)
			protected.GET("/users/username/:username", userHandler.GetUserByUsername)
			protected.GET("/users/email/:email", userHandler.GetUserByEmail)

			// 材料相关路由（需要认证）
			protected.POST("/materials/upload", materialHandler.UploadMaterial)
			protected.GET("/materials", materialHandler.ListMaterials)
			protected.GET("/materials/:id", materialHandler.GetMaterialByID)
			protected.DELETE("/materials/:id", materialHandler.DeleteMaterial)

			// AI处理相关路由（需要认证）
			protected.POST("/materials/process", materialHandler.ProcessMaterial)
			protected.GET("/processing/results", materialHandler.ListProcessingResults)
			protected.GET("/processing/results/:material_id", materialHandler.GetProcessingResult)
			protected.PUT("/processing/results/:task_id", materialHandler.UpdateProcessingResult)

			// LLM 对外最小可行路由
			protected.POST("/ai/ask", llmHandler.Ask)
			protected.GET("/ai/ask/stream", llmHandler.AskStream)
			protected.POST("/ai/ask/stream", llmHandler.AskStream)
			protected.GET("/ai/search", llmHandler.Search)

			// Quiz 自动出题相关路由（需要认证）
			protected.POST("/quiz/generate", quizHandler.GenerateQuiz)
			protected.GET("/quiz/:questionId", quizHandler.GetQuiz)
			protected.GET("/quiz", quizHandler.ListQuizzes)
			protected.POST("/quiz/:questionId/submit", quizHandler.SubmitAnswer)
			protected.GET("/quiz/user/:userId/history", quizHandler.GetUserHistory)
			protected.GET("/quiz/user/:userId/stats", quizHandler.GetKnowledgeStats)

			// ASR 语音识别相关路由（需要认证）
			protected.POST("/asr/process", asrHandler.ProcessVideo)
			protected.GET("/asr/segments/:material_id", asrHandler.GetSegments)
			protected.POST("/asr/search", asrHandler.SearchSegments)
			protected.GET("/asr/health", asrHandler.HealthCheck)

			// OCR 相关路由 (需要认证)
			protected.POST("/ocr/process", ocrHandler.ProcessOCR)
		}
	}
	return r
}
