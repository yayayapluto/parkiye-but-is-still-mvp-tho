package rfid

import (
	"context"

	"github.com/google/uuid"
)

type RFIDCardRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*RFIDCard, error)
	FindByUID(ctx context.Context, cardUID string) (*RFIDCard, error)
	FindAll(ctx context.Context, filter ListRFIDFilter, page, pageSize int) ([]RFIDCard, int64, error)
	Create(ctx context.Context, card *RFIDCard) error
	Update(ctx context.Context, card *RFIDCard) error
	Deactivate(ctx context.Context, id uuid.UUID, deactivatedBy uuid.UUID) error
}

type ServicePort interface {
	ListCards(ctx context.Context, filter ListRFIDFilter, page, pageSize int) ([]RFIDCard, int64, error)
	GetCard(ctx context.Context, id uuid.UUID) (*RFIDCard, error)
	GetCardByUID(ctx context.Context, cardUID string) (*RFIDCard, error)

	// RegisterOrGet is called on first tap — creates card if UID is unknown,
	// returns existing card if already registered. Idempotent.
	RegisterOrGet(ctx context.Context, cardUID string) (*RFIDCard, error)

	// LinkVehicle back-fills vehicle_id after OCR resolves the plate.
	LinkVehicle(ctx context.Context, id uuid.UUID, vehicleID uuid.UUID) (*RFIDCard, error)

	// Deactivate marks a card inactive (lost card report, etc).
	Deactivate(ctx context.Context, id uuid.UUID, operatorID uuid.UUID) error

	// Enrichment
	EnrichCard(ctx context.Context, card *RFIDCard, includes map[string]bool) *RFIDCardEnrichment
	EnrichCardList(ctx context.Context, cards []RFIDCard, includes map[string]bool) map[uuid.UUID]RFIDCardEnrichment
}

