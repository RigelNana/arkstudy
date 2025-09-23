package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"

	"github.com/RigelNana/arkstudy/quiz-service/models"
)

type QuizService struct {
	openaiClient *openai.Client
	llmClient    *LLMServiceClient
	logger       *logrus.Logger
}

func NewQuizService(apiKey string, baseURL string, llmServiceAddr string, logger *logrus.Logger) *QuizService {
	var client *openai.Client
	if baseURL != "" {
		// 使用自定义baseURL创建客户端
		config := openai.DefaultConfig(apiKey)
		config.BaseURL = baseURL
		client = openai.NewClientWithConfig(config)
	} else {
		// 使用默认OpenAI客户端
		client = openai.NewClient(apiKey)
	}

	// 初始化LLM服务客户端
	llmClient, err := NewLLMServiceClient(llmServiceAddr, logger)
	if err != nil {
		logger.Errorf("Failed to create LLM client: %v", err)
		llmClient = nil // 如果连接失败，设为nil，后续使用OpenAI作为后备
	}

	return &QuizService{
		openaiClient: client,
		llmClient:    llmClient,
		logger:       logger,
	}
}

type QuestionGenerationRequest struct {
	MaterialContent string                 `json:"material_content"`
	MaterialID      string                 `json:"material_id"`
	UserID          string                 `json:"user_id"`
	QuestionTypes   []models.QuestionType  `json:"question_types"`
	Difficulty      models.DifficultyLevel `json:"difficulty"`
	Count           int                    `json:"count"`
	KnowledgePoints []string               `json:"knowledge_points"`
}

type GeneratedQuestion struct {
	Type            models.QuestionType    `json:"type"`
	Content         string                 `json:"content"`
	Options         []string               `json:"options,omitempty"`
	CorrectAnswer   string                 `json:"correct_answer"`
	Explanation     string                 `json:"explanation"`
	Difficulty      models.DifficultyLevel `json:"difficulty"`
	KnowledgePoints []string               `json:"knowledge_points"`
}

// 生成题目的主要方法
func (s *QuizService) GenerateQuestions(ctx context.Context, req *QuestionGenerationRequest) ([]*GeneratedQuestion, error) {
	s.logger.Infof("开始生成题目，材料ID: %s, 用户ID: %s, 题目数量: %d", req.MaterialID, req.UserID, req.Count)

	// 从LLM服务获取材料内容
	var materialContent string
	var err error

	if s.llmClient != nil {
		materialContent, err = s.llmClient.GetMaterialContent(ctx, req.MaterialID, req.UserID)
		if err != nil {
			s.logger.Errorf("从LLM服务获取材料内容失败: %v", err)
			// 使用传入的内容作为后备
			materialContent = req.MaterialContent
		}
	} else {
		// 如果LLM客户端不可用，使用传入的内容
		materialContent = req.MaterialContent
	}

	if materialContent == "" {
		return nil, fmt.Errorf("无法获取材料内容")
	}

	// 如果没有指定知识点，尝试从LLM服务提取
	knowledgePoints := req.KnowledgePoints
	if len(knowledgePoints) == 0 && s.llmClient != nil {
		extractedPoints, err := s.llmClient.ExtractKnowledgePoints(ctx, materialContent, req.UserID)
		if err != nil {
			s.logger.Errorf("提取知识点失败: %v", err)
		} else {
			knowledgePoints = extractedPoints
		}
	}

	// 更新请求对象
	req.MaterialContent = materialContent
	req.KnowledgePoints = knowledgePoints

	var questions []*GeneratedQuestion

	for _, questionType := range req.QuestionTypes {
		count := req.Count / len(req.QuestionTypes)
		if count == 0 {
			count = 1
		}

		typeQuestions, err := s.generateQuestionsByType(ctx, req, questionType, count)
		if err != nil {
			s.logger.Errorf("生成 %s 类型题目失败: %v", questionType.String(), err)
			continue
		}

		questions = append(questions, typeQuestions...)
	}

	s.logger.Infof("成功生成 %d 道题目", len(questions))
	return questions, nil
}

