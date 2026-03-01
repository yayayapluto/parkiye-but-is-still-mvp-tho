package fee

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"parkieee/pkg/types"
)

// DecimalFromFloat safely converts *float64 from JSON into *decimal.Decimal.
func decimalPtrFromFloat(f *float64) *decimal.Decimal {
	if f == nil {
		return nil
	}
	d := decimal.NewFromFloat(*f)
	return &d
}

type CreateFeeTierRequest struct {
	TierOrder       int  `json:"tier_order"       validate:"required,min=1"`
	DurationMinutes int  `json:"duration_minutes" validate:"required,min=1"`
	FeeAmount       int  `json:"fee_amount"       validate:"min=0"`
	IsLastTier      bool `json:"is_last_tier"`
}

type CreateFeeConfigRequest struct {
	ZoneID             uuid.UUID              `json:"zone_id"              validate:"required"`
	VehicleTypeID      uuid.UUID              `json:"vehicle_type_id"      validate:"required"`
	BaseFee            int                    `json:"base_fee"             validate:"min=0"`
	GracePeriodMinutes int                    `json:"grace_period_minutes" validate:"min=0"`
	EffectiveFrom      time.Time              `json:"effective_from"       validate:"required"`
	EffectiveUntil     *time.Time             `json:"effective_until"`
	Tiers              []CreateFeeTierRequest `json:"tiers"                validate:"omitempty,dive"`
}

type CreateHolidayRateRequest struct {
	Name                   string                `json:"name"                     validate:"required,min=2,max=100"`
	DateStart              types.DateOnly        `json:"date_start"               validate:"required"`
	DateEnd                types.DateOnly        `json:"date_end"                 validate:"required"`
	RateType               types.HolidayRateType `json:"rate_type"                validate:"required,oneof=multiplier override"`
	Multiplier             *float64              `json:"multiplier"`
	OverrideFee            *int                  `json:"override_fee"`
	AppliesToZoneID        *uuid.UUID            `json:"applies_to_zone_id"`
	AppliesToVehicleTypeID *uuid.UUID            `json:"applies_to_vehicle_type_id"`
}

type UpdateHolidayRateRequest struct {
	Name                   *string                `json:"name"                     validate:"omitempty,min=2,max=100"`
	DateStart              *types.DateOnly        `json:"date_start"`
	DateEnd                *types.DateOnly        `json:"date_end"`
	RateType               *types.HolidayRateType `json:"rate_type"                validate:"omitempty,oneof=multiplier override"`
	Multiplier             *float64               `json:"multiplier"`
	OverrideFee            *int                   `json:"override_fee"`
	AppliesToZoneID        *uuid.UUID             `json:"applies_to_zone_id"`
	AppliesToVehicleTypeID *uuid.UUID             `json:"applies_to_vehicle_type_id"`
}

type FeeTierResponse struct {
	ID              uuid.UUID `json:"id"`
	TierOrder       int       `json:"tier_order"`
	DurationMinutes int       `json:"duration_minutes"`
	FeeAmount       int       `json:"fee_amount"`
	IsLastTier      bool      `json:"is_last_tier"`
}

type FeeConfigResponse struct {
	ID                 uuid.UUID         `json:"id"`
	ZoneID             uuid.UUID         `json:"zone_id"`
	VehicleTypeID      uuid.UUID         `json:"vehicle_type_id"`
	BaseFee            int               `json:"base_fee"`
	GracePeriodMinutes int               `json:"grace_period_minutes"`
	IsActive           bool              `json:"is_active"`
	EffectiveFrom      time.Time         `json:"effective_from"`
	EffectiveUntil     *time.Time        `json:"effective_until"`
	CreatedBy          *uuid.UUID        `json:"created_by"`
	CreatedAt          time.Time         `json:"created_at"`
	Tiers              []FeeTierResponse `json:"tiers"`
}

type HolidayRateResponse struct {
	ID                     uuid.UUID             `json:"id"`
	Name                   string                `json:"name"`
	DateStart              time.Time             `json:"date_start"`
	DateEnd                time.Time             `json:"date_end"`
	RateType               types.HolidayRateType `json:"rate_type"`
	Multiplier             *decimal.Decimal      `json:"multiplier"`
	OverrideFee            *int                  `json:"override_fee"`
	AppliesToZoneID        *uuid.UUID            `json:"applies_to_zone_id"`
	AppliesToVehicleTypeID *uuid.UUID            `json:"applies_to_vehicle_type_id"`
	CreatedBy              *uuid.UUID            `json:"created_by"`
	CreatedAt              time.Time             `json:"created_at"`
}

func toFeeConfigResponse(cfg *FeeConfig) FeeConfigResponse {
	tiers := make([]FeeTierResponse, len(cfg.Tiers))
	for i, t := range cfg.Tiers {
		tiers[i] = FeeTierResponse{
			ID:              t.ID,
			TierOrder:       t.TierOrder,
			DurationMinutes: t.DurationMinutes,
			FeeAmount:       t.FeeAmount,
			IsLastTier:      t.IsLastTier,
		}
	}
	return FeeConfigResponse{
		ID:                 cfg.ID,
		ZoneID:             cfg.ZoneID,
		VehicleTypeID:      cfg.VehicleTypeID,
		BaseFee:            cfg.BaseFee,
		GracePeriodMinutes: cfg.GracePeriodMinutes,
		IsActive:           cfg.IsActive,
		EffectiveFrom:      cfg.EffectiveFrom,
		EffectiveUntil:     cfg.EffectiveUntil,
		CreatedBy:          cfg.CreatedBy,
		CreatedAt:          cfg.CreatedAt,
		Tiers:              tiers,
	}
}

func toHolidayRateResponse(r *HolidayRate) HolidayRateResponse {
	return HolidayRateResponse{
		ID:                     r.ID,
		Name:                   r.Name,
		DateStart:              r.DateStart,
		DateEnd:                r.DateEnd,
		RateType:               r.RateType,
		Multiplier:             r.Multiplier,
		OverrideFee:            r.OverrideFee,
		AppliesToZoneID:        r.AppliesToZoneID,
		AppliesToVehicleTypeID: r.AppliesToVehicleTypeID,
		CreatedBy:              r.CreatedBy,
		CreatedAt:              r.CreatedAt,
	}
}
