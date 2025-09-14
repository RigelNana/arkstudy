package main

import (
	"log"
	"net"

	"github.com/RigelNana/arkstudy/proto/material"
	"github.com/RigelNana/arkstudy/services/material-service/config"
	"github.com/RigelNana/arkstudy/services/material-service/database"
	rpc "github.com/RigelNana/arkstudy/services/material-service/handler/grpc"
	"github.com/RigelNana/arkstudy/services/material-service/models"
	"github.com/RigelNana/arkstudy/services/material-service/repository"
	"github.com/RigelNana/arkstudy/services/material-service/service"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

func autoMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(&models.Material{}, &models.ProcessingResult{}); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
}

func main() {
	db := database.InitDB()
	autoMigrate(db)

	repo := repository.NewMaterialRepository(db)
	processingRepo := repository.NewProcessingResultRepository(db)
	config := config.LoadConfig()

	service, err := service.NewMaterialService(repo, processingRepo, config)
	if err != nil {
		log.Fatalf("failed to create material service: %v", err)
	}
	grpcServer := grpc.NewServer()
	material.RegisterMaterialServiceServer(grpcServer, rpc.NewMaterialRPCServer(service))
	port := config.Database.MaterialGRPCAddr
	if port == "" {
		port = "50053"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	log.Printf("Material gRPC server listening on %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve error: %v", err)
	}

}