// 根据题目类型生成题目
func (s *QuizService) generateQuestionsByType(ctx context.Context, req *QuestionGenerationRequest, questionType models.QuestionType, count int) ([]*GeneratedQuestion, error) {
	prompt := s.buildPrompt(req, questionType, count)

	// 优先使用LLM服务生成题目
	if s.llmClient != nil {
		s.logger.Infof("使用LLM服务生成 %s 类型题目", questionType.String())

		response, err := s.llmClient.GenerateQuestionsWithLLM(ctx, req.MaterialContent, prompt, req.UserID)
		if err != nil {
			s.logger.Errorf("LLM服务生成题目失败，回退到OpenAI: %v", err)
		} else {
			// 解析LLM服务的响应
			questions, err := s.parseQuestions(response, questionType, req.Difficulty)
			if err != nil {
				s.logger.Errorf("解析LLM响应失败，回退到OpenAI: %v", err)
			} else {
				s.logger.Infof("LLM服务成功生成 %d 道题目", len(questions))
				return questions, nil
			}
		}
	}

	// 回退到直接使用OpenAI
	s.logger.Infof("使用OpenAI直接生成 %s 类型题目", questionType.String())

	resp, err := s.openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       openai.GPT3Dot5Turbo,
		Temperature: 0.7,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "你是一个专业的出题专家，能够根据给定的学习材料生成高质量的题目。请严格按照用户提供的JSON格式返回结果。",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("调用OpenAI API失败: %v", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI API返回为空")
	}

	return s.parseQuestions(resp.Choices[0].Message.Content, questionType, req.Difficulty)
}

// 构建提示词
func (s *QuizService) buildPrompt(req *QuestionGenerationRequest, questionType models.QuestionType, count int) string {
	var promptBuilder strings.Builder

	promptBuilder.WriteString(fmt.Sprintf("基于以下学习材料，生成 %d 道 %s 类型的题目。\n\n", count, s.getQuestionTypeDescription(questionType)))
	promptBuilder.WriteString("学习材料内容:\n")
	promptBuilder.WriteString(req.MaterialContent)
	promptBuilder.WriteString("\n\n")

	promptBuilder.WriteString(fmt.Sprintf("难度级别: %s\n", s.getDifficultyDescription(req.Difficulty)))

	if len(req.KnowledgePoints) > 0 {
		promptBuilder.WriteString("重点关注的知识点: " + strings.Join(req.KnowledgePoints, ", ") + "\n")
	}

	promptBuilder.WriteString("\n请按照以下JSON格式返回题目:\n")
	promptBuilder.WriteString(s.getQuestionFormat(questionType))

	return promptBuilder.String()
}

// 获取题目类型描述
func (s *QuizService) getQuestionTypeDescription(questionType models.QuestionType) string {
	switch questionType {
	case models.MultipleChoice:
		return "选择题"
	case models.FillBlank:
		return "填空题"
	case models.ShortAnswer:
		return "简答题"
	case models.TrueFalse:
		return "判断题"
	case models.Essay:
		return "论述题"
	default:
		return "未知类型"
	}
}

// 获取难度描述
func (s *QuizService) getDifficultyDescription(difficulty models.DifficultyLevel) string {
	switch difficulty {
	case models.Easy:
		return "简单（基础概念理解）"
	case models.Medium:
		return "中等（概念应用）"
	case models.Hard:
		return "困难（深度分析和综合运用）"
	default:
		return "中等"
	}
}

// 获取题目格式模板
func (s *QuizService) getQuestionFormat(questionType models.QuestionType) string {
	switch questionType {
	case models.MultipleChoice:
		return `{
  "questions": [
    {
      "content": "题目内容",
      "options": ["A. 选项1", "B. 选项2", "C. 选项3", "D. 选项4"],
      "correct_answer": "A",
      "explanation": "答案解析",
      "knowledge_points": ["知识点1", "知识点2"]
    }
  ]
}`
	case models.FillBlank:
		return `{
  "questions": [
    {
      "content": "题目内容，使用 _____ 表示填空位置",
      "correct_answer": "正确答案",
      "explanation": "答案解析",
      "knowledge_points": ["知识点1", "知识点2"]
    }
  ]
}`
	case models.TrueFalse:
		return `{
  "questions": [
    {
      "content": "判断题内容",
      "correct_answer": "true/false",
      "explanation": "判断理由",
      "knowledge_points": ["知识点1", "知识点2"]
    }
  ]
}`
	default:
		return `{
  "questions": [
    {
      "content": "题目内容",
      "correct_answer": "参考答案",
      "explanation": "答案要点",
      "knowledge_points": ["知识点1", "知识点2"]
    }
  ]
}`
	}
}

