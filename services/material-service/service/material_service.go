package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	aipb "github.com/RigelNana/arkstudy/proto/ai"
	llmpb "github.com/RigelNana/arkstudy/proto/llm"
	"github.com/RigelNana/arkstudy/services/material-service/config"
	"github.com/RigelNana/arkstudy/services/material-service/models"
	"github.com/RigelNana/arkstudy/services/material-service/repository"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	kafka "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
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
	repo                     repository.MaterialRepository
	processingRepo           repository.ProcessingResultRepository
	minioClient              *minio.Client
	config                   *config.Config
	kafkaWriter              *kafka.Writer
	textExtractedKafkaWriter *kafka.Writer
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

	log.Printf("Initializing MaterialService with Kafka writer...")
	kafkaWriter := newFileProcessingKafkaWriter(cfg)
	if kafkaWriter != nil {
		log.Printf("Kafka writer initialized successfully")
	} else {
		log.Printf("Kafka writer is nil - not configured")
	}

	textExtractedKafkaWriter := newTextExtractedKafkaWriter(cfg)
	if textExtractedKafkaWriter != nil {
		log.Printf("Text extracted Kafka writer initialized successfully")
	} else {
		log.Printf("Text extracted Kafka writer is nil - not configured")
	}

	return &MaterialServiceImpl{
		repo:                     repo,
		processingRepo:           processingRepo,
		minioClient:              minioClient,
		config:                   cfg,
		kafkaWriter:              kafkaWriter,
		textExtractedKafkaWriter: textExtractedKafkaWriter,
	}, nil
}

// newKafkaWriter creates a Kafka writer if brokers and topic are configured; otherwise returns nil.
func newKafkaWriter(cfg *config.Config) *kafka.Writer {
	brokers := strings.TrimSpace(cfg.Database.KafkaBrokers)
	topic := strings.TrimSpace(cfg.Database.KafkaTopicOCRReqs)
	if brokers == "" || topic == "" {
		return nil
	}
	// support comma-separated brokers
	var bs []string
	for _, b := range strings.Split(brokers, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			bs = append(bs, b)
		}
	}
	if len(bs) == 0 {
		return nil
	}
	return &kafka.Writer{
		Addr:         kafka.TCP(bs...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
	}
}

// newFileProcessingKafkaWriter 创建文件处理的 Kafka writer
func newFileProcessingKafkaWriter(cfg *config.Config) *kafka.Writer {
	brokers := strings.TrimSpace(cfg.Database.KafkaBrokers)
	topic := strings.TrimSpace(cfg.Database.KafkaTopicFileProcess)
	log.Printf("Kafka config: brokers=%s, topic=%s", brokers, topic)
	if brokers == "" || topic == "" {
		log.Printf("Kafka writer not created: missing brokers or topic")
		return nil
	}
	// support comma-separated brokers
	var bs []string
	for _, b := range strings.Split(brokers, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			bs = append(bs, b)
		}
	}
	if len(bs) == 0 {
		return nil
	}
	return &kafka.Writer{
		Addr:         kafka.TCP(bs...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
	}
}

func newTextExtractedKafkaWriter(cfg *config.Config) *kafka.Writer {
	brokers := strings.TrimSpace(cfg.Database.KafkaBrokers)
	topic := strings.TrimSpace(cfg.Database.KafkaTopicTextExtracted)
	if brokers == "" || topic == "" {
		return nil
	}
	var bs []string
	for _, b := range strings.Split(brokers, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			bs = append(bs, b)
		}
	}
	if len(bs) == 0 {
		return nil
	}
	return &kafka.Writer{
		Addr:         kafka.TCP(bs...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
	}
}

