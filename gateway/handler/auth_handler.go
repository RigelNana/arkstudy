package handler

import (
	authpb "auth-service/rpc"
	"context"
	"log"
	"net/http"
	"os"
	userpb "user-service/rpc"

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	// create user
	cuResp, err := h.userClient.CreateUser(context.Background(), &userpb.CreateUserRequest{Username: req.Username, Email: req.Email, Role: "student", Description: ""})
	if err != nil || !cuResp.Success {
		c.JSON(http.StatusBadRequest, gin.H{"error": "create user failed", "detail": cuResp.GetMessage()})
		return
	}
	userID := cuResp.User.Id
	// register auth
	ar, err := h.authClient.Register(context.Background(), &authpb.RegisterRequest{UserId: userID, Password: req.Password})
	if err != nil || !ar.Success {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth register failed", "detail": ar.GetMessage()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_id": userID, "message": "registered"})
}

// Login expects user_id + password (或未来支持 username/email -> 查询 user-service)
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct{ UserID, Password string }
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	lr, err := h.authClient.Login(context.Background(), &authpb.LoginRequest{UserId: req.UserID, Password: req.Password})
	if err != nil || !lr.Success {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login failed", "detail": lr.GetMessage()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": lr.Token})
}

// Validate token -> 返回 user_id
func (h *AuthHandler) Validate(c *gin.Context) {
	token := c.Query("token")
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
