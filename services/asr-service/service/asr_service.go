package service

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/RigelNana/arkstudy/services/asr-service/config"
	"github.com/RigelNana/arkstudy/services/asr-service/database"
	"github.com/RigelNana/arkstudy/services/asr-service/models"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

type ASRService struct {
	config       *config.Config
	openAIClient *openai.Client
}

func NewASRService(cfg *config.Config) *ASRService {
	// Initialize OpenAI client with custom config
	clientConfig := openai.DefaultConfig(cfg.OpenAIAPIKey)
	if cfg.OpenAIBaseURL != "" {
		clientConfig.BaseURL = cfg.OpenAIBaseURL
	}

	client := openai.NewClientWithConfig(clientConfig)

	return &ASRService{
		config:       cfg,
		openAIClient: client,
	}
}

// ProcessVideo processes a video file to extract audio and perform ASR
func (s *ASRService) ProcessVideo(req *models.ASRRequest) (*models.ASRResponse, error) {
	response := &models.ASRResponse{
		MaterialID:  req.MaterialID,
		UserID:      req.UserID,
		ProcessedAt: time.Now().Format(time.RFC3339),
		Success:     false,
	}

	// Step 1: Download video file (simulate for now)
	videoPath := filepath.Join(s.config.TempDir, fmt.Sprintf("%s_%s.mp4", req.MaterialID, uuid.New().String()[:8]))
	if err := s.downloadVideo(req.VideoURL, videoPath); err != nil {
		response.Message = "Failed to download video: " + err.Error()
		return response, err
	}
	defer os.Remove(videoPath)

	// Step 2: Extract audio using ffmpeg
	audioPath := strings.Replace(videoPath, ".mp4", "."+s.config.AudioFormat, 1)
	if err := s.extractAudio(videoPath, audioPath); err != nil {
		response.Message = "Failed to extract audio: " + err.Error()
		return response, err
	}
	defer os.Remove(audioPath)

	// Step 3: Transcribe audio using Whisper
	whisperResponse, err := s.transcribeAudio(audioPath, req.Language)
	if err != nil {
		response.Message = "Failed to transcribe audio: " + err.Error()
		return response, err
	}

	// Step 4: Process segments and store in database
	segments, err := s.processAndStoreSegments(whisperResponse, req.MaterialID, req.UserID)
	if err != nil {
		response.Message = "Failed to store segments: " + err.Error()
		return response, err
	}

	response.Segments = segments
	response.TotalDuration = whisperResponse.Duration
	response.Language = whisperResponse.Language
	response.Success = true
	response.Message = "ASR processing completed successfully"

	return response, nil
}

// downloadVideo downloads video from URL (placeholder implementation)
func (s *ASRService) downloadVideo(videoURL, outputPath string) error {
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// For now, just create a placeholder file
	// In real implementation, you would download from videoURL
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create video file: %w", err)
	}
	defer file.Close()

	log.Printf("Video download simulated for URL: %s", videoURL)
	return nil
}

// extractAudio extracts audio from video using ffmpeg
func (s *ASRService) extractAudio(videoPath, audioPath string) error {
	cmd := exec.Command(s.config.FFmpegBinaryPath,
		"-i", videoPath,
		"-vn",                  // no video
		"-acodec", "pcm_s16le", // audio codec
		"-ar", "16000", // sample rate
		"-ac", "1", // mono
		"-f", s.config.AudioFormat,
		audioPath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extraction failed: %w", err)
	}

	log.Printf("Audio extracted successfully: %s", audioPath)
	return nil
}

