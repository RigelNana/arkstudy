package main

import (
	"log"
	"net"
	"os"

	"github.com/RigelNana/arkstudy/pkg/metrics"
	grpcMetrics "github.com/RigelNana/arkstudy/pkg/metrics/grpc"
	"github.com/RigelNana/arkstudy/proto/user"
	"github.com/RigelNana/arkstudy/services/user-service/database"
	urpc "github.com/RigelNana/arkstudy/services/user-service/handler/rpc"
	"github.com/RigelNana/arkstudy/services/user-service/models"
	"github.com/RigelNana/arkstudy/services/user-service/repository"
	"github.com/RigelNana/arkstudy/services/user-service/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"
)

func autoMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(&models.User{}); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
}

func main() {
	// 启动 Prometheus metrics 服务器
	metrics.StartMetricsServer("2112")
	log.Printf("Prometheus metrics server started on :2112")

	db := database.InitDB()
	autoMigrate(db)

	repo := repository.NewUserRepository(db)
	svc := service.NewUserService(repo)

	// 创建带监控的 gRPC 服务器
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor("user-service")),
		grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor("user-service")),
	)

	user.RegisterUserServiceServer(grpcServer, urpc.NewUserRPCServer(svc))
	// Enable server reflection
	reflection.Register(grpcServer)

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
