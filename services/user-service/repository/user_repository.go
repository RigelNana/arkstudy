package repository

import (
	"user-service/models"

	"gorm.io/gorm"
)

type UserRepository interface {
	BaseRepository[models.User]
	GetByUsername(username string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByUsernameOrEmail(usernameOrEmail string) (*models.User, error)
}

type UserRepositoryImpl struct {
	*BaseRepositoryImpl[models.User]
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &UserRepositoryImpl{
		BaseRepositoryImpl: NewBaseRepository[models.User](db),
	}
}

func (r *UserRepositoryImpl) GetByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) GetByUsernameOrEmail(usernameOrEmail string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ? OR email = ?", usernameOrEmail, usernameOrEmail).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
