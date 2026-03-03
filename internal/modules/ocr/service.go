package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	vehicleDomain "parkieee/internal/modules/vehicle"
	zoneDomain "parkieee/internal/modules/zone"
	"parkieee/pkg/logger"
	"parkieee/pkg/types"
)

type service struct {
	jobRepo             OCRJobRepositoryPort
	resultRepo          OCRResultRepositoryPort
	reviewLogRepo       OCRReviewLogRepositoryPort
	vehicleSvc          vehicleDomain.ServicePort
	zoneRepo            zoneDomain.ZoneRepositoryPort
	adapter             *httpAdapter
	log                 logger.Logger
	db                  *gorm.DB
	maxRetries          int
	enabled             bool
	autoAcceptThreshold float64
}

func NewService(
	db *gorm.DB,
	jobRepo OCRJobRepositoryPort,
	resultRepo OCRResultRepositoryPort,
	reviewLogRepo OCRReviewLogRepositoryPort,
	vehicleSvc vehicleDomain.ServicePort,
	zoneRepo zoneDomain.ZoneRepositoryPort,
	log logger.Logger,
	apiURL string,
	timeout time.Duration,
	maxRetries int,
	enabled bool,
	autoAcceptThreshold float64,
) ServicePort {
	return &service{
		db:                  db,
		jobRepo:             jobRepo,
		resultRepo:          resultRepo,
		reviewLogRepo:       reviewLogRepo,
		vehicleSvc:          vehicleSvc,
		zoneRepo:            zoneRepo,
		adapter:             newHTTPAdapter(apiURL, timeout),
		log:                 log,
		maxRetries:          maxRetries,
		enabled:             enabled,
		autoAcceptThreshold: autoAcceptThreshold,
	}
}

// DispatchOCRJob is the fire-and-forget entry point called from a goroutine.
// zoneID is used to resolve the default vehicle type configured on the zone.
// photoType distinguishes entry vs exit so plate-match logic runs on exit jobs.
func (s *service) DispatchOCRJob(ctx context.Context, transactionID uuid.UUID, imagePath string, zoneID uuid.UUID, photoType types.OCRPhotoType) {
	if !s.enabled {
		s.log.Info(ctx, "ocr: disabled, skipping job", "transaction_id", transactionID)
		return
	}
	if imagePath == "" {
		s.log.Info(ctx, "ocr: no image path provided, skipping job", "transaction_id", transactionID)
		return
	}

	now := time.Now()
	job := &OCRJob{
		ID:            uuid.New(),
		TransactionID: transactionID,
		ImageURL:      imagePath,
		PhotoType:     photoType,
		Status:        types.OCRJobQueued,
		RetryCount:    0,
		QueuedAt:      now,
	}

	if err := s.jobRepo.Create(ctx, job); err != nil {
		s.log.Error(ctx, "ocr: failed to create job record",
			"transaction_id", transactionID,
			"error", err,
		)
		return
	}

	s.processJob(ctx, job, imagePath, zoneID)
}

func (s *service) processJob(ctx context.Context, job *OCRJob, imagePath string, zoneID uuid.UUID) {
	now := time.Now()
	job.Status = types.OCRJobProcessing
	job.StartedAt = &now
	if err := s.jobRepo.UpdateStatus(ctx, job); err != nil {
		s.log.Error(ctx, "ocr: failed to mark job as processing", "job_id", job.ID, "error", err)
	}

	if err := s.adapter.ping(ctx); err != nil {
		s.log.Warn(ctx, "ocr: python service unavailable, skipping job",
			"job_id", job.ID,
			"transaction_id", job.TransactionID,
			"error", err,
		)
		s.markJobSkipped(ctx, job, "python ocr service unreachable: "+err.Error())
		return
	}

	var result *detectPlateResponse
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			s.log.Info(ctx, "ocr: retrying",
				"job_id", job.ID,
				"attempt", attempt,
				"max_retries", s.maxRetries,
			)
			job.RetryCount = attempt
			_ = s.jobRepo.UpdateStatus(ctx, job)
		}

		result, lastErr = s.adapter.callDetectPlate(ctx, imagePath)
		if lastErr == nil {
			break
		}

		s.log.Warn(ctx, "ocr: attempt failed",
			"job_id", job.ID,
			"attempt", attempt,
			"error", lastErr,
		)
	}

	if lastErr != nil {
		s.markJobFailed(ctx, job, lastErr.Error())
		return
	}

	s.handleSuccess(ctx, job, result, zoneID)
}

