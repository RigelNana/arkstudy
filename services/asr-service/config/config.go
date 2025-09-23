package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database config
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
	DBName     string

	// Server config
	Port     string
	GRPCPort string
	Host     string

	// OpenAI config for Whisper
	OpenAIAPIKey  string
	OpenAIBaseURL string
	OpenAIModel   string

	// FFmpeg config
	FFmpegBinaryPath string
	TempDir          string
	AudioFormat      string

	// Storage config
	MaxFileSize    string
	AllowedFormats string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system env")
	}

	return &Config{
		// Database
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBName:     getEnv("DB_NAME", "arkdb"),

		// Server
		Port:     getEnv("ASR_PORT", "50057"),
		GRPCPort: getEnv("GRPC_PORT", "50057"),
		Host:     getEnv("ASR_HOST", "0.0.0.0"),

		// OpenAI
		OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
		OpenAIBaseURL: getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:   getEnv("OPENAI_MODEL", "whisper-1"),

		// FFmpeg
		FFmpegBinaryPath: getEnv("FFMPEG_BINARY_PATH", "ffmpeg"),
		TempDir:          getEnv("TEMP_DIR", "/tmp/asr"),
		AudioFormat:      getEnv("AUDIO_FORMAT", "wav"),

		// Storage
		MaxFileSize:    getEnv("MAX_FILE_SIZE", "104857600"), // 100MB
		AllowedFormats: getEnv("ALLOWED_FORMATS", "mp4,avi,mov,mkv,webm"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
