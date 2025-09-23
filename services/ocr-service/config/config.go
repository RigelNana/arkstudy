package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	GRPCAddr string
	MinIO    MinIOConfig
	Paddle   PaddleOCRConfig
	Kafka    KafkaConfig
	Material MaterialCallbackConfig
	OpenAI   OpenAIConfig
}

type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
}

// PaddleOCR HTTP 服务配置
// 典型示例：Endpoint = http://paddleocr:8868/predict/ocr_system
type PaddleOCRConfig struct {
	Endpoint      string
	TimeoutSecond int
}

// Kafka consumer configuration
type KafkaConfig struct {
	Brokers string
	Topic   string
	GroupID string
}

// material-service callback (gRPC)
type MaterialCallbackConfig struct {
	Addr string // e.g., material-service:50053 (UpdateProcessingResult)
}

type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		GRPCAddr: getEnv("OCR_GRPC_ADDR", "50055"),
		MinIO: MinIOConfig{
			Endpoint:        os.Getenv("MINIO_ENDPOINT"),
			AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY"),
			SecretAccessKey: os.Getenv("MINIO_SECRET_KEY"),
			UseSSL:          false,
			BucketName:      os.Getenv("MINIO_BUCKET_NAME"),
		},
		Paddle: PaddleOCRConfig{
			Endpoint:      os.Getenv("PADDLE_OCR_ENDPOINT"),
			TimeoutSecond: getEnvInt("PADDLE_OCR_TIMEOUT", 20),
		},
		Kafka: KafkaConfig{
			Brokers: os.Getenv("KAFKA_BROKERS"),
			Topic:   getEnv("KAFKA_TOPIC_OCR_REQUESTS", "ocr.requests"),
			GroupID: getEnv("KAFKA_GROUP_ID", "ocr-worker"),
		},
		Material: MaterialCallbackConfig{
			Addr: getEnv("MATERIAL_GRPC_ADDR", "material-service:50053"),
		},
		OpenAI: OpenAIConfig{
			APIKey:  os.Getenv("OPENAI_API_KEY"),
			BaseURL: os.Getenv("OPENAI_BASE_URL"),
			Model:   getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		},
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Must[T any](v T, err error) T {
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}
	return v
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var out int
		_, err := fmt.Sscanf(v, "%d", &out)
		if err == nil {
			return out
		}
	}
	return def
}
