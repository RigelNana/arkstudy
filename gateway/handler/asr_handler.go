package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RigelNana/arkstudy/proto/asr"
)

type ASRHandler struct {
	client asr.ASRServiceClient
	logger *logrus.Logger
}

func NewASRHandler(serviceAddr string, logger *logrus.Logger) *ASRHandler {
	// Create gRPC connection to ASR service
	conn, err := grpc.Dial(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to ASR service")
	}

	client := asr.NewASRServiceClient(conn)

	return &ASRHandler{
		client: client,
		logger: logger,
	}
}

// ProcessVideo handles video ASR processing requests
func (h *ASRHandler) ProcessVideo(c *gin.Context) {
	h.logger.Info("Received ASR process video request")

	// Parse request JSON
	var req struct {
		MaterialID uint   `json:"material_id" binding:"required"`
		VideoURL   string `json:"video_url"`
		VideoPath  string `json:"video_path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).Error("Failed to parse request body")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request format",
		})
		return
	}

	// Create gRPC request
	grpcReq := &asr.ProcessVideoRequest{
		MaterialId: uint64(req.MaterialID),
		VideoUrl:   req.VideoURL,
		VideoPath:  req.VideoPath,
	}

	// Call ASR service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.client.ProcessVideo(ctx, grpcReq)
	if err != nil {
		h.logger.WithError(err).Error("Failed to call ASR service")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "ASR service call failed",
		})
		return
	}

	// Convert gRPC response to HTTP response
	segments := make([]map[string]interface{}, len(resp.Segments))
	for i, segment := range resp.Segments {
		segments[i] = map[string]interface{}{
			"id":               segment.Id,
			"material_id":      segment.MaterialId,
			"start_time":       segment.StartTime,
			"end_time":         segment.EndTime,
			"text":             segment.Text,
			"confidence":       segment.Confidence,
			"embedding_vector": segment.EmbeddingVector,
			"created_at":       segment.CreatedAt,
			"updated_at":       segment.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  resp.Success,
		"message":  resp.Message,
		"segments": segments,
	})
}

// GetSegments retrieves ASR segments for a material
func (h *ASRHandler) GetSegments(c *gin.Context) {
	materialIDStr := c.Param("material_id")
	if materialIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Material ID is required",
		})
		return
	}

	materialID, err := strconv.ParseUint(materialIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid material ID",
		})
		return
	}

	h.logger.WithField("material_id", materialID).Info("Retrieving ASR segments")

	// Create gRPC request
	grpcReq := &asr.GetSegmentsRequest{
		MaterialId: materialID,
	}

	// Call ASR service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetSegments(ctx, grpcReq)
	if err != nil {
		h.logger.WithError(err).Error("Failed to call ASR service")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "ASR service call failed",
		})
		return
	}

	// Convert gRPC response to HTTP response
	segments := make([]map[string]interface{}, len(resp.Segments))
	for i, segment := range resp.Segments {
		segments[i] = map[string]interface{}{
			"id":               segment.Id,
			"material_id":      segment.MaterialId,
			"start_time":       segment.StartTime,
			"end_time":         segment.EndTime,
			"text":             segment.Text,
			"confidence":       segment.Confidence,
			"embedding_vector": segment.EmbeddingVector,
			"created_at":       segment.CreatedAt,
			"updated_at":       segment.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  resp.Success,
		"message":  resp.Message,
		"segments": segments,
	})
}

// SearchSegments searches for ASR segments
func (h *ASRHandler) SearchSegments(c *gin.Context) {
	h.logger.Info("Received ASR search segments request")

	// Parse request JSON
	var req struct {
		Query      string `json:"query" binding:"required"`
		MaterialID *uint  `json:"material_id,omitempty"`
		Limit      *int   `json:"limit,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).Error("Failed to parse request body")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request format",
		})
		return
	}

	// Create gRPC request
	grpcReq := &asr.SearchSegmentsRequest{
		Query: req.Query,
	}

	if req.MaterialID != nil {
		grpcReq.MaterialId = uint64(*req.MaterialID)
	}

	if req.Limit != nil {
		grpcReq.Limit = int32(*req.Limit)
	}

	// Call ASR service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.SearchSegments(ctx, grpcReq)
	if err != nil {
		h.logger.WithError(err).Error("Failed to call ASR service")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "ASR service call failed",
		})
		return
	}

	// Convert gRPC response to HTTP response
	segments := make([]map[string]interface{}, len(resp.Segments))
	for i, segment := range resp.Segments {
		segments[i] = map[string]interface{}{
			"id":               segment.Id,
			"material_id":      segment.MaterialId,
			"start_time":       segment.StartTime,
			"end_time":         segment.EndTime,
			"text":             segment.Text,
			"confidence":       segment.Confidence,
			"embedding_vector": segment.EmbeddingVector,
			"created_at":       segment.CreatedAt,
			"updated_at":       segment.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  resp.Success,
		"message":  resp.Message,
		"segments": segments,
	})
}

// HealthCheck provides health status of ASR service
func (h *ASRHandler) HealthCheck(c *gin.Context) {
	h.logger.Info("ASR health check request")

	// Create gRPC request
	grpcReq := &asr.HealthCheckRequest{}

	// Call ASR service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.client.HealthCheck(ctx, grpcReq)
	if err != nil {
		h.logger.WithError(err).Error("ASR health check failed")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"message": "ASR service unavailable",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  resp.Status,
		"message": resp.Message,
	})
}