package service

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/RigelNana/arkstudy/services/material-service/config"
	"github.com/RigelNana/arkstudy/services/material-service/models"
	"github.com/RigelNana/arkstudy/services/material-service/repository"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MaterialService interface {
	UploadFile(userID uuid.UUID, title, originalFilename string, fileData []byte) (*models.Material, error)
	GetByID(id uuid.UUID) (*models.Material, error)
	GetByUserIDWithPagination(userID uuid.UUID, page, pageSize int32) ([]*models.Material, int64, error)
	UpdateStatus(id uuid.UUID, status string) error
	Delete(id uuid.UUID) error
	GetFileURL(material *models.Material, expiry time.Duration) (string, error)

	// AI 处理相关方法
	ProcessMaterial(materialID uuid.UUID, userID uuid.UUID, processType string, options map[string]string) (*models.ProcessingResult, error)
	GetProcessingResult(materialID uuid.UUID, processType string) (*models.ProcessingResult, error)
	ListProcessingResults(materialID uuid.UUID, page, pageSize int32) ([]*models.ProcessingResult, int64, error)
	UpdateProcessingResult(taskID string, status string, content string, metadata map[string]interface{}, errorMessage string) error
}

type MaterialServiceImpl struct {
	repo           repository.MaterialRepository
	processingRepo repository.ProcessingResultRepository
	minioClient    *minio.Client
	config         *config.Config
}

func NewMaterialService(repo repository.MaterialRepository, processingRepo repository.ProcessingResultRepository, cfg *config.Config) (MaterialService, error) {
	// 初始化 MinIO 客户端
	minioClient, err := minio.New(cfg.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKeyID, cfg.MinIO.SecretAccessKey, ""),
		Secure: cfg.MinIO.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// 确保存储桶存在
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, cfg.MinIO.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, cfg.MinIO.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &MaterialServiceImpl{
		repo:           repo,
		processingRepo: processingRepo,
		minioClient:    minioClient,
		config:         cfg,
	}, nil
}

func (s *MaterialServiceImpl) UploadFile(userID uuid.UUID, title, originalFilename string, fileData []byte) (*models.Material, error) {
	// 生成唯一的对象名
	ext := filepath.Ext(originalFilename)
	objectName := fmt.Sprintf("%s/%s%s", userID.String(), uuid.New().String(), ext)

	// 检测文件类型
	fileType := s.detectFileType(originalFilename)

	// 创建材料记录
	material := &models.Material{
		UserID:           userID,
		Title:            title,
		OriginalFilename: originalFilename,
		FileType:         fileType,
		SizeBytes:        int64(len(fileData)),
		Status:           "uploading",
		MinioBucket:      s.config.MinIO.BucketName,
		MinioObjectName:  objectName,
	}

	// 先保存到数据库
	if err := s.repo.Create(material); err != nil {
		return nil, fmt.Errorf("failed to save material record: %w", err)
	}

	// 上传到 MinIO
	ctx := context.Background()
	reader := bytes.NewReader(fileData)

	_, err := s.minioClient.PutObject(ctx, s.config.MinIO.BucketName, objectName, reader, int64(len(fileData)), minio.PutObjectOptions{
		ContentType: s.getContentType(fileType),
	})

	if err != nil {
		// 上传失败，更新状态
		s.repo.UpdateStatus(material.ID, "failed")
		return nil, fmt.Errorf("failed to upload file to MinIO: %w", err)
	}

	// 上传成功，更新状态
	if err := s.repo.UpdateStatus(material.ID, "success"); err != nil {
		return nil, fmt.Errorf("failed to update material status: %w", err)
	}

	material.Status = "success"
	return material, nil
}

func (s *MaterialServiceImpl) GetByID(id uuid.UUID) (*models.Material, error) {
	return s.repo.GetByID(id)
}

func (s *MaterialServiceImpl) GetByUserIDWithPagination(userID uuid.UUID, page, pageSize int32) ([]*models.Material, int64, error) {
	return s.repo.GetByUserIDWithPagination(userID, page, pageSize)
}

func (s *MaterialServiceImpl) UpdateStatus(id uuid.UUID, status string) error {
	return s.repo.UpdateStatus(id, status)
}

