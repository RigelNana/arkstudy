package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type ProcessingResult struct {
	Base
	MaterialID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"material_id"`
	TaskID       string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"task_id"`
	Type         string         `gorm:"type:varchar(50);not null;index" json:"type"` // OCR, ASR, LLM_ANALYSIS
	Status       string         `gorm:"type:varchar(50);not null;index;default:'pending'" json:"status"`
	Content      string         `gorm:"type:text" json:"content"`
	Metadata     datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	ErrorMessage string         `gorm:"type:text" json:"error_message"`

	// 关联关系
	Material Material `gorm:"foreignKey:MaterialID" json:"material,omitempty"`
}

func (ProcessingResult) TableName() string {
	return "processing_results"
}

// 处理类型常量
const (
	ProcessingTypeOCR         = "OCR"
	ProcessingTypeASR         = "ASR"
	ProcessingTypeLLMAnalysis = "LLM_ANALYSIS"
)

// 处理状态常量
const (
	ProcessingStatusPending    = "pending"
	ProcessingStatusProcessing = "processing"
	ProcessingStatusCompleted  = "completed"
	ProcessingStatusFailed     = "failed"
)
