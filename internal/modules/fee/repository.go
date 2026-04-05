package fee

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"parkieee/pkg/errors"
)

type feeConfigRepo struct{ db *gorm.DB }
type feeTierRepo struct{ db *gorm.DB }
type holidayRateRepo struct{ db *gorm.DB }

func NewFeeConfigRepository(db *gorm.DB) FeeConfigRepositoryPort {
	return &feeConfigRepo{db}
}

func NewFeeTierRepository(db *gorm.DB) FeeTierRepositoryPort {
	return &feeTierRepo{db}
}

func NewHolidayRateRepository(db *gorm.DB) HolidayRateRepositoryPort {
	return &holidayRateRepo{db}
}

func (r *feeConfigRepo) FindByID(ctx context.Context, id uuid.UUID) (*FeeConfig, error) {
	var cfg FeeConfig
	err := r.db.WithContext(ctx).Preload("Tiers").First(&cfg, "id = ?", id).Error
	return &cfg, errors.FromDB(err, "fee config not found")
}

func (r *feeConfigRepo) FindActiveByZoneAndVehicle(ctx context.Context, zoneID, vehicleTypeID uuid.UUID) (*FeeConfig, error) {
	var cfg FeeConfig
	now := time.Now()
	err := r.db.WithContext(ctx).
		Preload("Tiers", func(db *gorm.DB) *gorm.DB {
			return db.Order("tier_order ASC")
		}).
		Where("zone_id = ? AND vehicle_type_id = ? AND is_active = true AND effective_from <= ?", zoneID, vehicleTypeID, now).
		Where("effective_until IS NULL OR effective_until > ?", now).
		Order("effective_from DESC").
		First(&cfg).Error
	return &cfg, errors.FromDB(err, "no active fee config found for this zone and vehicle type")
}

func (r *feeConfigRepo) FindAll(ctx context.Context, filter ListFeeConfigFilter, page, pageSize int) ([]FeeConfig, int64, error) {
	var configs []FeeConfig
	var total int64

	q := r.db.WithContext(ctx).Model(&FeeConfig{})
	if filter.ZoneID != nil {
		q = q.Where("zone_id = ?", *filter.ZoneID)
	}
	if filter.VehicleTypeID != nil {
		q = q.Where("vehicle_type_id = ?", *filter.VehicleTypeID)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	sortCol := "created_at"
	sortOrder := "desc"
	if filter.SortBy != "" {
		sortCol = filter.SortBy
	}
	if strings.ToLower(filter.SortOrder) == "asc" {
		sortOrder = "asc"
	}

	offset := (page - 1) * pageSize
	err := q.Preload("Tiers", func(db *gorm.DB) *gorm.DB {
		return db.Order("tier_order ASC")
	}).Order(fmt.Sprintf("%s %s", sortCol, sortOrder)).Offset(offset).Limit(pageSize).Find(&configs).Error
	return configs, total, errors.FromDB(err, "")
}

func (r *feeConfigRepo) Create(ctx context.Context, cfg *FeeConfig) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(cfg).Error, "")
}

func (r *feeConfigRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&FeeConfig{}).
		Where("id = ? AND is_active = true", id).
		Update("is_active", false)
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "fee config not found or already inactive")
	}
	return nil
}

func (r *feeTierRepo) FindByFeeConfigID(ctx context.Context, feeConfigID uuid.UUID) ([]FeeTier, error) {
	var tiers []FeeTier
	err := r.db.WithContext(ctx).
		Where("fee_config_id = ?", feeConfigID).
		Order("tier_order ASC").
		Find(&tiers).Error
	return tiers, errors.FromDB(err, "")
}

func (r *feeTierRepo) CreateBatch(ctx context.Context, tiers []FeeTier) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(&tiers).Error, "")
}

func (r *feeTierRepo) DeleteByFeeConfigID(ctx context.Context, feeConfigID uuid.UUID) error {
	return errors.FromDB(
		r.db.WithContext(ctx).Where("fee_config_id = ?", feeConfigID).Delete(&FeeTier{}).Error,
		"",
	)
}

func (r *holidayRateRepo) FindByID(ctx context.Context, id uuid.UUID) (*HolidayRate, error) {
	var rate HolidayRate
	err := r.db.WithContext(ctx).First(&rate, "id = ?", id).Error
	return &rate, errors.FromDB(err, "holiday rate not found")
}

func (r *holidayRateRepo) FindAll(ctx context.Context, filter ListHolidayRateFilter, page, pageSize int) ([]HolidayRate, int64, error) {
	var rates []HolidayRate
	var total int64

	q := r.db.WithContext(ctx).Model(&HolidayRate{})
	if filter.Search != "" {
		q = q.Where("name ILIKE ?", "%"+filter.Search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	sortCol := "date_start"
	sortOrder := "desc"
	if filter.SortBy != "" {
		sortCol = filter.SortBy
	}
	if strings.ToLower(filter.SortOrder) == "asc" {
		sortOrder = "asc"
	}

	offset := (page - 1) * pageSize
	err := q.Order(fmt.Sprintf("%s %s", sortCol, sortOrder)).Offset(offset).Limit(pageSize).Find(&rates).Error
	return rates, total, errors.FromDB(err, "")
}

func (r *holidayRateRepo) FindActiveForDate(ctx context.Context, date time.Time, zoneID *uuid.UUID, vehicleTypeID *uuid.UUID) ([]HolidayRate, error) {
	var rates []HolidayRate
	q := r.db.WithContext(ctx).
		Where("date_start <= ? AND date_end >= ?", date, date).
		Where("(applies_to_zone_id IS NULL OR applies_to_zone_id = ?)", zoneID).
		Where("(applies_to_vehicle_type_id IS NULL OR applies_to_vehicle_type_id = ?)", vehicleTypeID)

	err := q.Find(&rates).Error
	return rates, errors.FromDB(err, "")
}

func (r *holidayRateRepo) Create(ctx context.Context, rate *HolidayRate) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(rate).Error, "")
}

func (r *holidayRateRepo) Update(ctx context.Context, rate *HolidayRate) error {
	return errors.FromDB(
		r.db.WithContext(ctx).Model(rate).Select("*").Updates(rate).Error,
		"",
	)
}

func (r *holidayRateRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&HolidayRate{}, "id = ?", id)
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "holiday rate not found")
	}
	return nil
}

// applyHolidayRates applies all active holiday rate multipliers/overrides to a base fee.
// Multiple multipliers are averaged per domain rules (agents.md: "average all active multipliers").
// Override type takes precedence: if any override exists, average override values are used.
func applyHolidayRates(baseFee int, rates []HolidayRate) int {
	if len(rates) == 0 {
		return baseFee
	}

	var overrideFees []int
	var multipliers []decimal.Decimal

	for _, r := range rates {
		switch r.RateType {
		case "override":
			if r.OverrideFee != nil {
				overrideFees = append(overrideFees, *r.OverrideFee)
			}
		case "multiplier":
			if r.Multiplier != nil {
				multipliers = append(multipliers, *r.Multiplier)
			}
		}
	}

	if len(overrideFees) > 0 {
		sum := 0
		for _, f := range overrideFees {
			sum += f
		}
		return sum / len(overrideFees)
	}

	if len(multipliers) > 0 {
		sum := decimal.Zero
		for _, m := range multipliers {
			sum = sum.Add(m)
		}
		avg := sum.Div(decimal.NewFromInt(int64(len(multipliers))))
		return int(avg.Mul(decimal.NewFromInt(int64(baseFee))).IntPart())
	}

	return baseFee
}
