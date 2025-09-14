package handler

import (
	"context"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"

	materialpb "github.com/RigelNana/arkstudy/proto/material"
	"github.com/gin-gonic/gin"
)

type MaterialHandler struct {
	materialClient materialpb.MaterialServiceClient
}

func NewMaterialHandler(materialClient materialpb.MaterialServiceClient) *MaterialHandler {
	return &MaterialHandler{materialClient: materialClient}
}

// UploadMaterial 上传文件
// POST /api/materials/upload
func (h *MaterialHandler) UploadMaterial(c *gin.Context) {
	// 从认证中间件获取 user_id
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userID, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user_id format"})
		return
	}

	// 解析表单数据
	title := c.PostForm("title")
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("UploadMaterial get file error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required", "detail": err.Error()})
		return
	}
	defer file.Close()

	log.Printf("UploadMaterial request: userID=%s, title=%s, filename=%s, size=%d",
		userID, title, header.Filename, header.Size)

	// 读取文件内容
	fileData, err := readFileData(file)
	if err != nil {
		log.Printf("UploadMaterial read file error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file", "detail": err.Error()})
		return
	}

	// 创建 gRPC 流
	stream, err := h.materialClient.UploadMaterial(context.Background())
	if err != nil {
		log.Printf("UploadMaterial create stream error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create upload stream", "detail": err.Error()})
		return
	}

	// 发送元数据
	metadataReq := &materialpb.UploadMaterialRequest{
		Data: &materialpb.UploadMaterialRequest_Metadata{
			Metadata: &materialpb.MaterialInfo{
				UserId:           userID,
				Title:            title,
				OriginalFilename: header.Filename,
			},
		},
	}

	if err := stream.Send(metadataReq); err != nil {
		log.Printf("UploadMaterial send metadata error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send metadata", "detail": err.Error()})
		return
	}

	// 分块发送文件数据
	chunkSize := 1024 * 64 // 64KB chunks
	for i := 0; i < len(fileData); i += chunkSize {
		end := i + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}

		chunkReq := &materialpb.UploadMaterialRequest{
			Data: &materialpb.UploadMaterialRequest_ChunkData{
				ChunkData: fileData[i:end],
			},
		}

		if err := stream.Send(chunkReq); err != nil {
			log.Printf("UploadMaterial send chunk error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send file chunk", "detail": err.Error()})
			return
		}
	}

	// 关闭流并接收响应
	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("UploadMaterial close and receive error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete upload", "detail": err.Error()})
		return
	}

	if !resp.Success {
		log.Printf("UploadMaterial failed: %s", resp.Message)
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload failed", "detail": resp.Message})
		return
	}

	log.Printf("UploadMaterial success: materialID=%s", resp.MaterialId)
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     resp.Message,
		"material_id": resp.MaterialId,
	})
}

// DeleteMaterial 删除文件
// DELETE /api/materials/:id
func (h *MaterialHandler) DeleteMaterial(c *gin.Context) {
	// 从认证中间件获取 user_id
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userID, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user_id format"})
		return
	}

	materialID := c.Param("id")
	if materialID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "material_id is required"})
		return
	}

	log.Printf("DeleteMaterial request: materialID=%s, userID=%s", materialID, userID)

	resp, err := h.materialClient.DeleteMaterial(context.Background(), &materialpb.DeleteMaterialRequest{
		MaterialId: materialID,
		UserId:     userID,
	})
	if err != nil {
		log.Printf("DeleteMaterial gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "detail": err.Error()})
		return
	}

	if !resp.Success {
		log.Printf("DeleteMaterial failed: %s", resp.Message)
		// 根据错误消息决定状态码
		statusCode := http.StatusBadRequest
		if resp.Message == "material not found" {
			statusCode = http.StatusNotFound
		} else if resp.Message == "permission denied" {
			statusCode = http.StatusForbidden
		}
		c.JSON(statusCode, gin.H{"error": resp.Message})
		return
	}

	log.Printf("DeleteMaterial success: materialID=%s", materialID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": resp.Message,
	})
}

