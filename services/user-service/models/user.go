package models

type User struct {
	Base
	Username    string `gorm:"uniqueIndex;not null"`
	Email       string `gorm:"uniqueIndex;not null"`
	Role        string `gorm:"default:'student'"`
	Description string `gorm:"type:text"`
}
