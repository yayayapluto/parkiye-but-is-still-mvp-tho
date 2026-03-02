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
	ID              uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TransactionID   uuid.UUID          `gorm:"type:uuid;not null;index"`
	ImageURL        string             `gorm:"type:text"`
	PhotoType       types.OCRPhotoType `gorm:"type:varchar(10);not null;default:'entry'"` // "entry"|"exit"
	Status          types.OCRJobStatus `gorm:"type:varchar(20);not null"`                 // "queued"|"processing"|"completed"|"failed"|"skipped"
	RetryCount      int                `gorm:"not null;default:0"`
	QueuedAt        time.Time          `gorm:"not null"`
	StartedAt       *time.Time
	CompletedAt     *time.Time
	ErrorMessage    *string `gorm:"type:text"`
	OutputImagePath *string `gorm:"type:text"` // container-side path to annotated output image
	OutputImageURL  *string `gorm:"type:text"` // public HTTP URL served via /storage
}

func (OCRJob) TableName() string { return "ocr_jobs" }

// OCRResult holds the raw and resolved output of a single OCR job run.
type OCRResult struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OCRJobID      uuid.UUID       `gorm:"type:uuid;not null;index"`
	PlateDetected string          `gorm:"type:varchar(30)"`           // raw string from OCR engine
	ActualPlate   string          `gorm:"type:varchar(30)"`           // plate on the linked vehicle (may differ if operator corrected)
	IsMatch       *bool           `gorm:"default:null"`               // null until vehicle is resolved; true if PlateDetected == ActualPlate
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
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OCRResultID     uuid.UUID       `gorm:"type:uuid;not null;index"`
	OCRPlate        string          `gorm:"type:varchar(30)"`
	OCRConfidence   decimal.Decimal `gorm:"type:decimal(5,4);not null"`
	ManualPlate     string          `gorm:"type:varchar(30)"`
	ReviewedBy      *uuid.UUID      `gorm:"type:uuid"`    // null if not yet reviewed by operator
	Match           bool            `gorm:"not null"`     // true if manual review confirms match
	AutoMatchResult *bool           `gorm:"default:null"` // auto-filled: true if exit plate matches entry plate, null if entry OCR not yet done
	ReviewNote      string          `gorm:"type:text"`
	ReviewedAt      *time.Time      // null if pending operator review
	CreatedAt       time.Time       `gorm:"autoCreateTime"`

	OCRResult *OCRResult `gorm:"foreignKey:OCRResultID"`
}

func (OCRReviewLog) TableName() string { return "ocr_review_logs" }

// OCRResultWithJob is a flat projection used for transaction OCR summary responses.
type OCRResultWithJob struct {
	PhotoType       types.OCRPhotoType
	PlateDetected   string
	ActualPlate     string
	IsMatch         *bool
	Confidence      decimal.Decimal
	OutputImagePath *string
	OutputImageURL  *string
	IsVerified      bool
}
