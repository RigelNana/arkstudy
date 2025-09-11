package rpc

import (
	"context"
	pb "github.com/RigelNana/arkstudy/services/user-service/rpc"
	"github.com/RigelNana/arkstudy/services/user-service/service"

	"github.com/google/uuid"
)

type UserRPCServer struct {
	pb.UnimplementedUserServiceServer
	svc service.UserService
}

func NewUserRPCServer(svc service.UserService) *UserRPCServer { return &UserRPCServer{svc: svc} }

func (s *UserRPCServer) CreateUser(ctx context.Context, in *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	u, err := s.svc.Create(in.Username, in.Email, in.Role, in.Description)
	if err != nil {
		return &pb.CreateUserResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.CreateUserResponse{Success: true, Message: "ok", User: &pb.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description}}, nil
}

func (s *UserRPCServer) GetUserByID(ctx context.Context, in *pb.GetUserByIDRequest) (*pb.GetUserResponse, error) {
	id, err := uuid.Parse(in.Id)
	if err != nil {
		return &pb.GetUserResponse{Found: false, Message: "invalid id"}, nil
	}
	u, err := s.svc.GetByID(id)
	if err != nil {
		return &pb.GetUserResponse{Found: false, Message: err.Error()}, nil
	}
	return &pb.GetUserResponse{Found: true, Message: "ok", User: &pb.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description}}, nil
}

func (s *UserRPCServer) GetUserByUsername(ctx context.Context, in *pb.GetUserByUsernameRequest) (*pb.GetUserResponse, error) {
	u, err := s.svc.GetByUsername(in.Username)
	if err != nil {
		return &pb.GetUserResponse{Found: false, Message: err.Error()}, nil
	}
	return &pb.GetUserResponse{Found: true, Message: "ok", User: &pb.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description}}, nil
}

func (s *UserRPCServer) GetUserByEmail(ctx context.Context, in *pb.GetUserByEmailRequest) (*pb.GetUserResponse, error) {
	u, err := s.svc.GetByEmail(in.Email)
	if err != nil {
		return &pb.GetUserResponse{Found: false, Message: err.Error()}, nil
	}
	return &pb.GetUserResponse{Found: true, Message: "ok", User: &pb.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description}}, nil
}

func (s *UserRPCServer) ListUsers(ctx context.Context, in *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	users, total, _ := s.svc.List(int(in.Limit), int(in.Offset))
	resp := &pb.ListUsersResponse{Total: total}
	for _, u := range users {
		resp.Users = append(resp.Users, &pb.UserInfo{Id: u.ID.String(), Username: u.Username, Email: u.Email, Role: u.Role, Description: u.Description})
	}
	return resp, nil
}