// ListMaterials 获取用户的材料列表
// GET /api/materials?page=1&page_size=10
func (h *MaterialHandler) ListMaterials(c *gin.Context) {
	// 从认证中间件获取 user_id
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userID, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user_id format"})
		return
	}

	// 解析分页参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50 // 限制最大页面大小
	}

	log.Printf("ListMaterials request: userID=%s, page=%d, pageSize=%d", userID, page, pageSize)

	resp, err := h.materialClient.ListMaterials(context.Background(), &materialpb.ListMaterialsRequest{
		UserId:   userID,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		log.Printf("ListMaterials gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "detail": err.Error()})
		return
	}

	log.Printf("ListMaterials success: returned %d materials, total %d", len(resp.Materials), resp.Total)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"materials": resp.Materials,
			"total":     resp.Total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetMaterialByID 根据ID获取材料信息（需要添加到 proto 和 service 中）
// GET /api/materials/:id
func (h *MaterialHandler) GetMaterialByID(c *gin.Context) {
	// 从认证中间件获取 user_id
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userID, ok := userIDInterface.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user_id format"})
		return
	}

	materialID := c.Param("id")
	if materialID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "material_id is required"})
		return
	}

	log.Printf("GetMaterialByID request: materialID=%s, userID=%s", materialID, userID)

	// 由于 proto 中没有 GetMaterialByID，我们先用 ListMaterials 来实现
	// 在实际项目中，应该在 proto 中添加 GetMaterialByID RPC
	resp, err := h.materialClient.ListMaterials(context.Background(), &materialpb.ListMaterialsRequest{
		UserId:   userID,
		Page:     1,
		PageSize: 1000, // 设置较大值来获取所有材料
	})
	if err != nil {
		log.Printf("GetMaterialByID gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "detail": err.Error()})
		return
	}

	// 在结果中查找指定的材料
	for _, material := range resp.Materials {
		if material.Id == materialID {
			log.Printf("GetMaterialByID success: materialID=%s", materialID)
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    material,
			})
			return
		}
	}

	log.Printf("GetMaterialByID material not found: materialID=%s", materialID)
	c.JSON(http.StatusNotFound, gin.H{"error": "material not found"})
}

