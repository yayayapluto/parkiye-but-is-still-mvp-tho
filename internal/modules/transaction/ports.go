package transaction

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	ocrDomain "parkieee/internal/modules/ocr"
	"parkieee/pkg/types"
)

type ListFilter struct {
	Status      *types.TransactionStatus
	ZoneID      *uuid.UUID
	EntryGateID *uuid.UUID
	EntryMethod *types.EntryMethod
	DateFrom    *time.Time
	DateTo      *time.Time
}

type TransactionRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Transaction, error)
	FindByCode(ctx context.Context, code string) (*Transaction, error)
	FindAll(ctx context.Context, filter ListFilter, page, pageSize int) ([]Transaction, int64, error)

	// FindOpenByRFIDCard returns the active (status=open) transaction for a card.
	// Used to prevent double-entry by the same card.
	FindOpenByRFIDCard(ctx context.Context, cardID uuid.UUID) (*Transaction, error)

	// CountByDatePrefix counts rows whose transaction_code starts with prefix (e.g. "PKR-20260301-").
	// Must run on the passed tx so the count is consistent within the same DB transaction.
	CountByDatePrefix(ctx context.Context, db *gorm.DB, prefix string) (int64, error)

	Create(ctx context.Context, db *gorm.DB, t *Transaction) error
	Update(ctx context.Context, db *gorm.DB, t *Transaction) error
}

type TransactionLogRepositoryPort interface {
	FindByTransactionID(ctx context.Context, txID uuid.UUID) ([]TransactionLog, error)
	Append(ctx context.Context, db *gorm.DB, log *TransactionLog) error
}

type ServicePort interface {
	// RecordEntry creates an open parking session. Atomic: tx + log + capacity in one DB txn.
	RecordEntry(ctx context.Context, req RecordEntryRequest, operatorID uuid.UUID) (*Transaction, error)

	// RecordExit calculates the fee and moves status to awaiting_payment.
	// Atomic: update tx + log + capacity in one DB txn.
	RecordExit(ctx context.Context, id uuid.UUID, req RecordExitRequest, operatorID uuid.UUID) (*Transaction, error)

	// Cancel marks an open transaction as cancelled.
	Cancel(ctx context.Context, id uuid.UUID, reason string, operatorID uuid.UUID) (*Transaction, error)

	GetTransaction(ctx context.Context, id uuid.UUID) (*Transaction, error)
	GetByCode(ctx context.Context, code string) (*Transaction, error)
	ListTransactions(ctx context.Context, filter ListFilter, page, pageSize int) ([]Transaction, int64, error)
	GetLogs(ctx context.Context, txID uuid.UUID) ([]TransactionLog, error)
	LoadOCRSummary(ctx context.Context, txID uuid.UUID) []ocrDomain.OCRResultWithJob
}
