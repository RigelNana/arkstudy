package models

import "github.com/google/uuid"

// Auth 仅存储与认证相关的敏感数据（方案B：不在此保存用户名/邮箱等用户资料）
type Auth struct {
	Base
	UserID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
	Password string    `gorm:"not null"` // bcrypt hash
}
