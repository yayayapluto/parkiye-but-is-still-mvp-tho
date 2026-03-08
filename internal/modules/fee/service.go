package fee

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/types"
)

type service struct {
	feeConfigRepo   FeeConfigRepositoryPort
	feeTierRepo     FeeTierRepositoryPort
	holidayRateRepo HolidayRateRepositoryPort
	log             logger.Logger
}

func NewService(
	feeConfigRepo FeeConfigRepositoryPort,
	feeTierRepo FeeTierRepositoryPort,
	holidayRateRepo HolidayRateRepositoryPort,
	log logger.Logger,
) ServicePort {
	return &service{
		feeConfigRepo:   feeConfigRepo,
		feeTierRepo:     feeTierRepo,
		holidayRateRepo: holidayRateRepo,
		log:             log,
	}
}

func (s *service) ListFeeConfigs(ctx context.Context, zoneID *uuid.UUID, vehicleTypeID *uuid.UUID, page, pageSize int) ([]FeeConfig, int64, error) {
	return s.feeConfigRepo.FindAll(ctx, zoneID, vehicleTypeID, page, pageSize)
}

func (s *service) GetFeeConfig(ctx context.Context, id uuid.UUID) (*FeeConfig, error) {
	return s.feeConfigRepo.FindByID(ctx, id)
}

func (s *service) GetActiveFeeConfig(ctx context.Context, zoneID, vehicleTypeID uuid.UUID) (*FeeConfig, error) {
	return s.feeConfigRepo.FindActiveByZoneAndVehicle(ctx, zoneID, vehicleTypeID)
}

func (s *service) CreateFeeConfig(ctx context.Context, req CreateFeeConfigRequest, createdBy uuid.UUID) (*FeeConfig, error) {
	if err := s.validateTiers(req.Tiers); err != nil {
		s.log.Warn(ctx, "create fee config failed: invalid tiers", "zone_id", req.ZoneID, "error", err)
		return nil, err
	}

	cfg := &FeeConfig{
		ID:                 uuid.New(),
		ZoneID:             req.ZoneID,
		VehicleTypeID:      req.VehicleTypeID,
		BaseFee:            req.BaseFee,
		GracePeriodMinutes: req.GracePeriodMinutes,
		IsActive:           true,
		EffectiveFrom:      req.EffectiveFrom,
		EffectiveUntil:     req.EffectiveUntil,
		CreatedBy:          &createdBy,
	}

	if err := s.feeConfigRepo.Create(ctx, cfg); err != nil {
		s.log.Error(ctx, "failed to create fee config", "zone_id", cfg.ZoneID, "vehicle_type_id", cfg.VehicleTypeID, "error", err)
		return nil, err
	}

	if len(req.Tiers) > 0 {
		tiers := make([]FeeTier, len(req.Tiers))
		for i, t := range req.Tiers {
			tiers[i] = FeeTier{
				ID:              uuid.New(),
				FeeConfigID:     cfg.ID,
				TierOrder:       t.TierOrder,
				DurationMinutes: t.DurationMinutes,
				FeeAmount:       t.FeeAmount,
				IsLastTier:      t.IsLastTier,
			}
		}
		if err := s.feeTierRepo.CreateBatch(ctx, tiers); err != nil {
			s.log.Error(ctx, "failed to create fee tiers", "fee_config_id", cfg.ID, "error", err)
			return nil, err
		}
	}

	s.log.Info(ctx, "fee config created", "id", cfg.ID, "zone_id", cfg.ZoneID, "vehicle_type_id", cfg.VehicleTypeID)
	return s.feeConfigRepo.FindByID(ctx, cfg.ID)
}

func (s *service) DeactivateFeeConfig(ctx context.Context, id uuid.UUID) error {
	if err := s.feeConfigRepo.Deactivate(ctx, id); err != nil {
		s.log.Error(ctx, "failed to deactivate fee config", "id", id, "error", err)
		return err
	}
	s.log.Info(ctx, "fee config deactivated", "id", id)
	return nil
}

func (s *service) ListHolidayRates(ctx context.Context, page, pageSize int) ([]HolidayRate, int64, error) {
	return s.holidayRateRepo.FindAll(ctx, page, pageSize)
}

func (s *service) GetHolidayRate(ctx context.Context, id uuid.UUID) (*HolidayRate, error) {
	return s.holidayRateRepo.FindByID(ctx, id)
}

