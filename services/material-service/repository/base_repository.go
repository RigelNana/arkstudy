package repository

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseRepository[T any] interface {
	Create(entity *T) error
	GetByID(id uuid.UUID) (*T, error)
	Update(entity *T) error
	Delete(id uuid.UUID) error
	List(limit, offset int) ([]*T, error)
	Count() (int64, error)
}

type BaseRepositoryImpl[T any] struct {
	db *gorm.DB
}

func NewBaseRepository[T any](db *gorm.DB) *BaseRepositoryImpl[T] {
	return &BaseRepositoryImpl[T]{
		db: db,
	}
}

func (r *BaseRepositoryImpl[T]) Create(entity *T) error {
	return r.db.Create(entity).Error
}

func (r *BaseRepositoryImpl[T]) GetByID(id uuid.UUID) (*T, error) {
	var entity T
	err := r.db.First(&entity, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *BaseRepositoryImpl[T]) Update(entity *T) error {
	return r.db.Save(entity).Error
}

func (r *BaseRepositoryImpl[T]) Delete(id uuid.UUID) error {
	var entity T
	return r.db.Delete(&entity, "id = ?", id).Error
}

func (r *BaseRepositoryImpl[T]) List(limit, offset int) ([]*T, error) {
	var entities []*T
	err := r.db.Limit(limit).Offset(offset).Find(&entities).Error
	return entities, err
}

func (r *BaseRepositoryImpl[T]) Count() (int64, error) {
	var count int64
	var entity T
	err := r.db.Model(&entity).Count(&count).Error
	return count, err
}
