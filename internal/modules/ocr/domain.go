package ocr

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"parkieee/pkg/types"
)

// OCRJob tracks the lifecycle of an OCR processing task for a transaction image.
type OCRJob struct {
	ID            uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TransactionID uuid.UUID          `gorm:"type:uuid;not null;index"`
	ImageURL      string             `gorm:"type:text"`
	Status        types.OCRJobStatus `gorm:"type:varchar(20);not null"` // "queued"|"processing"|"completed"|"failed"|"skipped"
	RetryCount    int                `gorm:"not null;default:0"`
	QueuedAt      time.Time          `gorm:"not null"`
	StartedAt     *time.Time
	CompletedAt   *time.Time
	ErrorMessage  *string `gorm:"type:text"`
}

func (OCRJob) TableName() string { return "ocr_jobs" }

// OCRResult holds the raw and resolved output of a single OCR job run.
type OCRResult struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OCRJobID      uuid.UUID       `gorm:"type:uuid;not null;index"`
	PlateDetected string          `gorm:"type:varchar(30)"`           // raw string from OCR engine
	Confidence    decimal.Decimal `gorm:"type:decimal(5,4);not null"` // 0.0–1.0, compared against ocr_configs.auto_accept_threshold
	RawOutput     datatypes.JSON  `gorm:"type:jsonb"`                 // full response from EasyOCR/PaddleOCR
	VehicleID     *uuid.UUID      `gorm:"type:uuid;index"`            // resolved vehicle after find-or-create
	IsVerified    bool            `gorm:"not null;default:false"`     // true if confidence >= threshold (auto) or operator confirmed (manual)
	VerifiedBy    *uuid.UUID      `gorm:"type:uuid"`                  // null if auto-verified by system
	VerifiedAt    *time.Time
	CreatedAt     time.Time `gorm:"autoCreateTime"`

	OCRJob *OCRJob `gorm:"foreignKey:OCRJobID"`
}

func (OCRResult) TableName() string { return "ocr_results" }

// OCRReviewLog compares OCR result against a manual operator correction.
type OCRReviewLog struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OCRResultID   uuid.UUID       `gorm:"type:uuid;not null;index"`
	OCRPlate      string          `gorm:"type:varchar(30)"`
	OCRConfidence decimal.Decimal `gorm:"type:decimal(5,4);not null"`
	ManualPlate   string          `gorm:"type:varchar(30)"`
	ReviewedBy    uuid.UUID       `gorm:"type:uuid;not null"`
	Match         bool            `gorm:"not null"` // true if OCR plate = manual plate
	ReviewNote    string          `gorm:"type:text"`
	ReviewedAt    time.Time       `gorm:"not null;default:now()"`

	OCRResult *OCRResult `gorm:"foreignKey:OCRResultID"`
}

func (OCRReviewLog) TableName() string { return "ocr_review_logs" }
