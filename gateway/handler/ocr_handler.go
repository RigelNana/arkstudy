package handler

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	aipb "github.com/RigelNana/arkstudy/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type OCRHandler struct {
	client aipb.AIServiceClient
}

func NewOCRHandler() *OCRHandler {
	addr := os.Getenv("OCR_GRPC_ADDR")
	if addr == "" {
		// 检查K8s环境变量
		ocrHost := os.Getenv("ARKSTUDY_OCR_SERVICE_SERVICE_HOST")
		ocrPort := os.Getenv("ARKSTUDY_OCR_SERVICE_SERVICE_PORT")
		if ocrHost != "" && ocrPort != "" {
			addr = ocrHost + ":" + ocrPort
		} else {
			addr = "arkstudy-ocr-service:50055" // 使用服务名作为默认值
		}
	}
	log.Printf("OCR service address: %s", addr)
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("dial ocr-service: %v", err)
	}
	return &OCRHandler{client: aipb.NewAIServiceClient(conn)}
}

func (h *OCRHandler) ProcessOCR(c *gin.Context) {
	var req struct {
		FileURL  string `json:"file_url" binding:"required"`
		FileType string `json:"file_type"`
		TaskID   string `json:"task_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.client.ProcessOCR(ctx, &aipb.OCRRequest{
		FileUrl:  req.FileURL,
		FileType: req.FileType,
		TaskId:   req.TaskID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process ocr", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": resp.GetTaskId(),
		"text":    resp.GetText(),
	})
}