// ocrJob is the message schema sent to Kafka for OCR tasks.
type ocrJob struct {
	TaskID     string            `json:"task_id"`
	MaterialID string            `json:"material_id"`
	UserID     string            `json:"user_id"`
	FileURL    string            `json:"file_url"`
	FileType   string            `json:"file_type"`
	Options    map[string]string `json:"options,omitempty"`
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

	// 发送 Kafka 消息给 llm-service 进行文档处理
	log.Printf("Attempting to send file processing message for material %s", material.ID.String())
	if material.FileType == "pdf" || material.FileType == "document" || material.FileType == "image" {
		if err := s.sendOcrRequestMessage(material, userID); err != nil {
			log.Printf("Warning: failed to send ocr request message: %v", err)
		} else {
			log.Printf("Successfully sent ocr request message for material %s", material.ID.String())
		}
	} else if material.FileType == "text" {
		if err := s.sendTextExtractedMessage(material, userID); err != nil {
			log.Printf("Warning: failed to send text extracted message: %v", err)
		} else {
			log.Printf("Successfully sent text extracted message for material %s", material.ID.String())
		}
	} else {
		if err := s.sendFileProcessingMessage(material, userID); err != nil {
			// 记录错误但不影响上传成功
			log.Printf("Warning: failed to send file processing message: %v", err)
		} else {
			log.Printf("Successfully sent file processing message for material %s", material.ID.String())
		}
	}

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

	// 5. 触发异步处理
	if processType == models.ProcessingTypeOCR && s.textExtractedKafkaWriter != nil {
		// 5.1 生成短期下载 URL
		urlStr, err := s.GetFileURL(material, 15*time.Minute)
		if err != nil {
			_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("presign: %v", err))
			return result, nil
		}
		// 5.2 发送 Kafka 消息
		job := ocrJob{
			TaskID:     taskID,
			MaterialID: materialID.String(),
			UserID:     userID.String(),
			FileURL:    urlStr,
			FileType:   material.FileType,
			Options:    options,
		}
		// encode JSON
		payload, err := json.Marshal(job)
		if err != nil {
			_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("marshal job: %v", err))
			return result, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = s.kafkaWriter.WriteMessages(ctx, kafka.Message{Value: payload})
		if err != nil {
			_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("kafka publish: %v", err))
			return result, nil
		}
		// 更新状态为 processing，等待 ocr-service 回调
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusProcessing, "", map[string]interface{}{"dispatched": true}, "")
		return result, nil
	}

	// 回退：直接调用内部同步编排（gRPC 轮询）
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
	// 更新处理状态为 processing
	_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusProcessing, "", nil, "")

	switch processType {
	case models.ProcessingTypeOCR:
		s.handleOCR(material, result)
		return
	case models.ProcessingTypeASR:
		// 预留：交给独立 asr-service
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, "ASR handled by asr-service")
		return
	default:
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, "unsupported processing type")
		return
	}
}

func (s *MaterialServiceImpl) handleOCR(material *models.Material, result *models.ProcessingResult) {
	// 1) 生成短期下载 URL
	urlStr, err := s.GetFileURL(material, 15*time.Minute)
	if err != nil {
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("presign: %v", err))
		return
	}

	// 2) 连接 ocr-service
	addr := s.config.Database.OCRGRPCAddr
	if addr == "" {
		addr = "localhost:50055"
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("dial ocr: %v", err))
		return
	}
	defer conn.Close()
	ocr := aipb.NewAIServiceClient(conn)

	// 3) 发起 OCR 任务
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = ocr.ProcessOCR(ctx, &aipb.OCRRequest{TaskId: result.TaskID, FileUrl: urlStr, FileType: material.FileType})
	if err != nil {
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("process ocr: %v", err))
		return
	}

	// 4) 轮询任务状态
	deadline := time.Now().Add(10 * time.Minute)
	var finalText string
	for time.Now().Before(deadline) {
		stx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		status, err := ocr.GetTaskStatus(stx, &aipb.TaskStatusRequest{TaskId: result.TaskID})
		cancel2()
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if status.Status == aipb.TaskStatus_COMPLETED {
			// 再拉取一次最终结果（复用 ProcessOCR 返回完成结果的能力）
			rctx, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
			resp, err := ocr.ProcessOCR(rctx, &aipb.OCRRequest{TaskId: result.TaskID})
			cancel3()
			if err != nil {
				_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("fetch ocr result: %v", err))
				return
			}
			finalText = resp.GetText()
			break
		}
		if status.Status == aipb.TaskStatus_FAILED {
			_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, status.GetErrorMessage())
			return
		}
		time.Sleep(2 * time.Second)
	}
	if finalText == "" {
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, "ocr timeout or empty")
		return
	}

	// 5) 将 OCR 文本拆分为 chunk（简单策略：按换行 / 500 字一段）并入库向量
	chunks := splitTextToChunks(finalText)
	if len(chunks) == 0 {
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, "no chunks")
		return
	}

	llmAddr := s.config.Database.LLMGRPCAddr
	if llmAddr == "" {
		llmAddr = "localhost:50054"
	}
	lconn, err := grpc.Dial(llmAddr, grpc.WithInsecure())
	if err != nil {
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("dial llm: %v", err))
		return
	}
	defer lconn.Close()
	llm := llmpb.NewLLMServiceClient(lconn)

	ureq := &llmpb.UpsertChunksRequest{UserId: material.UserID.String(), MaterialId: material.ID.String()}
	for i, c := range chunks {
		ureq.Chunks = append(ureq.Chunks, &llmpb.UpsertChunkItem{Content: c, Page: int32(i), Metadata: map[string]string{"source": material.FileType}})
	}
	uctx, ucancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ucancel()
	uresp, err := llm.UpsertChunks(uctx, ureq)
	if err != nil {
		_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusFailed, "", nil, fmt.Sprintf("upsert chunks: %v", err))
		return
	}
	_ = s.UpdateProcessingResult(result.TaskID, models.ProcessingStatusCompleted, "embedded", map[string]interface{}{"chunks": uresp.Inserted}, "")
}

