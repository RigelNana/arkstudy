package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/RigelNana/arkstudy/proto/material"
	"github.com/RigelNana/arkstudy/services/material-service/models"
	"github.com/RigelNana/arkstudy/services/material-service/service"
	"github.com/google/uuid"
)

type MaterialRPCServer struct {
	material.UnimplementedMaterialServiceServer
	svc service.MaterialService
}

func NewMaterialRPCServer(svc service.MaterialService) *MaterialRPCServer {
	return &MaterialRPCServer{svc: svc}
}

func (s *MaterialRPCServer) UploadMaterial(stream material.MaterialService_UploadMaterialServer) error {
	var metadata *material.MaterialInfo
	var fileData bytes.Buffer

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("UploadMaterial receive error: %v", err)
			return err
		}

		switch data := req.Data.(type) {
		case *material.UploadMaterialRequest_Metadata:
			metadata = data.Metadata
			log.Printf("UploadMaterial metadata: UserID=%s, Title=%s, Filename=%s",
				metadata.UserId, metadata.Title, metadata.OriginalFilename)
		case *material.UploadMaterialRequest_ChunkData:
			fileData.Write(data.ChunkData)
		}
	}

	if metadata == nil {
		err := fmt.Errorf("metadata is required")
		log.Printf("UploadMaterial failed: %v", err)
		return stream.SendAndClose(&material.UploadMaterialResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	userID, err := uuid.Parse(metadata.UserId)
	if err != nil {
		log.Printf("UploadMaterial failed: invalid user_id %s", metadata.UserId)
		return stream.SendAndClose(&material.UploadMaterialResponse{
			Success: false,
			Message: "invalid user_id",
		})
	}

	mat, err := s.svc.UploadFile(userID, metadata.Title, metadata.OriginalFilename, fileData.Bytes())
	if err != nil {
		log.Printf("UploadMaterial failed: %v", err)
		return stream.SendAndClose(&material.UploadMaterialResponse{
			Success: false,
			Message: err.Error(),
		})
	}

	log.Printf("UploadMaterial success: ID=%s", mat.ID.String())
	return stream.SendAndClose(&material.UploadMaterialResponse{
		Success:    true,
		Message:    "Upload successful",
		MaterialId: mat.ID.String(),
	})
}

func (s *MaterialRPCServer) DeleteMaterial(ctx context.Context, req *material.DeleteMaterialRequest) (*material.DeleteMaterialResponse, error) {
	log.Printf("DeleteMaterial called: MaterialID=%s, UserID=%s", req.MaterialId, req.UserId)

	materialID, err := uuid.Parse(req.MaterialId)
	if err != nil {
		log.Printf("DeleteMaterial failed: invalid material_id %s", req.MaterialId)
		return &material.DeleteMaterialResponse{
			Success: false,
			Message: "invalid material_id",
		}, nil
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		log.Printf("DeleteMaterial failed: invalid user_id %s", req.UserId)
		return &material.DeleteMaterialResponse{
			Success: false,
			Message: "invalid user_id",
		}, nil
	}

	// 检查材料是否属于该用户
	mat, err := s.svc.GetByID(materialID)
	if err != nil {
		log.Printf("DeleteMaterial failed: material not found %s", req.MaterialId)
		return &material.DeleteMaterialResponse{
			Success: false,
			Message: "material not found",
		}, nil
	}

	if mat.UserID != userID {
		log.Printf("DeleteMaterial failed: permission denied for user %s", req.UserId)
		return &material.DeleteMaterialResponse{
			Success: false,
			Message: "permission denied",
		}, nil
	}

	err = s.svc.Delete(materialID)
	if err != nil {
		log.Printf("DeleteMaterial failed: %v", err)
		return &material.DeleteMaterialResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	log.Printf("DeleteMaterial success: ID=%s", req.MaterialId)
	return &material.DeleteMaterialResponse{
		Success: true,
		Message: "Delete successful",
	}, nil
}

func (s *MaterialRPCServer) ListMaterials(ctx context.Context, req *material.ListMaterialsRequest) (*material.ListMaterialsResponse, error) {
	log.Printf("ListMaterials called: UserID=%s, Page=%d, PageSize=%d", req.UserId, req.Page, req.PageSize)

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		log.Printf("ListMaterials failed: invalid user_id %s", req.UserId)
		return &material.ListMaterialsResponse{
			Materials: []*material.MaterialInfo{},
			Total:     0,
		}, nil
	}

	// 设置默认分页参数
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	materials, total, err := s.svc.GetByUserIDWithPagination(userID, page, pageSize)
	if err != nil {
		log.Printf("ListMaterials failed: %v", err)
		return &material.ListMaterialsResponse{
			Materials: []*material.MaterialInfo{},
			Total:     0,
		}, nil
	}

	resp := &material.ListMaterialsResponse{
		Total: total,
	}

	for _, mat := range materials {
		materialInfo := &material.MaterialInfo{
			Id:               mat.ID.String(),
			UserId:           mat.UserID.String(),
			Title:            mat.Title,
			OriginalFilename: mat.OriginalFilename,
			FileType:         mat.FileType,
			SizeBytes:        mat.SizeBytes,
			Status:           mat.Status,
			CreatedAt:        mat.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		resp.Materials = append(resp.Materials, materialInfo)
	}

	log.Printf("ListMaterials success: returned %d materials, total %d", len(resp.Materials), total)
	return resp, nil
}

// ======================= AI 处理相关 RPC 方法 =======================

func (s *MaterialRPCServer) ProcessMaterial(ctx context.Context, req *material.ProcessMaterialRequest) (*material.ProcessMaterialResponse, error) {
	log.Printf("ProcessMaterial called: MaterialID=%s, UserID=%s, Type=%s", req.MaterialId, req.UserId, req.Type.String())

	materialID, err := uuid.Parse(req.MaterialId)
	if err != nil {
		log.Printf("ProcessMaterial failed: invalid material_id %s", req.MaterialId)
		return &material.ProcessMaterialResponse{
			Success: false,
			Message: "invalid material_id",
		}, nil
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		log.Printf("ProcessMaterial failed: invalid user_id %s", req.UserId)
		return &material.ProcessMaterialResponse{
			Success: false,
			Message: "invalid user_id",
		}, nil
	}

	// 转换处理类型
	processType := convertProcessingType(req.Type)
	if processType == "" {
		log.Printf("ProcessMaterial failed: unsupported processing type %s", req.Type.String())
		return &material.ProcessMaterialResponse{
			Success: false,
			Message: "unsupported processing type",
		}, nil
	}

	// 调用服务层
	result, err := s.svc.ProcessMaterial(materialID, userID, processType, req.Options)
	if err != nil {
		log.Printf("ProcessMaterial failed: %v", err)
		return &material.ProcessMaterialResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// 转换结果
	protoResult := convertToProtoProcessingResult(result)

	log.Printf("ProcessMaterial success: TaskID=%s", result.TaskID)
	return &material.ProcessMaterialResponse{
		Success: true,
		Message: "Processing started successfully",
		TaskId:  result.TaskID,
		Result:  protoResult,
	}, nil
}

func (s *MaterialRPCServer) GetProcessingResult(ctx context.Context, req *material.GetProcessingResultRequest) (*material.GetProcessingResultResponse, error) {
	log.Printf("GetProcessingResult called: MaterialID=%s, UserID=%s, Type=%s", req.MaterialId, req.UserId, req.Type.String())

	materialID, err := uuid.Parse(req.MaterialId)
	if err != nil {
		log.Printf("GetProcessingResult failed: invalid material_id %s", req.MaterialId)
		return &material.GetProcessingResultResponse{
			Found:   false,
			Message: "invalid material_id",
		}, nil
	}

	// 转换处理类型
	processType := convertProcessingType(req.Type)
	if processType == "" {
		log.Printf("GetProcessingResult failed: unsupported processing type %s", req.Type.String())
		return &material.GetProcessingResultResponse{
			Found:   false,
			Message: "unsupported processing type",
		}, nil
	}

	// 调用服务层
	result, err := s.svc.GetProcessingResult(materialID, processType)
	if err != nil {
		log.Printf("GetProcessingResult not found: MaterialID=%s, Type=%s", req.MaterialId, processType)
		return &material.GetProcessingResultResponse{
			Found:   false,
			Message: "processing result not found",
		}, nil
	}

	// 转换结果
	protoResult := convertToProtoProcessingResult(result)

	log.Printf("GetProcessingResult success: MaterialID=%s, Type=%s, Status=%s", req.MaterialId, processType, result.Status)
	return &material.GetProcessingResultResponse{
		Found:   true,
		Message: "success",
		Result:  protoResult,
	}, nil
}

func (s *MaterialRPCServer) ListProcessingResults(ctx context.Context, req *material.ListProcessingResultsRequest) (*material.ListProcessingResultsResponse, error) {
	log.Printf("ListProcessingResults called: MaterialID=%s, UserID=%s, Type=%s, Page=%d, PageSize=%d",
		req.MaterialId, req.UserId, req.Type.String(), req.Page, req.PageSize)

	materialID, err := uuid.Parse(req.MaterialId)
	if err != nil {
		log.Printf("ListProcessingResults failed: invalid material_id %s", req.MaterialId)
		return &material.ListProcessingResultsResponse{
			Results: []*material.ProcessingResult{},
			Total:   0,
		}, nil
	}

	// 设置默认分页参数
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	// 调用服务层
	results, total, err := s.svc.ListProcessingResults(materialID, page, pageSize)
	if err != nil {
		log.Printf("ListProcessingResults failed: %v", err)
		return &material.ListProcessingResultsResponse{
			Results: []*material.ProcessingResult{},
			Total:   0,
		}, nil
	}

	// 转换结果
	resp := &material.ListProcessingResultsResponse{
		Total: total,
	}

	for _, result := range results {
		protoResult := convertToProtoProcessingResult(result)
		resp.Results = append(resp.Results, protoResult)
	}

	log.Printf("ListProcessingResults success: returned %d results, total %d", len(resp.Results), total)
	return resp, nil
}

func (s *MaterialRPCServer) UpdateProcessingResult(ctx context.Context, req *material.UpdateProcessingResultRequest) (*material.UpdateProcessingResultResponse, error) {
	log.Printf("UpdateProcessingResult called: TaskID=%s, Status=%s", req.TaskId, req.Status.String())

	if req.TaskId == "" {
		log.Printf("UpdateProcessingResult failed: task_id is required")
		return &material.UpdateProcessingResultResponse{
			Success: false,
			Message: "task_id is required",
		}, nil
	}

	// 转换状态
	status := convertProcessingStatus(req.Status)
	if status == "" {
		log.Printf("UpdateProcessingResult failed: unsupported status %s", req.Status.String())
		return &material.UpdateProcessingResultResponse{
			Success: false,
			Message: "unsupported status",
		}, nil
	}

	// 调用服务层
	err := s.svc.UpdateProcessingResult(req.TaskId, status, req.Content, convertMetadata(req.Metadata), req.ErrorMessage)
	if err != nil {
		log.Printf("UpdateProcessingResult failed: %v", err)
		return &material.UpdateProcessingResultResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	log.Printf("UpdateProcessingResult success: TaskID=%s", req.TaskId)
	return &material.UpdateProcessingResultResponse{
		Success: true,
		Message: "Processing result updated successfully",
	}, nil
}

// ======================= 辅助转换函数 =======================

func convertProcessingType(protoType material.ProcessingType) string {
	switch protoType {
	case material.ProcessingType_OCR:
		return models.ProcessingTypeOCR
	case material.ProcessingType_ASR:
		return models.ProcessingTypeASR
	case material.ProcessingType_LLM_ANALYSIS:
		return models.ProcessingTypeLLMAnalysis
	default:
		return ""
	}
}

func convertProcessingStatus(protoStatus material.ProcessingStatus) string {
	switch protoStatus {
	case material.ProcessingStatus_PENDING:
		return models.ProcessingStatusPending
	case material.ProcessingStatus_PROCESSING:
		return models.ProcessingStatusProcessing
	case material.ProcessingStatus_COMPLETED:
		return models.ProcessingStatusCompleted
	case material.ProcessingStatus_FAILED:
		return models.ProcessingStatusFailed
	default:
		return ""
	}
}

func convertToProtoProcessingType(modelType string) material.ProcessingType {
	switch modelType {
	case models.ProcessingTypeOCR:
		return material.ProcessingType_OCR
	case models.ProcessingTypeASR:
		return material.ProcessingType_ASR
	case models.ProcessingTypeLLMAnalysis:
		return material.ProcessingType_LLM_ANALYSIS
	default:
		return material.ProcessingType_OCR // 默认值
	}
}

func convertToProtoProcessingStatus(modelStatus string) material.ProcessingStatus {
	switch modelStatus {
	case models.ProcessingStatusPending:
		return material.ProcessingStatus_PENDING
	case models.ProcessingStatusProcessing:
		return material.ProcessingStatus_PROCESSING
	case models.ProcessingStatusCompleted:
		return material.ProcessingStatus_COMPLETED
	case models.ProcessingStatusFailed:
		return material.ProcessingStatus_FAILED
	default:
		return material.ProcessingStatus_PENDING // 默认值
	}
}

func convertToProtoProcessingResult(result *models.ProcessingResult) *material.ProcessingResult {
	metadata := make(map[string]string)
	// 简单的 JSON 转换，实际项目中可能需要更复杂的处理
	// 这里假设 Metadata 是简单的 key-value 对

	return &material.ProcessingResult{
		Id:           result.ID.String(),
		MaterialId:   result.MaterialID.String(),
		TaskId:       result.TaskID,
		Type:         convertToProtoProcessingType(result.Type),
		Status:       convertToProtoProcessingStatus(result.Status),
		Content:      result.Content,
		Metadata:     metadata,
		CreatedAt:    result.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    result.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ErrorMessage: result.ErrorMessage,
	}
}

func convertMetadata(protoMetadata map[string]string) map[string]interface{} {
	metadata := make(map[string]interface{})
	for k, v := range protoMetadata {
		metadata[k] = v
	}
	return metadata
}
