package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"

	userpb "github.com/RigelNana/arkstudy/proto/user"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userClient userpb.UserServiceClient
}

func NewUserHandler(userClient userpb.UserServiceClient) *UserHandler {
	return &UserHandler{userClient: userClient}
}

// GetUserByID 根据ID获取用户信息
// GET /api/users/:id
func (h *UserHandler) GetUserByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is required"})
		return
	}

	log.Printf("GetUserByID request: id=%s", id)

	resp, err := h.userClient.GetUserByID(context.Background(), &userpb.GetUserByIDRequest{Id: id})
	if err != nil {
		log.Printf("GetUserByID gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "detail": err.Error()})
		return
	}

	if !resp.Found {
		log.Printf("GetUserByID user not found: id=%s", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found", "detail": resp.Message})
		return
	}

	log.Printf("GetUserByID success: id=%s, username=%s", id, resp.User.Username)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp.User,
	})
}

// GetUserByUsername 根据用户名获取用户信息
// GET /api/users/username/:username
func (h *UserHandler) GetUserByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	log.Printf("GetUserByUsername request: username=%s", username)

	resp, err := h.userClient.GetUserByUsername(context.Background(), &userpb.GetUserByUsernameRequest{Username: username})
	if err != nil {
		log.Printf("GetUserByUsername gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "detail": err.Error()})
		return
	}

	if !resp.Found {
		log.Printf("GetUserByUsername user not found: username=%s", username)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found", "detail": resp.Message})
		return
	}

	log.Printf("GetUserByUsername success: username=%s, id=%s", username, resp.User.Id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp.User,
	})
}

// GetUserByEmail 根据邮箱获取用户信息
// GET /api/users/email/:email
func (h *UserHandler) GetUserByEmail(c *gin.Context) {
	email := c.Param("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}

	log.Printf("GetUserByEmail request: email=%s", email)

	resp, err := h.userClient.GetUserByEmail(context.Background(), &userpb.GetUserByEmailRequest{Email: email})
	if err != nil {
		log.Printf("GetUserByEmail gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "detail": err.Error()})
		return
	}

	if !resp.Found {
		log.Printf("GetUserByEmail user not found: email=%s", email)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found", "detail": resp.Message})
		return
	}

	log.Printf("GetUserByEmail success: email=%s, id=%s", email, resp.User.Id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp.User,
	})
}

// ListUsers 获取用户列表（分页）
// GET /api/users?limit=10&offset=0
func (h *UserHandler) ListUsers(c *gin.Context) {
	// 解析查询参数
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // 限制最大值
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	log.Printf("ListUsers request: limit=%d, offset=%d", limit, offset)

	resp, err := h.userClient.ListUsers(context.Background(), &userpb.ListUsersRequest{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		log.Printf("ListUsers gRPC error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "detail": err.Error()})
		return
	}

	log.Printf("ListUsers success: returned %d users, total %d", len(resp.Users), resp.Total)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"users":  resp.Users,
			"total":  resp.Total,
			"limit":  limit,
			"offset": offset,
		},
	})
}
