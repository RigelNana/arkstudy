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
	JWTExpireMins    int
}

type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
	return &Config{
		Database: DatabaseConfig{
			DBUser:           os.Getenv("DB_USER"),
			DBPassword:       os.Getenv("DB_PASSWORD"),
			DBName:           os.Getenv("DB_NAME"),
			DBHost:           os.Getenv("DB_HOST"),
			DBPort:           os.Getenv("DB_PORT"),
			JWTSecret:        os.Getenv("JWT_SECRET"),
			MaterialGRPCAddr: os.Getenv("MATERIAL_GRPC_ADDR"),
			JWTExpireMins:    60,
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
