package repository

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/RigelNana/arkstudy/quiz-service/models"
)

type QuizRepository struct {
	db *gorm.DB
}

func NewQuizRepository(dsn string) (*QuizRepository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// 自动迁移表结构 - 先创建独立表，再创建依赖表
	// 分别迁移每个表，避免外键依赖问题
	err = db.Migrator().AutoMigrate(&models.Question{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate Question table: %v", err)
	}

	err = db.Migrator().AutoMigrate(&models.KnowledgePointStats{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate KnowledgePointStats table: %v", err)
	}

	err = db.Migrator().AutoMigrate(&models.UserAnswer{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate UserAnswer table: %v", err)
	}

	return &QuizRepository{db: db}, nil
}

// 创建题目
func (r *QuizRepository) CreateQuestion(question *models.Question) error {
	return r.db.Create(question).Error
}

// 批量创建题目
func (r *QuizRepository) CreateQuestions(questions []*models.Question) error {
	return r.db.CreateInBatches(questions, 100).Error
}

// 根据ID获取题目
func (r *QuizRepository) GetQuestionByID(questionID string) (*models.Question, error) {
	var question models.Question
	err := r.db.Where("question_id = ?", questionID).First(&question).Error
	if err != nil {
		return nil, err
	}
	return &question, nil
}

// 获取题目列表
func (r *QuizRepository) ListQuestions(userID, materialID string, questionType *models.QuestionType, difficulty *models.DifficultyLevel, page, pageSize int) ([]*models.Question, int64, error) {
	var questions []*models.Question
	var total int64

	query := r.db.Model(&models.Question{})

	if userID != "" {
		query = query.Where("creator_id = ?", userID)
	}
	if materialID != "" {
		query = query.Where("material_id = ?", materialID)
	}
	if questionType != nil {
		query = query.Where("type = ?", *questionType)
	}
	if difficulty != nil {
		query = query.Where("difficulty = ?", *difficulty)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&questions).Error; err != nil {
		return nil, 0, err
	}

	return questions, total, nil
}

// 创建用户答题记录
func (r *QuizRepository) CreateUserAnswer(answer *models.UserAnswer) error {
	return r.db.Create(answer).Error
}

// 获取用户答题记录
func (r *QuizRepository) GetUserAnswer(questionID, userID string) (*models.UserAnswer, error) {
	var answer models.UserAnswer
	err := r.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&answer).Error
	if err != nil {
		return nil, err
	}
	return &answer, nil
}

// 获取用户答题历史
func (r *QuizRepository) GetUserAnswerHistory(userID string, page, pageSize int) ([]*models.UserAnswer, int64, error) {
	var answers []*models.UserAnswer
	var total int64

	query := r.db.Model(&models.UserAnswer{}).Where("user_id = ?", userID)

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询，包含题目信息
	offset := (page - 1) * pageSize
	if err := query.Preload("Question").Offset(offset).Limit(pageSize).Order("answered_at DESC").Find(&answers).Error; err != nil {
		return nil, 0, err
	}

	return answers, total, nil
}

// 获取知识点统计
func (r *QuizRepository) GetKnowledgeStats(userID, materialID string) ([]*models.KnowledgePointStats, error) {
	var stats []*models.KnowledgePointStats

	query := r.db.Model(&models.KnowledgePointStats{}).Where("user_id = ?", userID)
	if materialID != "" {
		query = query.Where("material_id = ?", materialID)
	}

	if err := query.Find(&stats).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// 更新知识点统计
func (r *QuizRepository) UpdateKnowledgeStats(stats *models.KnowledgePointStats) error {
	return r.db.Save(stats).Error
}

// 计算并更新知识点统计
func (r *QuizRepository) CalculateAndUpdateKnowledgeStats(userID, materialID, knowledgePoint string) error {
	// 查询该知识点的所有答题记录
	var answers []models.UserAnswer
	query := r.db.Table("user_answers").
		Joins("JOIN questions ON user_answers.question_id = questions.question_id").
		Where("user_answers.user_id = ? AND questions.knowledge_points LIKE ?", userID, "%"+knowledgePoint+"%")

	if materialID != "" {
		query = query.Where("questions.material_id = ?", materialID)
	}

	if err := query.Find(&answers).Error; err != nil {
		return err
	}

	if len(answers) == 0 {
		return nil
	}

	// 计算统计数据
	totalQuestions := len(answers)
	correctAnswers := 0
	for _, answer := range answers {
		if answer.IsCorrect {
			correctAnswers++
		}
	}

	accuracyRate := float32(correctAnswers) / float32(totalQuestions)

	// 查找或创建统计记录
	var stats models.KnowledgePointStats
	err := r.db.Where("user_id = ? AND material_id = ? AND knowledge_point = ?", userID, materialID, knowledgePoint).First(&stats).Error
	if err == gorm.ErrRecordNotFound {
		stats = models.KnowledgePointStats{
			UserID:         userID,
			MaterialID:     materialID,
			KnowledgePoint: knowledgePoint,
		}
	} else if err != nil {
		return err
	}

	stats.TotalQuestions = totalQuestions
	stats.CorrectAnswers = correctAnswers
	stats.AccuracyRate = accuracyRate

	return r.db.Save(&stats).Error
}
