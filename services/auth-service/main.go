package main

import (
	"log"
	"net"
	"os"

	"github.com/RigelNana/arkstudy/pkg/metrics"
	grpcMetrics "github.com/RigelNana/arkstudy/pkg/metrics/grpc"
	pb "github.com/RigelNana/arkstudy/proto/auth"
	"github.com/RigelNana/arkstudy/services/auth-service/database"
	"github.com/RigelNana/arkstudy/services/auth-service/handler/rpc"
	"github.com/RigelNana/arkstudy/services/auth-service/models"
	"github.com/RigelNana/arkstudy/services/auth-service/repository"
	"github.com/RigelNana/arkstudy/services/auth-service/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"
)

func autoMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(&models.Auth{}); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
}

func main() {
	// 启动 Prometheus metrics 服务器
	metrics.StartMetricsServer("2112")
	log.Printf("Prometheus metrics server started on :2112")

	db := database.InitDB()
	autoMigrate(db)

	repo := repository.NewAuthRepository(db)
	svc := service.NewAuthService(repo)

	// 创建带监控的 gRPC 服务器
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor("auth-service")),
		grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor("auth-service")),
	)

	pb.RegisterAuthServiceServer(grpcServer, rpc.NewAuthRPCServer(svc))
	// Enable server reflection for grpcui/insomnia
	reflection.Register(grpcServer)

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
