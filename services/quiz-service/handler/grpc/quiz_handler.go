package grpc

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	pb "github.com/RigelNana/arkstudy/proto/quiz"
	"github.com/RigelNana/arkstudy/quiz-service/models"
	"github.com/RigelNana/arkstudy/quiz-service/repository"
	"github.com/RigelNana/arkstudy/quiz-service/service"
)

type QuizGRPCHandler struct {
	pb.UnimplementedQuizServiceServer
	quizService    *service.QuizService
	quizRepository *repository.QuizRepository
	logger         *logrus.Logger
}

func NewQuizGRPCHandler(quizService *service.QuizService, quizRepository *repository.QuizRepository, logger *logrus.Logger) *QuizGRPCHandler {
	return &QuizGRPCHandler{
		quizService:    quizService,
		quizRepository: quizRepository,
		logger:         logger,
	}
}

// 生成题目
func (h *QuizGRPCHandler) GenerateQuiz(ctx context.Context, req *pb.GenerateQuizRequest) (*pb.GenerateQuizResponse, error) {
	h.logger.Infof("收到生成题目请求，材料ID: %s, 用户ID: %s", req.MaterialId, req.UserId)

	// 转换请求参数
	questionTypes := make([]models.QuestionType, len(req.Types))
	for i, t := range req.Types {
		questionTypes[i] = models.QuestionType(t)
	}

	genReq := &service.QuestionGenerationRequest{
		MaterialContent: "", // 将从LLM服务获取材料内容
		MaterialID:      req.MaterialId,
		UserID:          req.UserId,
		QuestionTypes:   questionTypes,
		Difficulty:      models.DifficultyLevel(req.Difficulty),
		Count:           int(req.Count),
		KnowledgePoints: req.KnowledgePoints,
	}

	// 生成题目
	generatedQuestions, err := h.quizService.GenerateQuestions(ctx, genReq)
	if err != nil {
		h.logger.Errorf("生成题目失败: %v", err)
		return &pb.GenerateQuizResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// 转换为数据库模型并保存
	var questions []*models.Question
	for _, gq := range generatedQuestions {
		question := h.quizService.ConvertToQuestionModel(gq, req.MaterialId, req.UserId)
		questions = append(questions, question)
	}

	if err := h.quizRepository.CreateQuestions(questions); err != nil {
		h.logger.Errorf("保存题目失败: %v", err)
		return &pb.GenerateQuizResponse{
			Success: false,
			Message: "保存题目失败",
		}, nil
	}

	// 转换为响应格式
	var pbQuestions []*pb.Question
	for _, q := range questions {
		pbQ, err := h.convertToPBQuestion(q)
		if err != nil {
			h.logger.Errorf("转换题目格式失败: %v", err)
			continue
		}
		pbQuestions = append(pbQuestions, pbQ)
	}

	return &pb.GenerateQuizResponse{
		Success:   true,
		Message:   "题目生成成功",
		Questions: pbQuestions,
	}, nil
}

// 获取题目
func (h *QuizGRPCHandler) GetQuiz(ctx context.Context, req *pb.GetQuizRequest) (*pb.GetQuizResponse, error) {
	question, err := h.quizRepository.GetQuestionByID(req.QuestionId)
	if err != nil {
		return &pb.GetQuizResponse{
			Success: false,
			Message: "题目不存在",
		}, nil
	}

	pbQuestion, err := h.convertToPBQuestion(question)
	if err != nil {
		return &pb.GetQuizResponse{
			Success: false,
			Message: "转换题目格式失败",
		}, nil
	}

	return &pb.GetQuizResponse{
		Success:  true,
		Message:  "获取成功",
		Question: pbQuestion,
	}, nil
}

// 获取题目列表
func (h *QuizGRPCHandler) ListQuizzes(ctx context.Context, req *pb.ListQuizzesRequest) (*pb.ListQuizzesResponse, error) {
	var questionType *models.QuestionType
	if req.Type != pb.QuestionType_MULTIPLE_CHOICE && req.Type != 0 {
		t := models.QuestionType(req.Type)
		questionType = &t
	}

	var difficulty *models.DifficultyLevel
	if req.Difficulty != pb.DifficultyLevel_EASY && req.Difficulty != 0 {
		d := models.DifficultyLevel(req.Difficulty)
		difficulty = &d
	}

	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 10
	}

	questions, total, err := h.quizRepository.ListQuestions(req.UserId, req.MaterialId, questionType, difficulty, page, pageSize)
	if err != nil {
		return &pb.ListQuizzesResponse{
			Success: false,
			Message: "获取题目列表失败",
		}, nil
	}

	var pbQuestions []*pb.Question
	for _, q := range questions {
		pbQ, err := h.convertToPBQuestion(q)
		if err != nil {
			h.logger.Errorf("转换题目格式失败: %v", err)
			continue
		}
		pbQuestions = append(pbQuestions, pbQ)
	}

	return &pb.ListQuizzesResponse{
		Success:   true,
		Message:   "获取成功",
		Questions: pbQuestions,
		Total:     int32(total),
		Page:      int32(page),
		PageSize:  int32(pageSize),
	}, nil
}

