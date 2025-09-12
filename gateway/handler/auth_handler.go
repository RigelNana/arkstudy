package handler

import (
	"context"
	"log"
	"net/http"
	"os"

	authpb "github.com/RigelNana/arkstudy/proto/auth"
	userpb "github.com/RigelNana/arkstudy/proto/user"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type AuthHandler struct {
	authClient authpb.AuthServiceClient
	userClient userpb.UserServiceClient
}

func NewAuthHandler(authClient authpb.AuthServiceClient, userClient userpb.UserServiceClient) *AuthHandler {
	return &AuthHandler{authClient: authClient, userClient: userClient}
}

// Register expects username,email,password ->
// 1. 调用 user-service CreateUser 获取 user_id
// 2. 调用 auth-service Register(user_id,password)
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct{ Username, Email, Password string }
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Email == "" || req.Password == "" {
		log.Printf("Register validation failed: err=%v, username=%s, email=%s, password=%s", err, req.Username, req.Email, req.Password)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	log.Printf("Register request: username=%s, email=%s", req.Username, req.Email)

	// create user
	cuResp, err := h.userClient.CreateUser(context.Background(), &userpb.CreateUserRequest{Username: req.Username, Email: req.Email, Role: "student", Description: ""})
	if err != nil {
		log.Printf("CreateUser gRPC error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "create user failed", "detail": err.Error()})
		return
	}
	if !cuResp.Success {
		log.Printf("CreateUser failed: success=%v, message=%s", cuResp.Success, cuResp.GetMessage())
		c.JSON(http.StatusBadRequest, gin.H{"error": "create user failed", "detail": cuResp.GetMessage()})
		return
	}
	userID := cuResp.User.Id
	log.Printf("CreateUser success: userID=%s", userID)

	// register auth
	ar, err := h.authClient.Register(context.Background(), &authpb.RegisterRequest{UserId: userID, Password: req.Password})
	if err != nil || !ar.Success {
		log.Printf("Auth register failed: err=%v, success=%v, message=%s", err, ar.Success, ar.GetMessage())
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth register failed", "detail": ar.GetMessage()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "message": "registered"})
}

// Login expects user_id + password (或未来支持 username/email -> 查询 user-service)
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct{ Identifier, Password string }
	if err := c.ShouldBindJSON(&req); err != nil || req.Identifier == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	// 查询 user_id
	ur, err := h.userClient.GetUserByUsername(context.Background(), &userpb.GetUserByUsernameRequest{Username: req.Identifier})
	if err != nil || !ur.Found {
		ur, err = h.userClient.GetUserByEmail(context.Background(), &userpb.GetUserByEmailRequest{Email: req.Identifier})
		if err != nil || !ur.Found {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
	}
	userID := ur.User.Id
	lr, err := h.authClient.Login(context.Background(), &authpb.LoginRequest{UserId: userID, Password: req.Password})
	if err != nil || !lr.Success {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login failed", "detail": lr.GetMessage()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": lr.Token})
}

// Validate token -> 返回 user_id
func (h *AuthHandler) Validate(c *gin.Context) {
	// 首先尝试从 Authorization header 获取 token
	authHeader := c.GetHeader("Authorization")
	var token string

	if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:] // 去掉 "Bearer " 前缀
	} else {
		// 如果没有 Authorization header，则从查询参数获取
		token = c.Query("token")
	}

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}

	vr, err := h.authClient.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: token})
	if err != nil || !vr.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"valid": false, "message": vr.GetMessage()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "user_id": vr.UserId})
}

// Helpers to create gRPC clients
func NewAuthServiceClient() authpb.AuthServiceClient {
	addr := os.Getenv("AUTH_GRPC_ADDR")
	if addr == "" {
		addr = "localhost:50051"
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("dial auth-service: %v", err)
	}
	return authpb.NewAuthServiceClient(conn)
}
func NewUserServiceClient() userpb.UserServiceClient {
	addr := os.Getenv("USER_GRPC_ADDR")
	if addr == "" {
		addr = "localhost:50052"
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("dial user-service: %v", err)
	}
	return userpb.NewUserServiceClient(conn)
}
