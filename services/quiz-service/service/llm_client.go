package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	llmPb "github.com/RigelNana/arkstudy/proto/llm"
)

type LLMServiceClient struct {
	client llmPb.LLMServiceClient
	logger *logrus.Logger
}

func NewLLMServiceClient(llmServiceAddr string, logger *logrus.Logger) (*LLMServiceClient, error) {
	conn, err := grpc.Dial(llmServiceAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LLM service: %v", err)
	}

	client := llmPb.NewLLMServiceClient(conn)
	return &LLMServiceClient{
		client: client,
		logger: logger,
	}, nil
}

// 获取材料内容，用于出题
func (c *LLMServiceClient) GetMaterialContent(ctx context.Context, materialID, userID string) (string, error) {
	c.logger.Infof("获取材料内容，材料ID: %s, 用户ID: %s", materialID, userID)

	// 策略1: 使用material_ids精确查找指定材料
	searchReq := &llmPb.SearchRequest{
		Query:       "深度学习 机器学习 神经网络 算法", // 使用更具体的查询词汇
		UserId:      userID,
		TopK:        50,
		MaterialIds: []string{materialID}, // 精确指定material_id
	}

	resp, err := c.client.SemanticSearch(ctx, searchReq)
	if err != nil {
		return "", fmt.Errorf("failed to search material content: %v", err)
	}

	// 收集内容片段
	var contentParts []string
	for _, result := range resp.Results {
		contentParts = append(contentParts, result.Content)
	}

	// 如果没有找到指定material_id的内容，尝试策略2: 不限制material_id的广泛搜索
	if len(contentParts) == 0 {
		c.logger.Infof("未找到material_id=%s的内容，尝试广泛搜索", materialID)

		// 策略2: 使用语义搜索该用户的所有内容
		searchReq.MaterialIds = nil // 移除material_id限制
		searchReq.Query = "学习 内容 知识 材料"
		resp, err = c.client.SemanticSearch(ctx, searchReq)
		if err != nil {
			return "", fmt.Errorf("failed to search with broad query: %v", err)
		}

		// 收集所有相关内容
		for _, result := range resp.Results {
			contentParts = append(contentParts, result.Content)
		}
	}

	if len(contentParts) == 0 {
		return "", fmt.Errorf("no content found for user %s", userID)
	}

	// 合并内容，限制总长度
	fullContent := strings.Join(contentParts, "\n\n")
	if len(fullContent) > 4000 {
		fullContent = fullContent[:4000] + "..."
	}

	c.logger.Infof("获取到材料内容，片段数: %d, 总长度: %d 字符", len(contentParts), len(fullContent))
	return fullContent, nil
}

// 使用LLM进行智能出题
func (c *LLMServiceClient) GenerateQuestionsWithLLM(ctx context.Context, materialContent, prompt string, userID string) (string, error) {
	c.logger.Infof("使用LLM生成题目，内容长度: %d, 用户ID: %s", len(materialContent), userID)

	// 构建完整的提示词
	fullPrompt := fmt.Sprintf("%s\n\n材料内容:\n%s", prompt, materialContent)

	questionReq := &llmPb.QuestionRequest{
		Question: fullPrompt,
		UserId:   userID,
		Context: map[string]string{
			"task": "question_generation",
			"type": "educational",
		},
	}

	resp, err := c.client.AskQuestion(ctx, questionReq)
	if err != nil {
		return "", fmt.Errorf("failed to generate questions with LLM: %v", err)
	}

	c.logger.Infof("LLM生成题目成功，置信度: %.2f", resp.Confidence)
	return resp.Answer, nil
}

// 评估主观题答案
func (c *LLMServiceClient) EvaluateSubjectiveAnswer(ctx context.Context, question, correctAnswer, userAnswer, userID string) (float32, string, error) {
	c.logger.Infof("评估主观题答案，用户ID: %s", userID)

	prompt := fmt.Sprintf(`请评估以下答案的质量：

题目：%s

标准答案：%s

学生答案：%s

请给出评分（0-1之间的小数）和简短的评价。请按以下JSON格式返回：
{
  "score": 0.85,
  "feedback": "答案基本正确，但缺少部分关键点..."
}`, question, correctAnswer, userAnswer)

	questionReq := &llmPb.QuestionRequest{
		Question: prompt,
		UserId:   userID,
		Context: map[string]string{
			"task": "answer_evaluation",
			"type": "subjective",
		},
	}

	resp, err := c.client.AskQuestion(ctx, questionReq)
	if err != nil {
		return 0, "", fmt.Errorf("failed to evaluate answer with LLM: %v", err)
	}

	// 这里应该解析JSON响应，简化处理
	// 实际实现中应该使用JSON解析
	score := resp.Confidence // 简化处理，使用置信度作为分数
	feedback := resp.Answer

	c.logger.Infof("答案评估完成，得分: %.2f", score)
	return score, feedback, nil
}

// 提取知识点
func (c *LLMServiceClient) ExtractKnowledgePoints(ctx context.Context, content, userID string) ([]string, error) {
	c.logger.Infof("提取知识点，内容长度: %d", len(content))

	prompt := fmt.Sprintf(`请从以下内容中提取3-5个主要知识点，每个知识点用简短的词语表达：

%s

请只返回知识点列表，每行一个，格式如：
- 知识点1
- 知识点2
- 知识点3`, content)

	questionReq := &llmPb.QuestionRequest{
		Question: prompt,
		UserId:   userID,
		Context: map[string]string{
			"task": "knowledge_extraction",
			"type": "educational",
		},
	}

	resp, err := c.client.AskQuestion(ctx, questionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to extract knowledge points: %v", err)
	}

	// 解析知识点
	lines := strings.Split(resp.Answer, "\n")
	var knowledgePoints []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "•") {
			point := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "-"), "•"))
			if point != "" {
				knowledgePoints = append(knowledgePoints, point)
			}
		}
	}

	c.logger.Infof("提取到 %d 个知识点", len(knowledgePoints))
	return knowledgePoints, nil
}
