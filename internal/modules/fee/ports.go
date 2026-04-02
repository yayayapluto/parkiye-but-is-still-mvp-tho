package fee

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type FeeConfigRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*FeeConfig, error)
	FindActiveByZoneAndVehicle(ctx context.Context, zoneID, vehicleTypeID uuid.UUID) (*FeeConfig, error)
	FindAll(ctx context.Context, zoneID *uuid.UUID, vehicleTypeID *uuid.UUID, page, pageSize int) ([]FeeConfig, int64, error)
	Create(ctx context.Context, config *FeeConfig) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

type FeeTierRepositoryPort interface {
	FindByFeeConfigID(ctx context.Context, feeConfigID uuid.UUID) ([]FeeTier, error)
	CreateBatch(ctx context.Context, tiers []FeeTier) error
	DeleteByFeeConfigID(ctx context.Context, feeConfigID uuid.UUID) error
}

type HolidayRateRepositoryPort interface {
	FindByID(ctx context.Context, id uuid.UUID) (*HolidayRate, error)
	FindAll(ctx context.Context, page, pageSize int) ([]HolidayRate, int64, error)
	FindActiveForDate(ctx context.Context, date time.Time, zoneID *uuid.UUID, vehicleTypeID *uuid.UUID) ([]HolidayRate, error)
	Create(ctx context.Context, rate *HolidayRate) error
	Update(ctx context.Context, rate *HolidayRate) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ServicePort interface {
	// FeeConfig
	ListFeeConfigs(ctx context.Context, zoneID *uuid.UUID, vehicleTypeID *uuid.UUID, page, pageSize int) ([]FeeConfig, int64, error)
	GetFeeConfig(ctx context.Context, id uuid.UUID) (*FeeConfig, error)
	GetActiveFeeConfig(ctx context.Context, zoneID, vehicleTypeID uuid.UUID) (*FeeConfig, error)
	CreateFeeConfig(ctx context.Context, req CreateFeeConfigRequest, createdBy uuid.UUID) (*FeeConfig, error)
	DeactivateFeeConfig(ctx context.Context, id uuid.UUID) error

	// HolidayRate
	ListHolidayRates(ctx context.Context, page, pageSize int) ([]HolidayRate, int64, error)
	GetHolidayRate(ctx context.Context, id uuid.UUID) (*HolidayRate, error)
	CreateHolidayRate(ctx context.Context, req CreateHolidayRateRequest, createdBy uuid.UUID) (*HolidayRate, error)
	UpdateHolidayRate(ctx context.Context, id uuid.UUID, req UpdateHolidayRateRequest) (*HolidayRate, error)
	DeleteHolidayRate(ctx context.Context, id uuid.UUID) error

	// CalculateFee is called by the transaction module during exit processing.
	// It returns the total fee in IDR for the given parking duration.
	CalculateFee(ctx context.Context, zoneID, vehicleTypeID uuid.UUID, entryTime, exitTime time.Time) (int, error)

	// Enrichment
	EnrichConfig(ctx context.Context, cfg *FeeConfig, includes map[string]bool) *FeeEnrichment
	EnrichConfigList(ctx context.Context, configs []FeeConfig, includes map[string]bool) map[uuid.UUID]FeeEnrichment
}

