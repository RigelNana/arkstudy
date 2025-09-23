package handler


import (
	"net/http"

	"github.com/RigelNana/arkstudy/services/asr-service/config"
	"github.com/RigelNana/arkstudy/services/asr-service/models"
	"github.com/RigelNana/arkstudy/services/asr-service/service"

	"github.com/gin-gonic/gin"
)

type ASRHandler struct {
	asrService *service.ASRService
}

func NewASRHandler(cfg *config.Config) *ASRHandler {
	return &ASRHandler{
		asrService: service.NewASRService(cfg),
	}
}

// ProcessVideo handles video processing requests
func (h *ASRHandler) ProcessVideo(c *gin.Context) {
	var req models.ASRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request format: " + err.Error(),
		})
		return
	}

	response, err := h.asrService.ProcessVideo(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Processing failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetSegments retrieves ASR segments for a material
func (h *ASRHandler) GetSegments(c *gin.Context) {
	materialID := c.Param("material_id")
	if materialID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Material ID is required",
		})
		return
	}

	segments, err := h.asrService.GetSegmentsByMaterialID(materialID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve segments: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"material_id": materialID,
		"segments":    segments,
	})
}

// SearchSegments handles semantic search requests
func (h *ASRHandler) SearchSegments(c *gin.Context) {
	var req models.SearchASRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request format: " + err.Error(),
		})
		return
	}

	response, err := h.asrService.SearchSegments(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Search failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}