func splitTextToChunks(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	// 简单切分：按换行聚合，保证每段<=500字符
	maxLen := 500
	var out []string
	var cur strings.Builder
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if cur.Len()+len(line)+1 > maxLen {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n")
		}
		cur.WriteString(line)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func (s *MaterialServiceImpl) sendOcrRequestMessage(material *models.Material, userID uuid.UUID) error {
	if s.kafkaWriter == nil {
		return fmt.Errorf("kafka writer not configured")
	}

	// 构建消息
	message := map[string]interface{}{
		"task_id":     uuid.New().String(),
		"material_id": material.ID.String(),
		"user_id":     userID.String(),
		"file_url":    fmt.Sprintf("materials/%s/%s", material.MinioBucket, material.MinioObjectName),
		"file_type":   material.FileType,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 发送到 Kafka
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(material.ID.String()),
		Value: messageBytes,
	})

	if err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}
	return nil
}

func (s *MaterialServiceImpl) sendTextExtractedMessage(material *models.Material, userID uuid.UUID) error {
	if s.textExtractedKafkaWriter == nil {
		return fmt.Errorf("text extracted kafka writer not configured")
	}

	// 读取文件内容
	ctx := context.Background()
	obj, err := s.minioClient.GetObject(ctx, material.MinioBucket, material.MinioObjectName, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get object from minio: %w", err)
	}
	defer obj.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(obj); err != nil {
		return fmt.Errorf("failed to read object content: %w", err)
	}
	content := buf.String()

	// 构建消息
	message := map[string]interface{}{
		"material_id": material.ID.String(),
		"user_id":     userID.String(),
		"text":        content,
		"source":      "text",
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 发送到 Kafka
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = s.textExtractedKafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(material.ID.String()),
		Value: messageBytes,
	})

	if err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	return nil
}

// sendFileProcessingMessage 发送文件处理消息到 Kafka
func (s *MaterialServiceImpl) sendFileProcessingMessage(material *models.Material, userID uuid.UUID) error {
	if s.kafkaWriter == nil {
		return fmt.Errorf("kafka writer not configured")
	}

	// 构建消息
	message := map[string]interface{}{
		"file_id":   material.ID.String(),
		"file_path": fmt.Sprintf("materials/%s/%s", material.MinioBucket, material.MinioObjectName),
		"user_id":   userID.String(),
		"file_type": material.FileType,
		"title":     material.Title,
		"timestamp": time.Now().Unix(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 发送到 Kafka
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = s.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(material.ID.String()),
		Value: messageBytes,
	})

	if err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}

	return nil
}
