package main

import (
	"log"
	"net"
	"os"
	"github.com/RigelNana/arkstudy/services/user-service/database"
	urpc "github.com/RigelNana/arkstudy/services/user-service/handler/rpc"
	"github.com/RigelNana/arkstudy/services/user-service/models"
	"github.com/RigelNana/arkstudy/services/user-service/repository"
	upb "github.com/RigelNana/arkstudy/services/user-service/rpc"
	"github.com/RigelNana/arkstudy/services/user-service/service"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

func autoMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(&models.User{}); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
}

func main() {
	db := database.InitDB()
	autoMigrate(db)

	repo := repository.NewUserRepository(db)
	svc := service.NewUserService(repo)

	grpcServer := grpc.NewServer()
	upb.RegisterUserServiceServer(grpcServer, urpc.NewUserRPCServer(svc))

	port := os.Getenv("USER_GRPC_PORT")
	if port == "" {
		port = "50052"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	log.Printf("User gRPC server listening on %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve error: %v", err)
	}
}
