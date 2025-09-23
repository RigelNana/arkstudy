package main

import (
	"log"
	"net"

	"github.com/RigelNana/arkstudy/pkg/metrics"
	grpcMetrics "github.com/RigelNana/arkstudy/pkg/metrics/grpc"
	"github.com/RigelNana/arkstudy/proto/asr"
	"github.com/RigelNana/arkstudy/services/asr-service/config"
	"github.com/RigelNana/arkstudy/services/asr-service/database"
	grpcHandler "github.com/RigelNana/arkstudy/services/asr-service/handler/grpc"
	"github.com/RigelNana/arkstudy/services/asr-service/service"

	"google.golang.org/grpc"
)

func main() {
	// 启动 Prometheus metrics 服务器
	metrics.StartMetricsServer("2112")
	log.Printf("Prometheus metrics server started on :2112")

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	db := database.InitDB()
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// Initialize ASR service
	asrService := service.NewASRService(cfg)

	// Create gRPC server
	s := grpc.NewServer(
		grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor("asr-service")),
		grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor("asr-service")),
	)

	// Register ASR service
	asrServer := grpcHandler.NewASRServer(asrService)
	asr.RegisterASRServiceServer(s, asrServer)

	// Listen on the configured port
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("ASR gRPC service starting on port %s", cfg.GRPCPort)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