func (s *service) handleSuccess(ctx context.Context, job *OCRJob, result *detectPlateResponse, zoneID uuid.UUID) {
	rawBytes, _ := json.Marshal(result)
	rawOutput := datatypes.JSON(rawBytes)

	vehicleTypeID, err := s.resolveVehicleTypeID(ctx, zoneID)
	if err != nil {
		s.log.Error(ctx, "ocr: failed to resolve vehicle type from zone",
			"job_id", job.ID,
			"zone_id", zoneID,
			"error", err,
		)
		s.markJobFailed(ctx, job, fmt.Sprintf("resolve vehicle type failed: %v", err))
		return
	}

	// Python now returns a public S3 URL directly — store it as-is.
	if result.OutputImagePath != "" {
		job.OutputImageURL = &result.OutputImagePath
		job.OutputImagePath = &result.OutputImagePath
	}

	confidence := decimal.NewFromFloat(result.Confidence)
	isAutoVerified := result.Confidence >= s.autoAcceptThreshold

	ocrResult := &OCRResult{
		ID:            uuid.New(),
		OCRJobID:      job.ID,
		PlateDetected: result.DetectedPlate,
		Confidence:    confidence,
		RawOutput:     rawOutput,
		IsVerified:    isAutoVerified,
	}
	if isAutoVerified {
		now := time.Now()
		ocrResult.VerifiedAt = &now
	}

	var vehicleID *uuid.UUID
	if vehicleTypeID != uuid.Nil {
		vehicle, err := s.vehicleSvc.UpsertVehicle(ctx, &vehicleDomain.UpsertVehicleRequest{
			PlateNumber:   result.DetectedPlate,
			VehicleTypeID: vehicleTypeID,
			Source:        types.VehicleSourceOCR,
		})
		if err != nil {
			s.log.Error(ctx, "ocr: failed to upsert vehicle",
				"job_id", job.ID,
				"plate", result.DetectedPlate,
				"error", err,
			)
			s.markJobFailed(ctx, job, fmt.Sprintf("upsert vehicle failed: %v", err))
			return
		}
		vehicleID = &vehicle.ID
		ocrResult.VehicleID = vehicleID
		// ActualPlate mirrors the canonical vehicle record. is_match is intentionally
		// left nil here — it is only meaningful for the entry-vs-exit comparison done
		// in createReviewLog, not for OCR-vs-vehicle-record (which is always identical
		// because we just upserted with the detected plate).
		ocrResult.ActualPlate = vehicle.PlateNumber
	} else {
		s.log.Info(ctx, "ocr: zone has no for_vehicle_type_id, plate detected but vehicle not linked",
			"job_id", job.ID,
			"zone_id", zoneID,
			"plate", result.DetectedPlate,
		)
	}

	if err := s.resultRepo.Create(ctx, ocrResult); err != nil {
		s.log.Error(ctx, "ocr: failed to save result", "job_id", job.ID, "error", err)
		s.markJobFailed(ctx, job, fmt.Sprintf("save result failed: %v", err))
		return
	}

	if vehicleID != nil {
		if err := updateTransactionVehicle(ctx, s.db, job.TransactionID, *vehicleID); err != nil {
			s.log.Error(ctx, "ocr: failed to update transaction vehicle_id",
				"job_id", job.ID,
				"transaction_id", job.TransactionID,
				"vehicle_id", *vehicleID,
				"error", err,
			)
		}
	}

	// For exit jobs: compare exit plate against entry OCR plate and flag mismatch.
	if job.PhotoType == types.OCRPhotoTypeExit {
		s.createReviewLog(ctx, job, ocrResult, result.DetectedPlate)
	}

	completedAt := time.Now()
	job.Status = types.OCRJobCompleted
	job.CompletedAt = &completedAt
	if err := s.jobRepo.UpdateStatus(ctx, job); err != nil {
		s.log.Error(ctx, "ocr: failed to mark job as completed", "job_id", job.ID, "error", err)
	}

	outputURL := ""
	if result.OutputImagePath != "" {
		outputURL = result.OutputImagePath
	}

	s.log.Info(ctx, "ocr: job completed",
		"job_id", job.ID,
		"transaction_id", job.TransactionID,
		"photo_type", job.PhotoType,
		"plate_detected", result.DetectedPlate,
		"confidence", result.Confidence,
		"output_image_url", outputURL,
		"auto_verified", isAutoVerified,
	)
}

