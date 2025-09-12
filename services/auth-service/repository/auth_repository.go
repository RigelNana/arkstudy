package repository

import (
	"github.com/RigelNana/arkstudy/services/auth-service/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthRepository 定义授权/认证数据访问接口
// 目前 Auth 仅存储密码哈希，可根据需要扩展（例如加入用户关联、令牌等）
type AuthRepository interface {
	BaseRepository[models.Auth]
	// UpdatePassword 按 user_id 更新密码哈希
	UpdatePassword(userID uuid.UUID, newHashedPassword string) error
	GetByUserID(userID uuid.UUID) (*models.Auth, error)
}

// AuthRepositoryImpl 实现 AuthRepository
type AuthRepositoryImpl struct {
	*BaseRepositoryImpl[models.Auth]
}

// NewAuthRepository 构造函数
func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &AuthRepositoryImpl{BaseRepositoryImpl: NewBaseRepository[models.Auth](db)}
}

// UpdatePassword 仅更新 password 字段，返回记录不存在错误
func (r *AuthRepositoryImpl) UpdatePassword(userID uuid.UUID, newHashedPassword string) error {
	result := r.db.Model(&models.Auth{}).Where("user_id = ?", userID).Update("password", newHashedPassword)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *AuthRepositoryImpl) GetByUserID(userID uuid.UUID) (*models.Auth, error) {
	var auth models.Auth
	err := r.db.Where("user_id = ?", userID).First(&auth).Error
	if err != nil {
		return nil, err
	}
	return &auth, nil
}
