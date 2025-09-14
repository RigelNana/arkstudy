package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Material struct {
	Base
	UserID           uuid.UUID      `gorm:"type:uuid;not null;index"`
	Title            string         `gorm:"not null"`
	OriginalFilename string         `gorm:"not null"`
	FileType         string         `gorm:"not null"`
	SizeBytes        int64          `gorm:"not null"`
	Status           string         `gorm:"default:'pending'"`
	MinioBucket      string         `gorm:"not null"`
	MinioObjectName  string         `gorm:"not null"`
	Metadata         datatypes.JSON `gorm:"type:jsonb"`
}
