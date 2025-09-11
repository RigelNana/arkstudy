package rpc

import (
	"context"
	"proto/user"
	"user-service/service"

	"github.com/google/uuid"
)

type UserRPCServer struct {
	user.UnimplementedUserServiceServer
	svc service.UserService
}

func NewUserRPCServer(svc service.UserService) *UserRPCServer { return &UserRPCServer{svc: svc} }

func (s *UserRPCServer) CreateUser(ctx context.Context, in *user.CreateUserRequest) (*user.CreateUserResponse, error) {
	u, err := s.svc.Create(in.Username, in.Email, in.Role, in.Description)
	if err != nil {
		return &user.CreateUserResponse{Success: false, Message: err.Error()}, nil
	}
	return &user.CreateUserResponse{Success: true, Message: "ok", User: &user.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description}}, nil
}

func (s *UserRPCServer) GetUserByID(ctx context.Context, in *user.GetUserByIDRequest) (*user.GetUserResponse, error) {
	id, err := uuid.Parse(in.Id)
	if err != nil {
		return &user.GetUserResponse{Found: false, Message: "invalid id"}, nil
	}
	u, err := s.svc.GetByID(id)
	if err != nil {
		return &user.GetUserResponse{Found: false, Message: err.Error()}, nil
	}
	return &user.GetUserResponse{Found: true, Message: "ok", User: &user.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description}}, nil
}

func (s *UserRPCServer) GetUserByUsername(ctx context.Context, in *user.GetUserByUsernameRequest) (*user.GetUserResponse, error) {
	u, err := s.svc.GetByUsername(in.Username)
	if err != nil {
		return &user.GetUserResponse{Found: false, Message: err.Error()}, nil
	}
	return &user.GetUserResponse{Found: true, Message: "ok", User: &user.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description}}, nil
}

func (s *UserRPCServer) GetUserByEmail(ctx context.Context, in *user.GetUserByEmailRequest) (*user.GetUserResponse, error) {
	u, err := s.svc.GetByEmail(in.Email)
	if err != nil {
		return &user.GetUserResponse{Found: false, Message: err.Error()}, nil
	}
	return &user.GetUserResponse{Found: true, Message: "ok", User: &user.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description}}, nil
}

func (s *UserRPCServer) ListUsers(ctx context.Context, in *user.ListUsersRequest) (*user.ListUsersResponse, error) {
	users, total, _ := s.svc.List(int(in.Limit), int(in.Offset))
	resp := &user.ListUsersResponse{Total: total}
	for _, u := range users {
		resp.Users = append(resp.Users, &user.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description})
	}
	return resp, nil
}
