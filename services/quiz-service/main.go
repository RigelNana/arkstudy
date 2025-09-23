package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/RigelNana/arkstudy/pkg/metrics"
	grpcMetrics "github.com/RigelNana/arkstudy/pkg/metrics/grpc"
	pb "github.com/RigelNana/arkstudy/proto/quiz"
	"github.com/RigelNana/arkstudy/quiz-service/config"
	grpcHandler "github.com/RigelNana/arkstudy/quiz-service/handler/grpc"
	"github.com/RigelNana/arkstudy/quiz-service/repository"
	"github.com/RigelNana/arkstudy/quiz-service/service"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 初始化日志
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// 启动 Prometheus metrics 服务器
	metrics.StartMetricsServer("2112")
	logger.Info("Prometheus metrics server started on :2112")

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatalf("加载配置失败: %v", err)
	}

	logger.Infof("Quiz服务启动，配置: %+v", cfg)

	// 初始化数据库
	quizRepo, err := repository.NewQuizRepository(cfg.Database.DSN())
	if err != nil {
		logger.Fatalf("初始化数据库失败: %v", err)
	}
	logger.Info("数据库连接成功")

	// 初始化服务
	quizService := service.NewQuizService(cfg.OpenAI.APIKey, cfg.OpenAI.BaseURL, cfg.LLMService.Address, logger)

	// 启动gRPC服务器
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPC.Port))
	if err != nil {
		logger.Fatalf("gRPC监听失败: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor("quiz-service")),
		grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor("quiz-service")),
	)
	quizGRPCHandler := grpcHandler.NewQuizGRPCHandler(quizService, quizRepo, logger)
	pb.RegisterQuizServiceServer(grpcServer, quizGRPCHandler)

	// 启用反射，便于调试
	reflection.Register(grpcServer)

	logger.Infof("gRPC服务器启动在端口 %s", cfg.GRPC.Port)

	// 在goroutine中启动gRPC服务器
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("gRPC服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Info("Quiz服务正在关闭...")
	grpcServer.GracefulStop()
}
