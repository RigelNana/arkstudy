package repository

import (
	"github.com/RigelNana/arkstudy/services/asr-service/database"
	"github.com/RigelNana/arkstudy/services/asr-service/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ASRRepository struct {
	db *gorm.DB
}

func NewASRRepository() *ASRRepository {
	return &ASRRepository{
		db: database.DB,
	}
}

// CreateSegments creates multiple ASR segments in database
func (r *ASRRepository) CreateSegments(segments []models.ASRSegment) error {
	return r.db.Create(&segments).Error
}

// GetSegmentsByMaterialID retrieves segments by material ID
func (r *ASRRepository) GetSegmentsByMaterialID(materialID string) ([]models.ASRSegment, error) {
	var segments []models.ASRSegment
	err := r.db.Where("material_id = ?", materialID).
		Order("segment_index ASC").
		Find(&segments).Error
	return segments, err
}

// GetSegmentsByUserID retrieves segments by user ID
func (r *ASRRepository) GetSegmentsByUserID(userID uuid.UUID) ([]models.ASRSegment, error) {
	var segments []models.ASRSegment
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&segments).Error
	return segments, err
}

// SearchSegmentsByText performs text search on segments
func (r *ASRRepository) SearchSegmentsByText(userID uuid.UUID, materialID, query string, limit int) ([]models.ASRSegment, error) {
	var segments []models.ASRSegment

	dbQuery := r.db.Where("user_id = ?", userID)

	if materialID != "" {
		dbQuery = dbQuery.Where("material_id = ?", materialID)
	}

	err := dbQuery.Where("text ILIKE ?", "%"+query+"%").
		Limit(limit).
		Order("start_time ASC").
		Find(&segments).Error

	return segments, err
}

// UpdateSegmentEmbedding updates the embedding for a segment
func (r *ASRRepository) UpdateSegmentEmbedding(segmentID uuid.UUID, embedding []float64) error {
	return r.db.Model(&models.ASRSegment{}).
		Where("id = ?", segmentID).
		Update("embedding", embedding).Error
}

// DeleteSegmentsByMaterialID deletes all segments for a material
func (r *ASRRepository) DeleteSegmentsByMaterialID(materialID string) error {
	return r.db.Where("material_id = ?", materialID).
		Delete(&models.ASRSegment{}).Error
}