func (s *service) CreateHolidayRate(ctx context.Context, req CreateHolidayRateRequest, createdBy uuid.UUID) (*HolidayRate, error) {
	if err := validateHolidayRateFields(req.RateType, decimalPtrFromFloat(req.Multiplier), req.OverrideFee); err != nil {
		s.log.Warn(ctx, "create holiday rate failed: invalid fields", "name", req.Name, "error", err)
		return nil, err
	}

	rate := &HolidayRate{
		ID:                     uuid.New(),
		Name:                   req.Name,
		DateStart:              req.DateStart.Time,
		DateEnd:                req.DateEnd.Time,
		RateType:               req.RateType,
		Multiplier:             decimalPtrFromFloat(req.Multiplier),
		OverrideFee:            req.OverrideFee,
		AppliesToZoneID:        req.AppliesToZoneID,
		AppliesToVehicleTypeID: req.AppliesToVehicleTypeID,
		CreatedBy:              &createdBy,
	}

	if err := s.holidayRateRepo.Create(ctx, rate); err != nil {
		s.log.Error(ctx, "failed to create holiday rate", "name", rate.Name, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "holiday rate created", "id", rate.ID, "name", rate.Name)
	return s.holidayRateRepo.FindByID(ctx, rate.ID)
}

func (s *service) UpdateHolidayRate(ctx context.Context, id uuid.UUID, req UpdateHolidayRateRequest) (*HolidayRate, error) {
	rate, err := s.holidayRateRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		rate.Name = *req.Name
	}
	if req.DateStart != nil {
		rate.DateStart = req.DateStart.Time
	}
	if req.DateEnd != nil {
		rate.DateEnd = req.DateEnd.Time
	}
	if req.RateType != nil {
		rate.RateType = *req.RateType
	}
	if req.Multiplier != nil {
		rate.Multiplier = decimalPtrFromFloat(req.Multiplier)
	}
	if req.OverrideFee != nil {
		rate.OverrideFee = req.OverrideFee
	}
	if req.AppliesToZoneID != nil {
		rate.AppliesToZoneID = req.AppliesToZoneID
	}
	if req.AppliesToVehicleTypeID != nil {
		rate.AppliesToVehicleTypeID = req.AppliesToVehicleTypeID
	}

	if err := validateHolidayRateFields(rate.RateType, rate.Multiplier, rate.OverrideFee); err != nil {
		s.log.Warn(ctx, "update holiday rate failed: invalid fields", "id", id, "error", err)
		return nil, err
	}

	if err := s.holidayRateRepo.Update(ctx, rate); err != nil {
		s.log.Error(ctx, "failed to update holiday rate", "id", id, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "holiday rate updated", "id", id)
	return s.holidayRateRepo.FindByID(ctx, id)
}

func (s *service) DeleteHolidayRate(ctx context.Context, id uuid.UUID) error {
	if err := s.holidayRateRepo.Delete(ctx, id); err != nil {
		s.log.Error(ctx, "failed to delete holiday rate", "id", id, "error", err)
		return err
	}
	s.log.Info(ctx, "holiday rate deleted", "id", id)
	return nil
}

// CalculateFee computes the total parking fee for a stay.
// Logic: skip grace period → apply tiers in order → last_tier repeats for remaining time → apply holiday rates.
func (s *service) CalculateFee(ctx context.Context, zoneID, vehicleTypeID uuid.UUID, entryTime, exitTime time.Time) (int, error) {
	cfg, err := s.feeConfigRepo.FindActiveByZoneAndVehicle(ctx, zoneID, vehicleTypeID)
	if err != nil {
		s.log.Warn(ctx, "calculate fee failed: no active fee config", "zone_id", zoneID, "vehicle_type_id", vehicleTypeID, "error", err)
		return 0, err
	}

	totalMinutes := int(exitTime.Sub(entryTime).Minutes())
	if totalMinutes <= 0 {
		return 0, nil
	}

	billableMinutes := totalMinutes - cfg.GracePeriodMinutes
	if billableMinutes <= 0 {
		return cfg.BaseFee, nil
	}

	baseFee := cfg.BaseFee
	tierFee := calculateTierFee(cfg.Tiers, billableMinutes)
	rawFee := baseFee + tierFee

	holidayRates, err := s.holidayRateRepo.FindActiveForDate(ctx, entryTime, &zoneID, &vehicleTypeID)
	if err != nil {
		return 0, err
	}

	finalFee := applyHolidayRates(rawFee, holidayRates)
	return finalFee, nil
}

// calculateTierFee walks tiers in order consuming billable minutes.
// The last tier (is_last_tier=true) repeats for all remaining time blocks.
func calculateTierFee(tiers []FeeTier, billableMinutes int) int {
	if len(tiers) == 0 {
		return 0
	}

	total := 0
	remaining := billableMinutes

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		if tier.IsLastTier {
			blocks := (remaining + tier.DurationMinutes - 1) / tier.DurationMinutes
			total += blocks * tier.FeeAmount
			remaining = 0
			break
		}

		consumed := min(remaining, tier.DurationMinutes)
		total += tier.FeeAmount
		remaining -= consumed
	}

	return total
}

func (s *service) validateTiers(tiers []CreateFeeTierRequest) error {
	lastTierCount := 0
	for _, t := range tiers {
		if t.IsLastTier {
			lastTierCount++
		}
		if t.DurationMinutes <= 0 {
			return errors.New(errors.ErrValidation, "tier duration_minutes must be greater than 0")
		}
		if t.FeeAmount < 0 {
			return errors.New(errors.ErrValidation, "tier fee_amount must be 0 or greater")
		}
	}
	if lastTierCount > 1 {
		return errors.New(errors.ErrValidation, "only one tier can be marked as is_last_tier")
	}
	return nil
}

func validateHolidayRateFields(rateType types.HolidayRateType, multiplier *decimal.Decimal, overrideFee *int) error {
	switch rateType {
	case types.HolidayRateMultiplier:
		if multiplier == nil {
			return errors.New(errors.ErrValidation, "multiplier is required when rate_type is 'multiplier'")
		}
		if multiplier.LessThanOrEqual(decimal.Zero) {
			return errors.New(errors.ErrValidation, "multiplier must be greater than 0")
		}
	case types.HolidayRateOverride:
		if overrideFee == nil {
			return errors.New(errors.ErrValidation, "override_fee is required when rate_type is 'override'")
		}
		if *overrideFee < 0 {
			return errors.New(errors.ErrValidation, "override_fee must be 0 or greater")
		}
	default:
		return errors.New(errors.ErrValidation, "rate_type must be 'multiplier' or 'override'")
	}
	return nil
}
