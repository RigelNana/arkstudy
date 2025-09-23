package main

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/RigelNana/arkstudy/proto/asr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestASRServiceIntegration(t *testing.T) {
	// 连接到 gRPC 服务器
	conn, err := grpc.Dial("localhost:50057", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := asr.NewASRServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 准备一个测试请求 (这里使用一个虚拟的视频 URL)
	// 在实际测试中，您可能需要上传一个文件到 MinIO 并获取其 URL
	req := &asr.ProcessVideoRequest{
		MaterialId: 1, // 使用一个示例 uint64 ID
		VideoUrl:   "https://example.com/video.mp4",
	}

	log.Println("Sending ProcessVideo request...")
	resp, err := client.ProcessVideo(ctx, req)
	if err != nil {
		t.Fatalf("ProcessVideo failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success to be true, but got false. Message: %s", resp.Message)
	}

	log.Printf("Received success message: %s", resp.Message)
	log.Println("Test passed!")
}
