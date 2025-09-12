package rpc

import (
	"context"
	"errors"

	pb "github.com/RigelNana/arkstudy/proto/auth"
	"github.com/RigelNana/arkstudy/services/auth-service/service"

	"github.com/google/uuid"
)

type AuthRPCServer struct {
	pb.UnimplementedAuthServiceServer
	svc service.AuthService
}

func NewAuthRPCServer(svc service.AuthService) *AuthRPCServer { return &AuthRPCServer{svc: svc} }

// Register 使用新的 RegisterRequest(user_id + password)
func (s *AuthRPCServer) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if in == nil || in.UserId == "" || in.Password == "" {
		return &pb.RegisterResponse{Success: false, Message: "missing user_id or password"}, nil
	}
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return &pb.RegisterResponse{Success: false, Message: "invalid user_id format"}, nil
	}
	if err := s.svc.Register(userID, in.Password); err != nil {
		return &pb.RegisterResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.RegisterResponse{Success: true, Message: "ok"}, nil
}

func (s *AuthRPCServer) Login(ctx context.Context, in *pb.LoginRequest) (*pb.LoginResponse, error) {
	if in == nil || in.UserId == "" || in.Password == "" {
		return &pb.LoginResponse{Success: false, Message: "missing user_id or password"}, nil
	}
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return &pb.LoginResponse{Success: false, Message: "invalid user_id format"}, nil
	}
	token, err := s.svc.Login(userID, in.Password)
	if err != nil {
		return &pb.LoginResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.LoginResponse{Success: true, Token: token, Message: "ok"}, nil
}

func (s *AuthRPCServer) ValidateToken(ctx context.Context, in *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	if in == nil || in.Token == "" {
		return &pb.ValidateTokenResponse{Valid: false, Message: "missing token"}, nil
	}
	userID, err := s.svc.ValidateToken(in.Token)
	if err != nil {
		return &pb.ValidateTokenResponse{Valid: false, Message: err.Error()}, nil
	}
	return &pb.ValidateTokenResponse{Valid: true, UserId: userID.String(), Message: "ok"}, nil
}

func (s *AuthRPCServer) CheckPassword(ctx context.Context, in *pb.CheckPasswordRequest) (*pb.CheckPasswordResponse, error) {
	if in == nil || in.UserId == "" || in.Password == "" {
		return &pb.CheckPasswordResponse{Valid: false}, nil
	}
	uid, err := uuid.Parse(in.UserId)
	if err != nil {
		return &pb.CheckPasswordResponse{Valid: false}, nil
	}
	valid, err := s.svc.CheckPassword(uid, in.Password)
	if err != nil && !errors.Is(err, errors.New("invalid credentials")) {
		return &pb.CheckPasswordResponse{Valid: false}, nil
	}
	return &pb.CheckPasswordResponse{Valid: valid}, nil
}
