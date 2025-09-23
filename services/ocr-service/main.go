package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"time"

	"strings"

	"github.com/RigelNana/arkstudy/pkg/metrics"
	grpcMetrics "github.com/RigelNana/arkstudy/pkg/metrics/grpc"
	"github.com/RigelNana/arkstudy/proto/ai"
	mpb "github.com/RigelNana/arkstudy/proto/material"
	"github.com/RigelNana/arkstudy/services/ocr-service/config"
	"github.com/RigelNana/arkstudy/services/ocr-service/service"
	kafka "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// ocrJob mirrors the schema published by material-service
type ocrJob struct {
	TaskID     string            `json:"task_id"`
	MaterialID string            `json:"material_id"`
	UserID     string            `json:"user_id"`
	FileURL    string            `json:"file_url"`
	FileType   string            `json:"file_type"`
	Options    map[string]string `json:"options"`
}

func main() {
	// 启动 Prometheus metrics 服务器
	metrics.StartMetricsServer("2112")
	log.Printf("Prometheus metrics server started on :2112")

	cfg := config.Load()
	svc, err := service.NewOCRService(cfg)
	if err != nil {
		log.Fatalf("init service: %v", err)
	}

	// Start Kafka consumer if configured
	if cfg.Kafka.Brokers != "" && cfg.Kafka.Topic != "" && cfg.Kafka.GroupID != "" {
		go startConsumer(cfg, svc)
	} else {
		log.Printf("Kafka consumer disabled (missing config)")
	}

	// Start gRPC server
	addr := cfg.GRPCAddr
	if addr == "" {
		addr = "50055"
	}
	lis, err := net.Listen("tcp", ":"+addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor("ocr-service")),
		grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor("ocr-service")),
	)
	ai.RegisterAIServiceServer(grpcServer, svc)
	// Enable server reflection
	reflection.Register(grpcServer)
	log.Printf("OCR gRPC server listening on %s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func startConsumer(cfg *config.Config, svc *service.OCRService) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  splitBrokers(cfg.Kafka.Brokers),
		GroupID:  cfg.Kafka.GroupID,
		Topic:    cfg.Kafka.Topic,
		MinBytes: 1,
		MaxBytes: 10 << 20,
	})
	defer r.Close()
	log.Printf("Kafka consumer started: topic=%s group=%s", cfg.Kafka.Topic, cfg.Kafka.GroupID)

	// Kafka writer for text.extracted topic
	textExtractedWriter := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  splitBrokers(cfg.Kafka.Brokers),
		Topic:    "text.extracted",
		Balancer: &kafka.LeastBytes{},
	})
	defer textExtractedWriter.Close()

	// material-service callback client
	conn, err := grpc.Dial(cfg.Material.Addr, grpc.WithInsecure())
	if err != nil {
		log.Printf("dial material-service: %v", err)
		return
	}
	defer conn.Close()
	mcli := mpb.NewMaterialServiceClient(conn)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		msg, err := r.FetchMessage(ctx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			log.Printf("kafka fetch: %v", err)
			time.Sleep(time.Second)
			continue
		}
		var job ocrJob
		if err := json.Unmarshal(msg.Value, &job); err != nil {
			log.Printf("bad job json: %v", err)
			_ = r.CommitMessages(context.Background(), msg)
			continue
		}
		// Run OCR via svc
		tctx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = svc.ProcessOCR(tctx, &ai.OCRRequest{TaskId: job.TaskID, FileUrl: job.FileURL, FileType: job.FileType})
		cancel2()
		if err != nil {
			log.Printf("ProcessOCR start err: %v", err)
		}

		// Poll until done or failed
		var content string
		var status mpb.ProcessingStatus = mpb.ProcessingStatus_FAILED
		deadline := time.Now().Add(10 * time.Minute)
		for time.Now().Before(deadline) {
			sctx, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
			st, err := svc.GetTaskStatus(sctx, &ai.TaskStatusRequest{TaskId: job.TaskID})
			cancel3()
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			if st.Status == ai.TaskStatus_COMPLETED {
				rctx, cancel4 := context.WithTimeout(context.Background(), 10*time.Second)
				resp, err := svc.ProcessOCR(rctx, &ai.OCRRequest{TaskId: job.TaskID})
				cancel4()
				if err == nil {
					content = resp.GetText()
					status = mpb.ProcessingStatus_COMPLETED
				}
				break
			}
			if st.Status == ai.TaskStatus_FAILED {
				status = mpb.ProcessingStatus_FAILED
				break
			}
			time.Sleep(2 * time.Second)
		}

		// Callback material-service
		uctx, cancel5 := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = mcli.UpdateProcessingResult(uctx, &mpb.UpdateProcessingResultRequest{
			TaskId:       job.TaskID,
			Status:       status,
			Content:      content,
			Metadata:     map[string]string{"source": "ocr-service"},
			ErrorMessage: "",
		})
		cancel5()
		if err != nil {
			log.Printf("update processing result: %v", err)
		} else if status == mpb.ProcessingStatus_COMPLETED {
			// Publish to text.extracted topic
			extractedPayload, _ := json.Marshal(map[string]string{
				"material_id": job.MaterialID,
				"user_id":     job.UserID,
				"text":        content,
				"source":      "ocr",
			})
			err = textExtractedWriter.WriteMessages(context.Background(), kafka.Message{
				Key:   []byte(job.MaterialID),
				Value: extractedPayload,
			})
			if err != nil {
				log.Printf("failed to write message to text.extracted topic: %v", err)
			}
		}

		// Commit offset
		_ = r.CommitMessages(context.Background(), msg)
	}
}

func splitBrokers(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