// resolveVehicleTypeID fetches the zone and returns its ForVehicleTypeID.
// Returns uuid.Nil (not an error) if the zone has no ForVehicleTypeID configured;
// the caller must handle uuid.Nil by skipping vehicle upsert.
func (s *service) resolveVehicleTypeID(ctx context.Context, zoneID uuid.UUID) (uuid.UUID, error) {
	zone, err := s.zoneRepo.FindByID(ctx, zoneID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("fetch zone: %w", err)
	}
	if zone.ForVehicleTypeID == nil {
		return uuid.Nil, nil
	}
	return *zone.ForVehicleTypeID, nil
}

// createReviewLog compares the exit plate against the entry OCR result and saves
// an OCRReviewLog with auto_match_result pre-filled. Called only for exit jobs.
// Non-fatal: failures are logged but do not affect the job completion status.
func (s *service) createReviewLog(ctx context.Context, job *OCRJob, exitResult *OCRResult, exitPlate string) {
	entryResult, err := s.resultRepo.FindEntryResultByTransactionID(ctx, job.TransactionID)
	if err != nil {
		s.log.Warn(ctx, "ocr: entry result not found for plate-match, skipping review log",
			"job_id", job.ID,
			"transaction_id", job.TransactionID,
			"error", err,
		)
		return
	}

	// Compare exit plate against what was detected at entry, not against the vehicle record.
	// The vehicle record always matches the detected plate (we upserted it), so that
	// comparison would always be true and never catch entry-exit mismatches.
	autoMatch := normalizePlate(exitPlate) == normalizePlate(entryResult.PlateDetected)

	reviewLog := &OCRReviewLog{
		ID:              uuid.New(),
		OCRResultID:     exitResult.ID,
		OCRPlate:        exitPlate,
		OCRConfidence:   exitResult.Confidence,
		AutoMatchResult: &autoMatch,
	}

	if err := s.reviewLogRepo.Create(ctx, reviewLog); err != nil {
		s.log.Error(ctx, "ocr: failed to create review log",
			"job_id", job.ID,
			"transaction_id", job.TransactionID,
			"error", err,
		)
		return
	}

	mismatch := !autoMatch
	if err := s.db.WithContext(ctx).Model(&struct {
		ID uuid.UUID `gorm:"primaryKey"`
	}{ID: job.TransactionID}).Table("transactions").Update("plate_mismatch", mismatch).Error; err != nil {
		s.log.Error(ctx, "ocr: failed to stamp plate_mismatch on transaction",
			"job_id", job.ID,
			"transaction_id", job.TransactionID,
			"error", err,
		)
	}

	s.log.Info(ctx, "ocr: plate match review log created",
		"job_id", job.ID,
		"transaction_id", job.TransactionID,
		"entry_plate", entryResult.PlateDetected,
		"exit_plate", exitPlate,
		"auto_match", autoMatch,
		"plate_mismatch_flagged", mismatch,
	)
}

func normalizePlate(plate string) string {
	return strings.ToUpper(strings.ReplaceAll(plate, " ", ""))
}

func (s *service) markJobFailed(ctx context.Context, job *OCRJob, reason string) {
	completedAt := time.Now()
	job.Status = types.OCRJobFailed
	job.CompletedAt = &completedAt
	job.ErrorMessage = &reason
	if err := s.jobRepo.UpdateStatus(ctx, job); err != nil {
		s.log.Error(ctx, "ocr: failed to mark job as failed", "job_id", job.ID, "error", err)
	}
	s.log.Warn(ctx, "ocr: job failed",
		"job_id", job.ID,
		"transaction_id", job.TransactionID,
		"reason", reason,
	)
}

func (s *service) markJobSkipped(ctx context.Context, job *OCRJob, reason string) {
	completedAt := time.Now()
	job.Status = types.OCRJobSkipped
	job.CompletedAt = &completedAt
	job.ErrorMessage = &reason
	if err := s.jobRepo.UpdateStatus(ctx, job); err != nil {
		s.log.Error(ctx, "ocr: failed to mark job as skipped", "job_id", job.ID, "error", err)
	}
	s.log.Warn(ctx, "ocr: job skipped",
		"job_id", job.ID,
		"transaction_id", job.TransactionID,
		"reason", reason,
	)
}
