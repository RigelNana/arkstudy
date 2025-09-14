package repository

import (
	"github.com/RigelNana/arkstudy/services/material-service/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MaterialRepository interface {
	BaseRepository[models.Material]
	GetByUserID(userID uuid.UUID, limit, offset int) ([]*models.Material, error)
	GetByUserIDWithPagination(userID uuid.UUID, page, pageSize int32) ([]*models.Material, int64, error)
	GetByStatus(status string, limit, offset int) ([]*models.Material, error)
	GetByUserIDAndStatus(userID uuid.UUID, status string, limit, offset int) ([]*models.Material, error)
	CountByUserID(userID uuid.UUID) (int64, error)
	CountByStatus(status string) (int64, error)
	UpdateStatus(id uuid.UUID, status string) error
}

type MaterialRepositoryImpl struct {
	*BaseRepositoryImpl[models.Material]
}

func NewMaterialRepository(db *gorm.DB) MaterialRepository {
	return &MaterialRepositoryImpl{
		BaseRepositoryImpl: NewBaseRepository[models.Material](db),
	}
}

func (r *MaterialRepositoryImpl) GetByUserID(userID uuid.UUID, limit, offset int) ([]*models.Material, error) {
	var materials []*models.Material
	err := r.db.Where("user_id = ?", userID).Limit(limit).Offset(offset).Find(&materials).Error
	if err != nil {
		return nil, err
	}
	return materials, nil
}

func (r *MaterialRepositoryImpl) GetByUserIDWithPagination(userID uuid.UUID, page, pageSize int32) ([]*models.Material, int64, error) {
	var materials []*models.Material
	var total int64

	// 计算 offset
	offset := (page - 1) * pageSize

	// 获取总数
	err := r.db.Model(&models.Material{}).Where("user_id = ?", userID).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err = r.db.Where("user_id = ?", userID).
		Limit(int(pageSize)).
		Offset(int(offset)).
		Order("created_at DESC").
		Find(&materials).Error
	if err != nil {
		return nil, 0, err
	}

	return materials, total, nil
}

func (r *MaterialRepositoryImpl) GetByStatus(status string, limit, offset int) ([]*models.Material, error) {
	var materials []*models.Material
	err := r.db.Where("status = ?", status).Limit(limit).Offset(offset).Find(&materials).Error
	if err != nil {
		return nil, err
	}
	return materials, nil
}

func (r *MaterialRepositoryImpl) GetByUserIDAndStatus(userID uuid.UUID, status string, limit, offset int) ([]*models.Material, error) {
	var materials []*models.Material
	err := r.db.Where("user_id = ? AND status = ?", userID, status).Limit(limit).Offset(offset).Find(&materials).Error
	if err != nil {
		return nil, err
	}
	return materials, nil
}

func (r *MaterialRepositoryImpl) CountByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Material{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *MaterialRepositoryImpl) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Material{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *MaterialRepositoryImpl) UpdateStatus(id uuid.UUID, status string) error {
	return r.db.Model(&models.Material{}).Where("id = ?", id).Update("status", status).Error
}