// 解析生成的题目
func (s *QuizService) parseQuestions(content string, questionType models.QuestionType, difficulty models.DifficultyLevel) ([]*GeneratedQuestion, error) {
	// 尝试提取JSON内容
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("无法找到有效的JSON内容")
	}

	jsonContent := content[jsonStart : jsonEnd+1]

	var result struct {
		Questions []struct {
			Content         string   `json:"content"`
			Options         []string `json:"options,omitempty"`
			CorrectAnswer   string   `json:"correct_answer"`
			Explanation     string   `json:"explanation"`
			KnowledgePoints []string `json:"knowledge_points"`
		} `json:"questions"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		s.logger.Errorf("解析JSON失败: %v, 内容: %s", err, jsonContent)
		return nil, fmt.Errorf("解析题目JSON失败: %v", err)
	}

	var questions []*GeneratedQuestion
	for _, q := range result.Questions {
		question := &GeneratedQuestion{
			Type:            questionType,
			Content:         q.Content,
			Options:         q.Options,
			CorrectAnswer:   q.CorrectAnswer,
			Explanation:     q.Explanation,
			Difficulty:      difficulty,
			KnowledgePoints: q.KnowledgePoints,
		}
		questions = append(questions, question)
	}

	return questions, nil
}

// 将生成的题目转换为数据库模型
func (s *QuizService) ConvertToQuestionModel(generated *GeneratedQuestion, materialID, userID string) *models.Question {
	question := &models.Question{
		QuestionID:    uuid.New().String(),
		Type:          generated.Type,
		Content:       generated.Content,
		CorrectAnswer: generated.CorrectAnswer,
		Explanation:   generated.Explanation,
		Difficulty:    generated.Difficulty,
		MaterialID:    materialID,
		CreatorID:     userID,
	}

	// 序列化选项
	if len(generated.Options) > 0 {
		optionsJSON, _ := json.Marshal(generated.Options)
		question.Options = string(optionsJSON)
	}

	// 序列化知识点
	if len(generated.KnowledgePoints) > 0 {
		knowledgePointsJSON, _ := json.Marshal(generated.KnowledgePoints)
		question.KnowledgePoints = string(knowledgePointsJSON)
	}

	return question
}

// 评估主观题答案
func (s *QuizService) EvaluateSubjectiveAnswer(ctx context.Context, question *models.Question, userAnswer, userID string) (float32, string, error) {
	// 对于客观题，直接比较答案
	if question.Type == models.MultipleChoice || question.Type == models.TrueFalse {
		if question.CorrectAnswer == userAnswer {
			return 1.0, "答案正确", nil
		}
		return 0.0, "答案错误", nil
	}

	// 对于填空题，进行简单的字符串匹配
	if question.Type == models.FillBlank {
		// 可以在这里添加更复杂的匹配逻辑，如模糊匹配、关键词匹配等
		if strings.TrimSpace(strings.ToLower(question.CorrectAnswer)) == strings.TrimSpace(strings.ToLower(userAnswer)) {
			return 1.0, "答案正确", nil
		}
		return 0.0, "答案不正确", nil
	}

	// 对于主观题，使用LLM服务评估
	if s.llmClient != nil {
		score, feedback, err := s.llmClient.EvaluateSubjectiveAnswer(ctx, question.Content, question.CorrectAnswer, userAnswer, userID)
		if err != nil {
			s.logger.Errorf("LLM评估主观题失败: %v", err)
			// 回退到简单评估
			return 0.8, "答案已提交，需要人工评估", nil
		}
		return score, feedback, nil
	}

	// 如果LLM服务不可用，对主观题给予默认分数
	return 0.8, "答案已提交，需要人工评估", nil
}
