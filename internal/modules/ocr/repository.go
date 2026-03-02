package ocr

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"parkieee/pkg/types"
)

type ocrJobRepository struct{ db *gorm.DB }
type ocrResultRepository struct{ db *gorm.DB }
type ocrReviewLogRepository struct{ db *gorm.DB }

func NewOCRJobRepository(db *gorm.DB) OCRJobRepositoryPort {
	return &ocrJobRepository{db: db}
}

func NewOCRResultRepository(db *gorm.DB) OCRResultRepositoryPort {
	return &ocrResultRepository{db: db}
}

func NewOCRReviewLogRepository(db *gorm.DB) OCRReviewLogRepositoryPort {
	return &ocrReviewLogRepository{db: db}
}

func (r *ocrJobRepository) Create(ctx context.Context, job *OCRJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("create ocr_job: %w", err)
	}
	return nil
}

func (r *ocrJobRepository) UpdateStatus(ctx context.Context, job *OCRJob) error {
	if err := r.db.WithContext(ctx).Save(job).Error; err != nil {
		return fmt.Errorf("update ocr_job status: %w", err)
	}
	return nil
}

func (r *ocrResultRepository) Create(ctx context.Context, result *OCRResult) error {
	if err := r.db.WithContext(ctx).Create(result).Error; err != nil {
		return fmt.Errorf("create ocr_result: %w", err)
	}
	return nil
}

// FindEntryResultByTransactionID finds the OCR result linked to the entry job of a transaction.
// It joins ocr_jobs to filter by photo_type = 'entry' and picks the latest completed one.
func (r *ocrResultRepository) FindEntryResultByTransactionID(ctx context.Context, transactionID uuid.UUID) (*OCRResult, error) {
	var result OCRResult
	err := r.db.WithContext(ctx).
		Joins("JOIN ocr_jobs ON ocr_jobs.id = ocr_results.ocr_job_id").
		Where("ocr_jobs.transaction_id = ? AND ocr_jobs.photo_type = 'entry' AND ocr_jobs.status = 'completed'", transactionID).
		Order("ocr_results.created_at DESC").
		First(&result).Error
	if err != nil {
		return nil, fmt.Errorf("find entry ocr result: %w", err)
	}
	return &result, nil
}

func (r *ocrReviewLogRepository) Create(ctx context.Context, log *OCRReviewLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("create ocr_review_log: %w", err)
	}
	return nil
}

func (r *ocrResultRepository) FindSummaryByTransactionID(ctx context.Context, transactionID uuid.UUID) ([]OCRResultWithJob, error) {
	var rows []struct {
		PhotoType       string
		PlateDetected   string
		ActualPlate     string
		IsMatch         *bool
		Confidence      string
		OutputImagePath *string
		OutputImageURL  *string
		IsVerified      bool
	}

	err := r.db.WithContext(ctx).
		Table("ocr_results").
		Select("ocr_jobs.photo_type, ocr_results.plate_detected, ocr_results.actual_plate, ocr_results.is_match, ocr_results.confidence, ocr_jobs.output_image_path, ocr_jobs.output_image_url, ocr_results.is_verified").
		Joins("JOIN ocr_jobs ON ocr_jobs.id = ocr_results.ocr_job_id").
		Where("ocr_jobs.transaction_id = ? AND ocr_jobs.status = 'completed'", transactionID).
		Order("ocr_results.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find ocr summary: %w", err)
	}

	out := make([]OCRResultWithJob, 0, len(rows))
	for _, row := range rows {
		conf, _ := decimal.NewFromString(row.Confidence)
		out = append(out, OCRResultWithJob{
			PhotoType:       types.OCRPhotoType(row.PhotoType),
			PlateDetected:   row.PlateDetected,
			ActualPlate:     row.ActualPlate,
			IsMatch:         row.IsMatch,
			Confidence:      conf,
			OutputImagePath: row.OutputImagePath,
			OutputImageURL:  row.OutputImageURL,
			IsVerified:      row.IsVerified,
		})
	}
	return out, nil
}

// transactionUpdater updates the vehicle_id on a transaction row directly.
// Kept minimal to avoid importing the full transaction module (circular deps).
func updateTransactionVehicle(ctx context.Context, db *gorm.DB, transactionID, vehicleID uuid.UUID) error {
	if err := db.WithContext(ctx).
		Table("transactions").
		Where("id = ?", transactionID).
		Update("vehicle_id", vehicleID).Error; err != nil {
		return fmt.Errorf("update transaction vehicle_id: %w", err)
	}
	return nil
}