// transcribeAudio transcribes audio using OpenAI Whisper
func (s *ASRService) transcribeAudio(audioPath, language string) (*models.WhisperResponse, error) {
	audioFile, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioFile.Close()

	req := openai.AudioRequest{
		Model:    s.config.OpenAIModel,
		FilePath: audioPath,
		Format:   openai.AudioResponseFormatVerboseJSON,
	}

	if language != "" {
		req.Language = language
	}

	resp, err := s.openAIClient.CreateTranscription(nil, req)
	if err != nil {
		return nil, fmt.Errorf("whisper transcription failed: %w", err)
	}

	// Convert OpenAI response to our model
	whisperResp := &models.WhisperResponse{
		Task:     resp.Task,
		Language: resp.Language,
		Duration: resp.Duration,
		Text:     resp.Text,
	}

	// Convert segments
	for _, seg := range resp.Segments {
		whisperResp.Segments = append(whisperResp.Segments, models.WhisperSegment{
			ID:               seg.ID,
			Seek:             seg.Seek,
			Start:            seg.Start,
			End:              seg.End,
			Text:             seg.Text,
			Tokens:           seg.Tokens,
			Temperature:      seg.Temperature,
			AvgLogprob:       seg.AvgLogprob,
			CompressionRatio: seg.CompressionRatio,
			NoSpeechProb:     seg.NoSpeechProb,
		})
	}

	log.Printf("Audio transcribed successfully, found %d segments", len(whisperResp.Segments))
	return whisperResp, nil
}

// processAndStoreSegments processes Whisper segments and stores them in database
func (s *ASRService) processAndStoreSegments(whisperResp *models.WhisperResponse, materialID string, userID uuid.UUID) ([]models.ASRSegment, error) {
	var segments []models.ASRSegment

	for i, seg := range whisperResp.Segments {
		asrSegment := models.ASRSegment{
			MaterialID:   materialID,
			UserID:       userID,
			SegmentIndex: i,
			StartTime:    seg.Start,
			EndTime:      seg.End,
			Text:         strings.TrimSpace(seg.Text),
			Confidence:   &seg.AvgLogprob, // Use avg_logprob as confidence
			Language:     &whisperResp.Language,
		}

		// TODO: Generate embeddings for semantic search
		// For now, we'll skip embeddings and add them later

		segments = append(segments, asrSegment)
	}

	// Store segments in database
	if err := database.DB.Create(&segments).Error; err != nil {
		return nil, fmt.Errorf("failed to store segments in database: %w", err)
	}

	log.Printf("Stored %d ASR segments in database", len(segments))
	return segments, nil
}

// GetSegmentsByMaterialID retrieves ASR segments for a specific material
func (s *ASRService) GetSegmentsByMaterialID(materialID string) ([]models.ASRSegment, error) {
	var segments []models.ASRSegment

	if err := database.DB.Where("material_id = ?", materialID).
		Order("segment_index ASC").
		Find(&segments).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve segments: %w", err)
	}

	return segments, nil
}

// SearchSegments performs semantic search on ASR segments
func (s *ASRService) SearchSegments(req *models.SearchASRRequest) (*models.SearchASRResponse, error) {
	response := &models.SearchASRResponse{
		Query:   req.Query,
		TopK:    req.TopK,
		Success: false,
	}

	if req.TopK <= 0 {
		req.TopK = 5
	}

	// For now, implement simple text search
	// TODO: Implement vector similarity search later
	var segments []models.ASRSegment
	query := database.DB.Where("user_id = ?", req.UserID)

	if req.MaterialID != "" {
		query = query.Where("material_id = ?", req.MaterialID)
	}

	if err := query.Where("text ILIKE ?", "%"+req.Query+"%").
		Limit(req.TopK).
		Find(&segments).Error; err != nil {
		response.Message = "Search failed: " + err.Error()
		return response, fmt.Errorf("database search failed: %w", err)
	}

	// Convert to search results
	for _, segment := range segments {
		result := models.ASRSearchResult{
			Segment:   segment,
			Score:     0.8, // Placeholder score
			Relevance: "medium",
		}
		response.Results = append(response.Results, result)
	}

	response.Success = true
	response.Message = fmt.Sprintf("Found %d matching segments", len(response.Results))

	return response, nil
}
