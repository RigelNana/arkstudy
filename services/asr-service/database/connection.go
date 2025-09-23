package database

import (
	"log"

	"github.com/RigelNana/arkstudy/services/asr-service/config"
	"github.com/RigelNana/arkstudy/services/asr-service/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() *gorm.DB {
	config := config.LoadConfig()
	dsn := "host=" + config.DBHost + " user=" + config.DBUser + " password=" + config.DBPassword + " dbname=" + config.DBName + " port=" + config.DBPort + " sslmode=disable TimeZone=UTC"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}

	// Auto migrate the schema
	err = db.AutoMigrate(&models.ASRSegment{})
	if err != nil {
		log.Printf("failed to migrate ASRSegment table: %v", err)
	}

	// Enable vector extension if needed
	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")

	log.Println("Database connected and migrated successfully")
	DB = db
	return db
}
