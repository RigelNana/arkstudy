package service

import (
	"context"
	"errors"
	"os"
	"strconv"

	"github.com/RigelNana/arkstudy/proto/user"

	"github.com/RigelNana/arkstudy/services/auth-service/models"
	"github.com/RigelNana/arkstudy/services/auth-service/repository"
	"github.com/RigelNana/arkstudy/services/auth-service/utils"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
)

type AuthService interface {
	Register(userID uuid.UUID, rawPassword string) error
	Login(userID uuid.UUID, rawPassword string) (string, error)
	ValidateToken(token string) (uuid.UUID, error)
	CheckPassword(userID uuid.UUID, rawPassword string) (bool, error)
	UpdatePassword(userID uuid.UUID, newPassword string) error
}

type AuthServiceImpl struct {
	repo               repository.AuthRepository
	tokenExpireMinutes int
	userClient         user.UserServiceClient
}

func NewAuthService(repo repository.AuthRepository) AuthService {
	expireStr := os.Getenv("JWT_EXPIRE_MINUTES")
	if expireStr == "" {
		expireStr = "60"
	}
	minutes, _ := strconv.Atoi(expireStr)
	// 建立 user-service gRPC 连接
	userAddr := os.Getenv("USER_GRPC_ADDR")
	if userAddr == "" {
		// 尝试使用 K8s 服务发现
		userHost := os.Getenv("ARKSTUDY_USER_SERVICE_SERVICE_HOST")
		userPort := os.Getenv("ARKSTUDY_USER_SERVICE_SERVICE_PORT")
		if userHost != "" && userPort != "" {
			userAddr = userHost + ":" + userPort
		} else {
			userAddr = "localhost:50052" // 单机默认端口
		}
	}
	conn, err := grpc.Dial(userAddr, grpc.WithInsecure())
	var client user.UserServiceClient
	if err == nil { // 若失败，client 为空，后续 Register 将报错提示
		client = user.NewUserServiceClient(conn)
	}
	return &AuthServiceImpl{repo: repo, tokenExpireMinutes: minutes, userClient: client}
}

func (s *AuthServiceImpl) Register(userID uuid.UUID, rawPassword string) error {
	// 先调用 user-service 校验 user_id 是否存在
	if s.userClient == nil {
		return errors.New("user-service client not initialized")
	}
	_, err := s.userClient.GetUserByID(context.Background(), &user.GetUserByIDRequest{Id: userID.String()})
	if err != nil {
		return errors.New("user_id not found in user-service")
	}
	// 若已存在记录返回错误
	_, err = s.getByUserID(userID)
	if err == nil { // 已存在
		return errors.New("auth record already exists")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	entity := &models.Auth{UserID: userID, Password: string(hash)}
	return s.repo.Create(entity)
}

func (s *AuthServiceImpl) Login(userID uuid.UUID, rawPassword string) (string, error) {
	authRec, err := s.getByUserID(userID)
	if err != nil {
		return "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(authRec.Password), []byte(rawPassword)) != nil {
		return "", errors.New("invalid credentials")
	}
	token, err := utils.GenerateToken(authRec.UserID.String(), s.tokenExpireMinutes)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *AuthServiceImpl) ValidateToken(token string) (uuid.UUID, error) {
	claims, err := utils.ParseToken(token)
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *AuthServiceImpl) CheckPassword(userID uuid.UUID, rawPassword string) (bool, error) {
	authRec, err := s.getByUserID(userID)
	if err != nil {
		return false, err
	}
	if bcrypt.CompareHashAndPassword([]byte(authRec.Password), []byte(rawPassword)) != nil {
		return false, nil
	}
	return true, nil
}

func (s *AuthServiceImpl) UpdatePassword(userID uuid.UUID, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(userID, string(hash))
}

// internal helper
func (s *AuthServiceImpl) getByUserID(userID uuid.UUID) (*models.Auth, error) {
	// 直接用 List + where 会更优，需要在 repo 添加方法；这里简化直接使用底层 db
	// 为保持 repository 规范性，扩展 AuthRepository: GetByUserID
	if ext, ok := s.repo.(interface {
		GetByUserID(uuid.UUID) (*models.Auth, error)
	}); ok {
		return ext.GetByUserID(userID)
	}
	return nil, errors.New("GetByUserID not implemented in repository")
}
