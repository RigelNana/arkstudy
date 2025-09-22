package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Database DatabaseConfig
	MinIO    MinIOConfig
}
type DatabaseConfig struct {
	DBUser           string
	DBPassword       string
	DBName           string
	DBHost           string
	DBPort           string
	JWTSecret        string
	MaterialGRPCAddr string
	LLMGRPCAddr      string
	OCRGRPCAddr      string
	JWTExpireMins    int
	// Kafka
	KafkaBrokers           string
	KafkaTopicOCRReqs      string
	KafkaTopicFileProcess  string
	KafkaGroupID           string
}

type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
}

func LoadConfig() *Config {
	// 在容器/ K8s 环境下通常没有 .env 文件，此处不应直接退出
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env")
	}
	return &Config{
		Database: DatabaseConfig{
			DBUser:            os.Getenv("DB_USER"),
			DBPassword:        os.Getenv("DB_PASSWORD"),
			DBName:            os.Getenv("DB_NAME"),
			DBHost:            os.Getenv("DB_HOST"),
			DBPort:            os.Getenv("DB_PORT"),
			JWTSecret:         os.Getenv("JWT_SECRET"),
			MaterialGRPCAddr:  os.Getenv("MATERIAL_GRPC_ADDR"),
			LLMGRPCAddr:       os.Getenv("LLM_GRPC_ADDR"),
			OCRGRPCAddr:       os.Getenv("OCR_GRPC_ADDR"),
			JWTExpireMins:     60,
			KafkaBrokers:           os.Getenv("KAFKA_BROKERS"),
			KafkaTopicOCRReqs:      os.Getenv("KAFKA_TOPIC_OCR_REQUESTS"),
			KafkaTopicFileProcess:  os.Getenv("KAFKA_TOPIC_FILE_PROCESSING"),
			KafkaGroupID:           os.Getenv("KAFKA_GROUP_ID"),
		},
		MinIO: MinIOConfig{
			Endpoint:        os.Getenv("MINIO_ENDPOINT"),
			AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY"),
			SecretAccessKey: os.Getenv("MINIO_SECRET_KEY"),
			UseSSL:          false,
			BucketName:      os.Getenv("MINIO_BUCKET_NAME"),
		},
	}
}