// ProcessMaterial AI处理材料
func (h *MaterialHandler) ProcessMaterial(c *gin.Context) {
	var req struct {
		MaterialID     string            `json:"material_id" binding:"required"`
		ProcessingType string            `json:"processing_type" binding:"required"`
		Options        map[string]string `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("ProcessMaterial bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 从认证中间件获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		log.Println("ProcessMaterial user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		log.Printf("ProcessMaterial invalid user_id type: %T", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// 转换处理类型
	var procType materialpb.ProcessingType
	switch req.ProcessingType {
	case "OCR":
		procType = materialpb.ProcessingType_OCR
	case "ASR":
		procType = materialpb.ProcessingType_ASR
	case "LLM_ANALYSIS":
		procType = materialpb.ProcessingType_LLM_ANALYSIS
	default:
		log.Printf("ProcessMaterial invalid processing type: %s", req.ProcessingType)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid processing type"})
		return
	}

	log.Printf("ProcessMaterial request: materialID=%s, type=%s, userID=%s",
		req.MaterialID, req.ProcessingType, userIDStr)

	// 调用 gRPC 服务
	resp, err := h.materialClient.ProcessMaterial(context.Background(), &materialpb.ProcessMaterialRequest{
		MaterialId: req.MaterialID,
		UserId:     userIDStr,
		Type:       procType,
		Options:    req.Options,
	})

	if err != nil {
		log.Printf("ProcessMaterial gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process material"})
		return
	}

	log.Printf("ProcessMaterial success: taskID=%s", resp.TaskId)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"task_id": resp.TaskId,
			"message": resp.Message,
			"result":  resp.Result,
		},
	})
}

// GetProcessingResult 获取处理结果
func (h *MaterialHandler) GetProcessingResult(c *gin.Context) {
	materialID := c.Param("material_id")
	processingTypeStr := c.Query("type")

	if materialID == "" {
		log.Println("GetProcessingResult material_id is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "material_id is required"})
		return
	}

	if processingTypeStr == "" {
		log.Println("GetProcessingResult processing type is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "processing type is required"})
		return
	}

	// 从认证中间件获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		log.Println("GetProcessingResult user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		log.Printf("GetProcessingResult invalid user_id type: %T", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// 转换处理类型
	var procType materialpb.ProcessingType
	switch processingTypeStr {
	case "OCR":
		procType = materialpb.ProcessingType_OCR
	case "ASR":
		procType = materialpb.ProcessingType_ASR
	case "LLM_ANALYSIS":
		procType = materialpb.ProcessingType_LLM_ANALYSIS
	default:
		log.Printf("GetProcessingResult invalid processing type: %s", processingTypeStr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid processing type"})
		return
	}

	log.Printf("GetProcessingResult request: materialID=%s, type=%s, userID=%s", materialID, processingTypeStr, userIDStr)

	// 调用 gRPC 服务
	resp, err := h.materialClient.GetProcessingResult(context.Background(), &materialpb.GetProcessingResultRequest{
		MaterialId: materialID,
		UserId:     userIDStr,
		Type:       procType,
	})

	if err != nil {
		log.Printf("GetProcessingResult gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get processing result"})
		return
	}

	if !resp.Found {
		log.Printf("GetProcessingResult not found: materialID=%s, type=%s", materialID, processingTypeStr)
		c.JSON(http.StatusNotFound, gin.H{"error": "Processing result not found"})
		return
	}

	log.Printf("GetProcessingResult success: materialID=%s, type=%s", materialID, processingTypeStr)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp.Result,
	})
}

// ListProcessingResults 列出处理结果
func (h *MaterialHandler) ListProcessingResults(c *gin.Context) {
	// 从认证中间件获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		log.Println("ListProcessingResults user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		log.Printf("ListProcessingResults invalid user_id type: %T", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// 获取查询参数
	materialID := c.Query("material_id")
	processingTypeStr := c.Query("processing_type")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// 转换处理类型（可选）
	var procType materialpb.ProcessingType
	if processingTypeStr != "" {
		switch processingTypeStr {
		case "OCR":
			procType = materialpb.ProcessingType_OCR
		case "ASR":
			procType = materialpb.ProcessingType_ASR
		case "LLM_ANALYSIS":
			procType = materialpb.ProcessingType_LLM_ANALYSIS
		default:
			log.Printf("ListProcessingResults invalid processing type: %s", processingTypeStr)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid processing type"})
			return
		}
	}

	log.Printf("ListProcessingResults request: userID=%s, materialID=%s, type=%s, page=%d, pageSize=%d",
		userIDStr, materialID, processingTypeStr, page, pageSize)

	// 调用 gRPC 服务
	resp, err := h.materialClient.ListProcessingResults(context.Background(), &materialpb.ListProcessingResultsRequest{
		MaterialId: materialID,
		UserId:     userIDStr,
		Type:       procType,
		Page:       int32(page),
		PageSize:   int32(pageSize),
	})

	if err != nil {
		log.Printf("ListProcessingResults gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list processing results"})
		return
	}

	log.Printf("ListProcessingResults success: count=%d, total=%d", len(resp.Results), resp.Total)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"results":   resp.Results,
			"total":     resp.Total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// UpdateProcessingResult 更新处理结果（主要供AI服务回调使用）
func (h *MaterialHandler) UpdateProcessingResult(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		log.Println("UpdateProcessingResult task_id is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id is required"})
		return
	}

	var req struct {
		Status       string            `json:"status"`
		Content      string            `json:"content"`
		ErrorMessage string            `json:"error_message"`
		Metadata     map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("UpdateProcessingResult bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 转换状态
	var procStatus materialpb.ProcessingStatus
	switch req.Status {
	case "PENDING":
		procStatus = materialpb.ProcessingStatus_PENDING
	case "PROCESSING":
		procStatus = materialpb.ProcessingStatus_PROCESSING
	case "COMPLETED":
		procStatus = materialpb.ProcessingStatus_COMPLETED
	case "FAILED":
		procStatus = materialpb.ProcessingStatus_FAILED
	default:
		log.Printf("UpdateProcessingResult invalid status: %s", req.Status)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	log.Printf("UpdateProcessingResult request: taskID=%s, status=%s",
		taskID, req.Status)

	// 调用 gRPC 服务
	resp, err := h.materialClient.UpdateProcessingResult(context.Background(), &materialpb.UpdateProcessingResultRequest{
		TaskId:       taskID,
		Status:       procStatus,
		Content:      req.Content,
		Metadata:     req.Metadata,
		ErrorMessage: req.ErrorMessage,
	})

	if err != nil {
		log.Printf("UpdateProcessingResult gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update processing result"})
		return
	}

	log.Printf("UpdateProcessingResult success: taskID=%s", taskID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": resp.Message,
	})
}

// readFileData 读取文件数据的辅助函数
func readFileData(file multipart.File) ([]byte, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return data, nil
}
