package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	pb "github.com/RigelNana/arkstudy/proto/quiz"
)

type QuizHandler struct {
	quizClient pb.QuizServiceClient
	logger     *logrus.Logger
}

func NewQuizHandler(quizServiceAddr string, logger *logrus.Logger) *QuizHandler {
	conn, err := grpc.Dial(quizServiceAddr, grpc.WithInsecure())
	if err != nil {
		logger.Fatalf("连接quiz服务失败: %v", err)
	}

	client := pb.NewQuizServiceClient(conn)
	return &QuizHandler{
		quizClient: client,
		logger:     logger,
	}
}

// 生成题目请求结构
type GenerateQuizRequest struct {
	MaterialID      string   `json:"material_id" binding:"required"`
	QuestionTypes   []int32  `json:"question_types"`
	Difficulty      int32    `json:"difficulty"`
	Count           int32    `json:"count"`
	KnowledgePoints []string `json:"knowledge_points"`
}

// 提交答案请求结构
type SubmitAnswerRequest struct {
	Answer string `json:"answer" binding:"required"`
}

// 生成题目
func (h *QuizHandler) GenerateQuiz(c *gin.Context) {
	var req GenerateQuizRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	// 设置默认值
	if req.Count == 0 {
		req.Count = 5
	}
	if len(req.QuestionTypes) == 0 {
		req.QuestionTypes = []int32{0, 1, 2} // 选择题、填空题、简答题
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 转换题目类型
	types := make([]pb.QuestionType, len(req.QuestionTypes))
	for i, t := range req.QuestionTypes {
		types[i] = pb.QuestionType(t)
	}

	grpcReq := &pb.GenerateQuizRequest{
		MaterialId:      req.MaterialID,
		UserId:          userID.(string),
		Types:           types,
		Difficulty:      pb.DifficultyLevel(req.Difficulty),
		Count:           req.Count,
		KnowledgePoints: req.KnowledgePoints,
	}

	resp, err := h.quizClient.GenerateQuiz(ctx, grpcReq)
	if err != nil {
		h.logger.Errorf("生成题目失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成题目失败"})
		return
	}

	if !resp.Success {
		c.JSON(http.StatusBadRequest, gin.H{"error": resp.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   resp.Success,
		"message":   resp.Message,
		"questions": resp.Questions,
	})
}

// 获取题目详情
func (h *QuizHandler) GetQuiz(c *gin.Context) {
	questionID := c.Param("questionId")
	if questionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "题目ID不能为空"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.quizClient.GetQuiz(ctx, &pb.GetQuizRequest{
		QuestionId: questionID,
	})
	if err != nil {
		h.logger.Errorf("获取题目失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取题目失败"})
		return
	}

	if !resp.Success {
		c.JSON(http.StatusNotFound, gin.H{"error": resp.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  resp.Success,
		"question": resp.Question,
	})
}

// 获取题目列表
func (h *QuizHandler) ListQuizzes(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	materialID := c.Query("material_id")
	questionType := c.Query("type")
	difficulty := c.Query("difficulty")
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "10")

	pageInt, _ := strconv.Atoi(page)
	pageSizeInt, _ := strconv.Atoi(pageSize)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListQuizzesRequest{
		UserId:   userID.(string),
		Page:     int32(pageInt),
		PageSize: int32(pageSizeInt),
	}

	if materialID != "" {
		req.MaterialId = materialID
	}
	if questionType != "" {
		if typeInt, err := strconv.Atoi(questionType); err == nil {
			req.Type = pb.QuestionType(typeInt)
		}
	}
	if difficulty != "" {
		if diffInt, err := strconv.Atoi(difficulty); err == nil {
			req.Difficulty = pb.DifficultyLevel(diffInt)
		}
	}

	resp, err := h.quizClient.ListQuizzes(ctx, req)
	if err != nil {
		h.logger.Errorf("获取题目列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取题目列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   resp.Success,
		"questions": resp.Questions,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
	})
}

// 提交答案
func (h *QuizHandler) SubmitAnswer(c *gin.Context) {
	questionID := c.Param("questionId")
	if questionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "题目ID不能为空"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	var req SubmitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.quizClient.SubmitAnswer(ctx, &pb.SubmitAnswerRequest{
		QuestionId: questionID,
		UserId:     userID.(string),
		Answer:     req.Answer,
	})
	if err != nil {
		h.logger.Errorf("提交答案失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交答案失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        resp.Success,
		"message":        resp.Message,
		"is_correct":     resp.IsCorrect,
		"score":          resp.Score,
		"correct_answer": resp.CorrectAnswer,
		"explanation":    resp.Explanation,
	})
}

// 获取用户答题历史
func (h *QuizHandler) GetUserHistory(c *gin.Context) {
	userIDParam := c.Param("userId")

	// 验证用户权限（只能查看自己的历史记录）
	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	if userIDParam != currentUserID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问他人的答题历史"})
		return
	}

	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "10")

	pageInt, _ := strconv.Atoi(page)
	pageSizeInt, _ := strconv.Atoi(pageSize)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.quizClient.GetUserQuizHistory(ctx, &pb.GetUserQuizHistoryRequest{
		UserId:   userIDParam,
		Page:     int32(pageInt),
		PageSize: int32(pageSizeInt),
	})
	if err != nil {
		h.logger.Errorf("获取答题历史失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取答题历史失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"answers": resp.Answers,
		"total":   resp.Total,
	})
}

// 获取知识点统计
func (h *QuizHandler) GetKnowledgeStats(c *gin.Context) {
	userIDParam := c.Param("userId")

	// 验证用户权限
	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未认证"})
		return
	}

	if userIDParam != currentUserID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问他人的学习统计"})
		return
	}

	materialID := c.Query("material_id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.quizClient.GetKnowledgeStats(ctx, &pb.GetKnowledgeStatsRequest{
		UserId:     userIDParam,
		MaterialId: materialID,
	})
	if err != nil {
		h.logger.Errorf("获取知识点统计失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取知识点统计失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          resp.Success,
		"stats":            resp.Stats,
		"overall_accuracy": resp.OverallAccuracy,
	})
}
