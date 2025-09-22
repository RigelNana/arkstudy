package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	llmpb "github.com/RigelNana/arkstudy/proto/llm"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type LLMHandler struct {
	client llmpb.LLMServiceClient
}

func NewLLMHandler(client llmpb.LLMServiceClient) *LLMHandler {
	return &LLMHandler{client: client}
}

// NewLLMServiceClient creates a gRPC client to llm-service using env LLM_GRPC_ADDR (default localhost:50054)
func NewLLMServiceClient() llmpb.LLMServiceClient {
	addr := os.Getenv("LLM_GRPC_ADDR")
	if addr == "" {
		addr = "localhost:50054"
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("dial llm-service: %v", err)
	}
	return llmpb.NewLLMServiceClient(conn)
}

// POST /api/ai/ask
func (h *LLMHandler) Ask(c *gin.Context) {
	var req struct {
		Question    string            `form:"question" json:"question"`
		MaterialIDs []string          `form:"material_ids" json:"material_ids"`
		Context     map[string]string `json:"context"`
		SessionID   string            `form:"session_id" json:"session_id"`
		MaxTurns    int               `form:"max_history_turns" json:"max_history_turns"`
	}
	// 同时支持 JSON 和表单
	_ = c.ShouldBind(&req)
	if req.Question == "" {
		// 表单或查询兜底
		if v := c.PostForm("question"); v != "" {
			req.Question = v
		} else if v := c.Query("question"); v != "" {
			req.Question = v
		}
	}
	// 读取 session 参数（也允许从 query/form 获取）
	if req.SessionID == "" {
		req.SessionID = c.PostForm("session_id")
		if req.SessionID == "" {
			req.SessionID = c.Query("session_id")
		}
	}
	if req.MaxTurns == 0 {
		if v := c.PostForm("max_history_turns"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.MaxTurns = n
			}
		} else if v := c.Query("max_history_turns"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.MaxTurns = n
			}
		}
	}
	// 兼容逗号分隔的 material_ids（表单/查询）
	if len(req.MaterialIDs) == 0 {
		raw := c.PostForm("material_ids")
		if raw == "" {
			raw = c.Query("material_ids")
		}
		if raw != "" {
			parts := strings.Split(raw, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					req.MaterialIDs = append(req.MaterialIDs, p)
				}
			}
		}
	}
	if req.Question == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	userIDVal, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userID, _ := userIDVal.(string)

	// 合并 context，透传会话参数
	if req.Context == nil {
		req.Context = map[string]string{}
	}
	if req.SessionID != "" {
		req.Context["session_id"] = req.SessionID
	}
	// 默认：如提供了 session_id 但未显式设置 max_history_turns，则默认取 3 轮
	mht := req.MaxTurns
	if mht == 0 && req.SessionID != "" {
		mht = 3
	}
	if mht > 0 {
		req.Context["max_history_turns"] = strconv.Itoa(mht)
	}

	resp, err := h.client.AskQuestion(context.Background(), &llmpb.QuestionRequest{
		Question:    req.Question,
		UserId:      userID,
		MaterialIds: req.MaterialIDs,
		Context:     req.Context,
	})
	if err != nil {
		log.Printf("AskQuestion gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ask failed", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"answer":     resp.Answer,
			"confidence": resp.Confidence,
			"sources":    resp.Sources,
			"metadata":   resp.Metadata,
		},
	})
}

// GET /api/ai/ask/stream?question=... 或 POST 表单/JSON，同步转为 SSE 输出
func (h *LLMHandler) AskStream(c *gin.Context) {
	// 输入解析复用 Ask 的逻辑（支持 JSON/表单/查询）
	var req struct {
		Question    string            `form:"question" json:"question"`
		MaterialIDs []string          `form:"material_ids" json:"material_ids"`
		Context     map[string]string `json:"context"`
		SessionID   string            `form:"session_id" json:"session_id"`
		MaxTurns    int               `form:"max_history_turns" json:"max_history_turns"`
	}
	_ = c.ShouldBind(&req)
	if req.Question == "" {
		if v := c.PostForm("question"); v != "" {
			req.Question = v
		} else if v := c.Query("question"); v != "" {
			req.Question = v
		}
	}
	if req.SessionID == "" {
		req.SessionID = c.PostForm("session_id")
		if req.SessionID == "" {
			req.SessionID = c.Query("session_id")
		}
	}
	if req.MaxTurns == 0 {
		if v := c.PostForm("max_history_turns"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.MaxTurns = n
			}
		} else if v := c.Query("max_history_turns"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.MaxTurns = n
			}
		}
	}
	if len(req.MaterialIDs) == 0 {
		raw := c.PostForm("material_ids")
		if raw == "" {
			raw = c.Query("material_ids")
		}
		if raw != "" {
			parts := strings.Split(raw, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					req.MaterialIDs = append(req.MaterialIDs, p)
				}
			}
		}
	}
	if req.Question == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	userIDVal, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userID, _ := userIDVal.(string)

	// SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// 合并 context，透传会话参数
	if req.Context == nil {
		req.Context = map[string]string{}
	}
	if req.SessionID != "" {
		req.Context["session_id"] = req.SessionID
	}
	// 默认：如提供了 session_id 但未显式设置 max_history_turns，则默认取 3 轮
	mht := req.MaxTurns
	if mht == 0 && req.SessionID != "" {
		mht = 3
	}
	if mht > 0 {
		req.Context["max_history_turns"] = strconv.Itoa(mht)
	}

	stream, err := h.client.AskQuestionStream(context.Background(), &llmpb.QuestionRequest{
		Question:    req.Question,
		UserId:      userID,
		MaterialIds: req.MaterialIDs,
		Context:     req.Context,
	})
	if err != nil {
		log.Printf("AskQuestionStream gRPC error: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// 逐条写入 SSE data: token\n\n
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		if token := chunk.GetContent(); token != "" {
			_, _ = c.Writer.WriteString("data: " + token + "\n\n")
			c.Writer.Flush()
		}
		if chunk.GetIsFinal() {
			// Emit a final JSON event with session_id and any metadata so clients can capture it
			finalPayload := map[string]any{
				"is_final":   true,
				"session_id": chunk.GetMetadata()["session_id"],
				"metadata":   chunk.GetMetadata(),
			}
			if b, err := json.Marshal(finalPayload); err == nil {
				_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
				c.Writer.Flush()
			}
			break
		}
	}
}

// GET /api/ai/search?query=&top_k=
func (h *LLMHandler) Search(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
		return
	}

	topK := 5
	if v := c.Query("top_k"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			topK = n
		}
	}

	userIDVal, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userID, _ := userIDVal.(string)

	resp, err := h.client.SemanticSearch(context.Background(), &llmpb.SearchRequest{
		Query:  query,
		UserId: userID,
		TopK:   int32(topK),
	})
	if err != nil {
		log.Printf("SemanticSearch gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp.Results})
}
