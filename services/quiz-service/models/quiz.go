package models

import (
	"time"

	"gorm.io/gorm"
)

// 题目类型枚举
type QuestionType int

const (
	MultipleChoice QuestionType = iota
	FillBlank
	ShortAnswer
	TrueFalse
	Essay
)

func (qt QuestionType) String() string {
	switch qt {
	case MultipleChoice:
		return "multiple_choice"
	case FillBlank:
		return "fill_blank"
	case ShortAnswer:
		return "short_answer"
	case TrueFalse:
		return "true_false"
	case Essay:
		return "essay"
	default:
		return "unknown"
	}
}

// 难度级别枚举
type DifficultyLevel int

const (
	Easy DifficultyLevel = iota
	Medium
	Hard
)

func (dl DifficultyLevel) String() string {
	switch dl {
	case Easy:
		return "easy"
	case Medium:
		return "medium"
	case Hard:
		return "hard"
	default:
		return "unknown"
	}
}

// 基础模型
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// 题目模型
type Question struct {
	BaseModel
	QuestionID      string          `gorm:"uniqueIndex;size:255" json:"question_id"`
	Type            QuestionType    `gorm:"type:int" json:"type"`
	Content         string          `gorm:"type:text" json:"content"`
	Options         string          `gorm:"type:text" json:"options"` // JSON格式存储选项
	CorrectAnswer   string          `gorm:"type:text" json:"correct_answer"`
	Explanation     string          `gorm:"type:text" json:"explanation"`
	Difficulty      DifficultyLevel `gorm:"type:int" json:"difficulty"`
	KnowledgePoints string          `gorm:"type:text" json:"knowledge_points"` // JSON格式存储知识点
	MaterialID      string          `gorm:"size:255;index" json:"material_id"`
	CreatorID       string          `gorm:"size:255;index" json:"creator_id"`
}

// 用户答题记录模型
type UserAnswer struct {
	BaseModel
	AnswerID   string    `gorm:"uniqueIndex;size:255" json:"answer_id"`
	QuestionID string    `gorm:"size:255;index" json:"question_id"`
	UserID     string    `gorm:"size:255;index" json:"user_id"`
	Answer     string    `gorm:"type:text" json:"answer"`
	IsCorrect  bool      `json:"is_correct"`
	Score      float32   `json:"score"`
	AnsweredAt time.Time `json:"answered_at"`
}

// 知识点统计模型
type KnowledgePointStats struct {
	BaseModel
	UserID         string          `gorm:"size:255;index" json:"user_id"`
	MaterialID     string          `gorm:"size:255;index" json:"material_id"`
	KnowledgePoint string          `gorm:"size:255" json:"knowledge_point"`
	TotalQuestions int             `json:"total_questions"`
	CorrectAnswers int             `json:"correct_answers"`
	AccuracyRate   float32         `json:"accuracy_rate"`
	AvgDifficulty  DifficultyLevel `gorm:"type:int" json:"avg_difficulty"`
}

// 表名设置
func (Question) TableName() string {
	return "questions"
}

func (UserAnswer) TableName() string {
	return "user_answers"
}

func (KnowledgePointStats) TableName() string {
	return "knowledge_point_stats"
}
