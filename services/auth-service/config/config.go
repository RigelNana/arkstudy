package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
)

type Config struct {
	DBUser     string
    DBPassword string
    DBHost     string
    DBPort     string
    DBName     string
}

func LoadConfig() *Config {
	err := godotenv.Load()
    if err != nil {
        log.Println("No .env file found, using system env")
    }
    return &Config{
        DBUser:     os.Getenv("DB_USER"),
        DBPassword: os.Getenv("DB_PASSWORD"),
        DBHost:     os.Getenv("DB_HOST"),
        DBPort:     os.Getenv("DB_PORT"),
        DBName:     os.Getenv("DB_NAME"),
    }
}