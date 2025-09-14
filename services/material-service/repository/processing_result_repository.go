package repository

import (
	"github.com/RigelNana/arkstudy/services/material-service/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProcessingResultRepository interface {
	BaseRepository[models.ProcessingResult]
	GetByTaskID(taskID string) (*models.ProcessingResult, error)
	GetByMaterialID(materialID uuid.UUID, limit, offset int) ([]*models.ProcessingResult, error)
	GetByMaterialIDAndType(materialID uuid.UUID, processType string) (*models.ProcessingResult, error)
	GetByMaterialIDWithPagination(materialID uuid.UUID, page, pageSize int32) ([]*models.ProcessingResult, int64, error)
	GetByStatus(status string, limit, offset int) ([]*models.ProcessingResult, error)
	UpdateByTaskID(taskID string, updates map[string]interface{}) error
	CountByMaterialID(materialID uuid.UUID) (int64, error)
	CountByStatus(status string) (int64, error)
}

type ProcessingResultRepositoryImpl struct {
	*BaseRepositoryImpl[models.ProcessingResult]
}

func NewProcessingResultRepository(db *gorm.DB) ProcessingResultRepository {
	return &ProcessingResultRepositoryImpl{
		BaseRepositoryImpl: NewBaseRepository[models.ProcessingResult](db),
	}
}

func (r *ProcessingResultRepositoryImpl) GetByTaskID(taskID string) (*models.ProcessingResult, error) {
	var result models.ProcessingResult
	err := r.db.Preload("Material").Where("task_id = ?", taskID).First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *ProcessingResultRepositoryImpl) GetByMaterialID(materialID uuid.UUID, limit, offset int) ([]*models.ProcessingResult, error) {
	var results []*models.ProcessingResult
	err := r.db.Where("material_id = ?", materialID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *ProcessingResultRepositoryImpl) GetByMaterialIDAndType(materialID uuid.UUID, processType string) (*models.ProcessingResult, error) {
	var result models.ProcessingResult
	err := r.db.Where("material_id = ? AND type = ?", materialID, processType).
		Order("created_at DESC").
		First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *ProcessingResultRepositoryImpl) GetByMaterialIDWithPagination(materialID uuid.UUID, page, pageSize int32) ([]*models.ProcessingResult, int64, error) {
	var results []*models.ProcessingResult
	var total int64

	// 计算 offset
	offset := (page - 1) * pageSize

	// 获取总数
	err := r.db.Model(&models.ProcessingResult{}).Where("material_id = ?", materialID).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	err = r.db.Where("material_id = ?", materialID).
		Order("created_at DESC").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Find(&results).Error
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (r *ProcessingResultRepositoryImpl) GetByStatus(status string, limit, offset int) ([]*models.ProcessingResult, error) {
	var results []*models.ProcessingResult
	err := r.db.Where("status = ?", status).
		Order("created_at ASC"). // 待处理的任务按创建时间正序
		Limit(limit).
		Offset(offset).
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *ProcessingResultRepositoryImpl) UpdateByTaskID(taskID string, updates map[string]interface{}) error {
	return r.db.Model(&models.ProcessingResult{}).Where("task_id = ?", taskID).Updates(updates).Error
}

func (r *ProcessingResultRepositoryImpl) CountByMaterialID(materialID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.ProcessingResult{}).Where("material_id = ?", materialID).Count(&count).Error
	return count, err
}

func (r *ProcessingResultRepositoryImpl) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&models.ProcessingResult{}).Where("status = ?", status).Count(&count).Error
	return count, err
}