func (s *MaterialServiceImpl) Delete(id uuid.UUID) error {
	// 获取材料信息
	material, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get material: %w", err)
	}

	// 从 MinIO 删除文件
	ctx := context.Background()
	err = s.minioClient.RemoveObject(ctx, material.MinioBucket, material.MinioObjectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove file from MinIO: %w", err)
	}

	// 从数据库删除记录
	return s.repo.Delete(id)
}

func (s *MaterialServiceImpl) GetFileURL(material *models.Material, expiry time.Duration) (string, error) {
	ctx := context.Background()
	url, err := s.minioClient.PresignedGetObject(ctx, material.MinioBucket, material.MinioObjectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return url.String(), nil
}

// 辅助方法：检测文件类型
func (s *MaterialServiceImpl) detectFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "document"
	case ".jpg", ".jpeg", ".png", ".gif":
		return "image"
	case ".mp4", ".avi", ".mov":
		return "video"
	case ".mp3", ".wav", ".flac":
		return "audio"
	case ".txt":
		return "text"
	default:
		return "other"
	}
}

// 辅助方法：获取 MIME 类型
func (s *MaterialServiceImpl) getContentType(fileType string) string {
	switch fileType {
	case "pdf":
		return "application/pdf"
	case "document":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "image":
		return "image/*"
	case "video":
		return "video/*"
	case "audio":
		return "audio/*"
	case "text":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// ======================= AI 处理相关方法 =======================

func (s *MaterialServiceImpl) ProcessMaterial(materialID uuid.UUID, userID uuid.UUID, processType string, options map[string]string) (*models.ProcessingResult, error) {
	// 1. 验证材料存在且属于该用户
	material, err := s.repo.GetByID(materialID)
	if err != nil {
		return nil, fmt.Errorf("material not found: %w", err)
	}
	if material.UserID != userID {
		return nil, fmt.Errorf("permission denied: material does not belong to user")
	}

	// 2. 检查是否已有相同类型的处理结果
	existingResult, err := s.processingRepo.GetByMaterialIDAndType(materialID, processType)
	if err == nil && existingResult.Status == models.ProcessingStatusCompleted {
		return existingResult, nil // 返回已有的结果
	}

	// 3. 生成任务ID
	taskID := uuid.New().String()

	// 4. 创建处理记录
	result := &models.ProcessingResult{
		MaterialID: materialID,
		TaskID:     taskID,
		Type:       processType,
		Status:     models.ProcessingStatusPending,
	}

	if err := s.processingRepo.Create(result); err != nil {
		return nil, fmt.Errorf("failed to create processing record: %w", err)
	}

	// 5. 异步调用AI服务 (TODO: 在后续实现中添加)
	go s.callAIService(material, result, processType, options)

	return result, nil
}

func (s *MaterialServiceImpl) GetProcessingResult(materialID uuid.UUID, processType string) (*models.ProcessingResult, error) {
	return s.processingRepo.GetByMaterialIDAndType(materialID, processType)
}

func (s *MaterialServiceImpl) ListProcessingResults(materialID uuid.UUID, page, pageSize int32) ([]*models.ProcessingResult, int64, error) {
	return s.processingRepo.GetByMaterialIDWithPagination(materialID, page, pageSize)
}

func (s *MaterialServiceImpl) UpdateProcessingResult(taskID string, status string, content string, metadata map[string]interface{}, errorMessage string) error {
	updates := map[string]interface{}{
		"status":  status,
		"content": content,
	}

	if metadata != nil {
		updates["metadata"] = metadata
	}

	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}

	return s.processingRepo.UpdateByTaskID(taskID, updates)
}

// callAIService 异步调用AI服务 (占位符，后续实现)
func (s *MaterialServiceImpl) callAIService(material *models.Material, result *models.ProcessingResult, processType string, options map[string]string) {
	// TODO: 实现AI服务调用
	// 1. 获取文件URL
	// 2. 根据processType调用相应的AI服务
	// 3. 更新处理结果

	// 暂时模拟处理完成
	s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusProcessing, "", nil, "")
	// 这里将来会实际调用AI服务
}
