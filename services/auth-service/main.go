package main

import (
	"github.com/RigelNana/arkstudy/services/auth-service/database"
	"github.com/RigelNana/arkstudy/services/auth-service/handler/rpc"
	"github.com/RigelNana/arkstudy/services/auth-service/models"
	"github.com/RigelNana/arkstudy/services/auth-service/repository"
	pb "github.com/RigelNana/arkstudy/services/auth-service/rpc"
	"github.com/RigelNana/arkstudy/services/auth-service/service"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

func autoMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(&models.Auth{}); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
}

func main() {
	db := database.InitDB()
	autoMigrate(db)

	repo := repository.NewAuthRepository(db)
	svc := service.NewAuthService(repo)
	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, rpc.NewAuthRPCServer(svc))

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("Auth gRPC server listening on %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve error: %v", err)
	}
}
