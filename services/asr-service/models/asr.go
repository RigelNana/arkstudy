package models

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ASRSegment represents a single transcribed segment from audio/video
type ASRSegment struct {
	Base
	MaterialID   string          `gorm:"type:varchar(255);not null;index" json:"material_id"`
	UserID       uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
	SegmentIndex int             `gorm:"not null" json:"segment_index"`
	StartTime    float64         `gorm:"not null" json:"start_time"`
	EndTime      float64         `gorm:"not null" json:"end_time"`
	Text         string          `gorm:"type:text;not null" json:"text"`
	Confidence   *float64        `gorm:"type:float" json:"confidence,omitempty"`
	Embedding    pq.Float64Array `gorm:"type:float[]" json:"embedding,omitempty"`
	Language     *string         `gorm:"type:varchar(10)" json:"language,omitempty"`
}

// TableName sets the table name for ASRSegment
func (ASRSegment) TableName() string {
	return "asr_segments"
}

// ASRRequest represents the request to process video/audio
type ASRRequest struct {
	MaterialID string    `json:"material_id" binding:"required"`
	UserID     uuid.UUID `json:"user_id" binding:"required"`
	VideoURL   string    `json:"video_url" binding:"required"`
	Language   string    `json:"language,omitempty"` // Optional language hint
}

// ASRResponse represents the response from ASR processing
type ASRResponse struct {
	MaterialID    string       `json:"material_id"`
	UserID        uuid.UUID    `json:"user_id"`
	Segments      []ASRSegment `json:"segments"`
	TotalDuration float64      `json:"total_duration"`
	Language      string       `json:"language"`
	ProcessedAt   string       `json:"processed_at"`
	Success       bool         `json:"success"`
	Message       string       `json:"message"`
}

// WhisperSegment represents the response from OpenAI Whisper API
type WhisperSegment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

// WhisperResponse represents the full response from Whisper API
type WhisperResponse struct {
	Task     string           `json:"task"`
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Segments []WhisperSegment `json:"segments"`
	Text     string           `json:"text"`
}

// AudioExtractionRequest represents request for audio extraction
type AudioExtractionRequest struct {
	VideoPath  string `json:"video_path"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"`
	Quality    string `json:"quality,omitempty"`
}

// AudioExtractionResponse represents response from audio extraction
type AudioExtractionResponse struct {
	Success    bool    `json:"success"`
	OutputPath string  `json:"output_path"`
	Duration   float64 `json:"duration"`
	FileSize   int64   `json:"file_size"`
	Message    string  `json:"message"`
}

// SearchASRRequest represents the request to search ASR segments
type SearchASRRequest struct {
	Query      string    `json:"query" binding:"required"`
	UserID     uuid.UUID `json:"user_id" binding:"required"`
	MaterialID string    `json:"material_id,omitempty"`
	TopK       int       `json:"top_k,omitempty"`
	MinScore   float64   `json:"min_score,omitempty"`
}

// SearchASRResponse represents the response from ASR search
type SearchASRResponse struct {
	Query   string            `json:"query"`
	Results []ASRSearchResult `json:"results"`
	TopK    int               `json:"top_k"`
	Success bool              `json:"success"`
	Message string            `json:"message"`
}

// ASRSearchResult represents a single search result
type ASRSearchResult struct {
	Segment   ASRSegment `json:"segment"`
	Score     float64    `json:"score"`
	Relevance string     `json:"relevance"` // "high", "medium", "low"
}
