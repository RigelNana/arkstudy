package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	authpb "auth-service/rpc"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuthValidator 负责与 auth-service 通信
type AuthValidator struct {
	client authpb.AuthServiceClient
}

func NewAuthValidator() *AuthValidator {
	addr := os.Getenv("AUTH_GRPC_ADDR")
	if addr == "" {
		addr = "localhost:50051"
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic("failed to dial auth-service: " + err.Error())
	}
	return &AuthValidator{client: authpb.NewAuthServiceClient(conn)}
}

// JWTAuth 中间件：提取 Bearer token -> 远程 ValidateToken -> 注入 user_id
func (v *AuthValidator) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			unauthorized(c, "missing Authorization header")
			return
		}
		token := header
		if after, ok := strings.CutPrefix(header, "Bearer "); ok {
			token = after
		}
		if token == "" {
			unauthorized(c, "empty bearer token")
			return
		}
		resp, err := v.client.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{Token: token})
		if err != nil || !resp.Valid {
			unauthorized(c, "invalid token")
			return
		}
		c.Set("user_id", resp.UserId)
		c.Next()
	}
}

func unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
	c.Abort()
}
