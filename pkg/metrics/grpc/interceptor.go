package grpc

import (
	"context"
	"time"

	"github.com/RigelNana/arkstudy/pkg/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor 为 gRPC 服务添加 Prometheus 指标
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		// 记录指标
		statusCode := "success"
		if err != nil {
			st, _ := status.FromError(err)
			statusCode = st.Code().String()
		}

		metrics.RecordRequest(serviceName, info.FullMethod, statusCode, time.Since(start))
		return resp, err
	}
}

// StreamServerInterceptor 为 gRPC 流添加 Prometheus 指标
func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)

		statusCode := "success"
		if err != nil {
			st, _ := status.FromError(err)
			statusCode = st.Code().String()
		}

		metrics.RecordRequest(serviceName, info.FullMethod, statusCode, time.Since(start))
		return err
	}
}
