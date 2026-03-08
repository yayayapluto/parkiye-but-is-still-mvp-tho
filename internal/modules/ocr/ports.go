package ocr

import (
	"context"

	"github.com/google/uuid"

	"parkieee/pkg/types"
)

// TransactionStamperPort is a minimal write interface injected into OCR service
// to avoid a direct cross-module DB write. Implemented by transaction.Service.
type TransactionStamperPort interface {
	StampPlateMismatch(ctx context.Context, txID uuid.UUID, mismatch bool) error
}

// ServicePort is the only interface transaction module needs to call.
// It is intentionally minimal: fire-and-forget job dispatch.
type ServicePort interface {
	// DispatchOCRJob enqueues and processes an OCR job for the given transaction.
	// imagePath is the absolute file path on the shared Docker volume (e.g. /mnt/storage/photos/xxx.jpg).
	// zoneID is used to resolve the default vehicle type from zone.for_vehicle_type_id.
	// photoType distinguishes entry vs exit photos so the service can run plate-match logic on exit.
	// This method is designed to be called from a goroutine; it handles all retries and DB writes internally.
	DispatchOCRJob(ctx context.Context, transactionID uuid.UUID, imagePath string, zoneID uuid.UUID, photoType types.OCRPhotoType)

	// SetTransactionStamper injects the transaction stamper post-construction to avoid circular dependency.
	SetTransactionStamper(stamper TransactionStamperPort)
}

type OCRJobRepositoryPort interface {
	Create(ctx context.Context, job *OCRJob) error
	UpdateStatus(ctx context.Context, job *OCRJob) error
}

type OCRResultRepositoryPort interface {
	Create(ctx context.Context, result *OCRResult) error
	// FindEntryResultByTransactionID returns the OCR result from the entry photo of a transaction.
	// Used to compare against exit plate for auto_match_result.
	FindEntryResultByTransactionID(ctx context.Context, transactionID uuid.UUID) (*OCRResult, error)
	// FindSummaryByTransactionID returns both entry and exit OCR results for a transaction.
	// Used to populate the OCR summary in transaction responses.
	FindSummaryByTransactionID(ctx context.Context, transactionID uuid.UUID) ([]OCRResultWithJob, error)
}

type OCRReviewLogRepositoryPort interface {
	Create(ctx context.Context, log *OCRReviewLog) error
}
