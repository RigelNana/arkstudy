package database

import (
	"github.com/RigelNana/arkstudy/services/material-service/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() *gorm.DB {
	config := config.LoadConfig()
	dsn := "host=" + config.Database.DBHost + " user=" + config.Database.DBUser + " password=" + config.Database.DBPassword + " dbname=" + config.Database.DBName + " port=" + config.Database.DBPort + " sslmode=disable TimeZone=Asia/Shanghai"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}
	return db
}