// 提交答案
func (h *QuizGRPCHandler) SubmitAnswer(ctx context.Context, req *pb.SubmitAnswerRequest) (*pb.SubmitAnswerResponse, error) {
	// 获取题目
	question, err := h.quizRepository.GetQuestionByID(req.QuestionId)
	if err != nil {
		return &pb.SubmitAnswerResponse{
			Success: false,
			Message: "题目不存在",
		}, nil
	}

	// 评估答案
	score, evaluationExplanation, err := h.quizService.EvaluateSubjectiveAnswer(ctx, question, req.Answer, req.UserId)
	if err != nil {
		h.logger.Errorf("评估答案失败: %v", err)
		return &pb.SubmitAnswerResponse{
			Success: false,
			Message: "答案评估失败",
		}, nil
	}

	isCorrect := score >= 0.6 // 设置及格线为60%

	// 保存答题记录
	userAnswer := &models.UserAnswer{
		AnswerID:   uuid.New().String(),
		QuestionID: req.QuestionId,
		UserID:     req.UserId,
		Answer:     req.Answer,
		IsCorrect:  isCorrect,
		Score:      score,
	}

	if err := h.quizRepository.CreateUserAnswer(userAnswer); err != nil {
		h.logger.Errorf("保存答题记录失败: %v", err)
	}

	// 更新知识点统计
	var knowledgePoints []string
	if question.KnowledgePoints != "" {
		json.Unmarshal([]byte(question.KnowledgePoints), &knowledgePoints)
		for _, kp := range knowledgePoints {
			h.quizRepository.CalculateAndUpdateKnowledgeStats(req.UserId, question.MaterialID, kp)
		}
	}

	return &pb.SubmitAnswerResponse{
		Success:       true,
		Message:       "答案提交成功",
		IsCorrect:     isCorrect,
		Score:         score,
		CorrectAnswer: question.CorrectAnswer,
		Explanation:   evaluationExplanation,
	}, nil
}

// 获取用户答题历史
func (h *QuizGRPCHandler) GetUserQuizHistory(ctx context.Context, req *pb.GetUserQuizHistoryRequest) (*pb.GetUserQuizHistoryResponse, error) {
	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 10
	}

	answers, total, err := h.quizRepository.GetUserAnswerHistory(req.UserId, page, pageSize)
	if err != nil {
		return &pb.GetUserQuizHistoryResponse{
			Success: false,
			Message: "获取答题历史失败",
		}, nil
	}

	var pbAnswers []*pb.UserAnswer
	for _, answer := range answers {
		pbAnswer := &pb.UserAnswer{
			AnswerId:   answer.AnswerID,
			QuestionId: answer.QuestionID,
			UserId:     answer.UserID,
			Answer:     answer.Answer,
			IsCorrect:  answer.IsCorrect,
			Score:      answer.Score,
			AnsweredAt: answer.AnsweredAt.Format("2006-01-02 15:04:05"),
		}
		pbAnswers = append(pbAnswers, pbAnswer)
	}

	return &pb.GetUserQuizHistoryResponse{
		Success: true,
		Message: "获取成功",
		Answers: pbAnswers,
		Total:   int32(total),
	}, nil
}

// 获取知识点统计
func (h *QuizGRPCHandler) GetKnowledgeStats(ctx context.Context, req *pb.GetKnowledgeStatsRequest) (*pb.GetKnowledgeStatsResponse, error) {
	stats, err := h.quizRepository.GetKnowledgeStats(req.UserId, req.MaterialId)
	if err != nil {
		return &pb.GetKnowledgeStatsResponse{
			Success: false,
			Message: "获取知识点统计失败",
		}, nil
	}

	var pbStats []*pb.KnowledgePointStats
	var totalCorrect, totalQuestions int
	for _, stat := range stats {
		pbStat := &pb.KnowledgePointStats{
			KnowledgePoint: stat.KnowledgePoint,
			TotalQuestions: int32(stat.TotalQuestions),
			CorrectAnswers: int32(stat.CorrectAnswers),
			AccuracyRate:   stat.AccuracyRate,
			AvgDifficulty:  pb.DifficultyLevel(stat.AvgDifficulty),
		}
		pbStats = append(pbStats, pbStat)
		totalCorrect += stat.CorrectAnswers
		totalQuestions += stat.TotalQuestions
	}

	overallAccuracy := float32(0)
	if totalQuestions > 0 {
		overallAccuracy = float32(totalCorrect) / float32(totalQuestions)
	}

	return &pb.GetKnowledgeStatsResponse{
		Success:         true,
		Message:         "获取成功",
		Stats:           pbStats,
		OverallAccuracy: overallAccuracy,
	}, nil
}

// 辅助函数：转换为protobuf格式
func (h *QuizGRPCHandler) convertToPBQuestion(q *models.Question) (*pb.Question, error) {
	var options []string
	if q.Options != "" {
		json.Unmarshal([]byte(q.Options), &options)
	}

	var knowledgePoints []string
	if q.KnowledgePoints != "" {
		json.Unmarshal([]byte(q.KnowledgePoints), &knowledgePoints)
	}

	return &pb.Question{
		QuestionId:      q.QuestionID,
		Type:            pb.QuestionType(q.Type),
		Content:         q.Content,
		Options:         options,
		CorrectAnswer:   q.CorrectAnswer,
		Explanation:     q.Explanation,
		Difficulty:      pb.DifficultyLevel(q.Difficulty),
		KnowledgePoints: knowledgePoints,
		MaterialId:      q.MaterialID,
		CreatedAt:       q.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// 辅助函数：检查答案
func (h *QuizGRPCHandler) checkAnswer(question *models.Question, userAnswer string) bool {
	switch question.Type {
	case models.MultipleChoice, models.TrueFalse:
		return question.CorrectAnswer == userAnswer
	case models.FillBlank:
		// 简单的字符串匹配，可以后续优化为模糊匹配
		return question.CorrectAnswer == userAnswer
	case models.ShortAnswer, models.Essay:
		// 对于主观题，这里可以集成AI评分
		return true // 暂时返回true，需要人工或AI评分
	default:
		return false
	}
}
