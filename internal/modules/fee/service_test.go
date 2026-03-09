package fee_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"parkieee/internal/modules/fee"
	feemocks "parkieee/internal/modules/fee/mocks"
	"parkieee/pkg/logger"
	"parkieee/pkg/types"
)

// noopLogger satisfies logger.Logger without printing anything during tests.
type noopLogger struct{}

func (noopLogger) Debug(_ context.Context, _ string, _ ...any) {}
func (noopLogger) Info(_ context.Context, _ string, _ ...any)  {}
func (noopLogger) Warn(_ context.Context, _ string, _ ...any)  {}
func (noopLogger) Error(_ context.Context, _ string, _ ...any) {}
func (noopLogger) Fatal(_ context.Context, _ string, _ ...any) {}
func (n noopLogger) With(_ ...any) logger.Logger               { return n }
func (n noopLogger) WithGroup(_ string) logger.Logger          { return n }

var _ logger.Logger = noopLogger{}

var (
	zoneID        = uuid.New()
	vehicleTypeID = uuid.New()
	baseEntry     = time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
)

func newService(
	t *testing.T,
	cfgRepo fee.FeeConfigRepositoryPort,
	holidayRepo fee.HolidayRateRepositoryPort,
) fee.ServicePort {
	// FeeTierRepositoryPort is not called by CalculateFee
	tierRepo := feemocks.NewMockFeeTierRepositoryPort(t)
	return fee.NewService(cfgRepo, tierRepo, holidayRepo, noopLogger{})
}

func TestCalculateTierFee(t *testing.T) {
	tests := []struct {
		name            string
		tiers           []fee.FeeTier
		billableMinutes int
		expected        int
	}{
		{
			name:            "no tiers returns 0",
			tiers:           nil,
			billableMinutes: 60,
			expected:        0,
		},
		{
			name: "single non-last tier, minutes fit exactly",
			tiers: []fee.FeeTier{
				{DurationMinutes: 60, FeeAmount: 3000, IsLastTier: false},
			},
			billableMinutes: 60,
			expected:        3000,
		},
		{
			name: "single non-last tier, minutes exceed — no last tier so remainder is dropped",
			tiers: []fee.FeeTier{
				{DurationMinutes: 60, FeeAmount: 3000, IsLastTier: false},
			},
			billableMinutes: 90,
			expected:        3000,
		},
		{
			name: "last tier repeats for remaining blocks, exact",
			tiers: []fee.FeeTier{
				{DurationMinutes: 60, FeeAmount: 2000, IsLastTier: true},
			},
			billableMinutes: 120,
			expected:        4000, // 2 blocks × 2000
		},
		{
			name: "last tier rounds up partial block",
			tiers: []fee.FeeTier{
				{DurationMinutes: 60, FeeAmount: 2000, IsLastTier: true},
			},
			billableMinutes: 90,
			expected:        4000, // ceil(90/60) = 2 blocks × 2000
		},
		{
			name: "two normal tiers then last tier",
			tiers: []fee.FeeTier{
				{DurationMinutes: 60, FeeAmount: 2000, IsLastTier: false},
				{DurationMinutes: 30, FeeAmount: 1000, IsLastTier: false},
				{DurationMinutes: 30, FeeAmount: 500, IsLastTier: true},
			},
			// 60 consumed by tier1 (2000), 30 consumed by tier2 (1000), 30 remaining → 1 block of tier3 (500)
			billableMinutes: 120,
			expected:        3500,
		},
		{
			name: "minutes stop before reaching last tier",
			tiers: []fee.FeeTier{
				{DurationMinutes: 60, FeeAmount: 2000, IsLastTier: false},
				{DurationMinutes: 60, FeeAmount: 2000, IsLastTier: true},
			},
			billableMinutes: 30,
			// 30 < 60 so tier1 is consumed (flat fee applies), remaining=0, last tier not reached
			expected: 2000,
		},
		{
			name:            "zero billable minutes",
			tiers:           []fee.FeeTier{{DurationMinutes: 60, FeeAmount: 2000, IsLastTier: true}},
			billableMinutes: 0,
			expected:        0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// calculateTierFee is unexported — test via CalculateFee with mocked repo
			// to keep the test in the same package boundary.
			// We mock FindActiveByZoneAndVehicle to return a config with the desired tiers,
			// and FindActiveForDate to return no holiday rates.
			cfgRepo := feemocks.NewMockFeeConfigRepositoryPort(t)
			holidayRepo := feemocks.NewMockHolidayRateRepositoryPort(t)

			cfg := &fee.FeeConfig{
				ID:                 uuid.New(),
				ZoneID:             zoneID,
				VehicleTypeID:      vehicleTypeID,
				BaseFee:            0,
				GracePeriodMinutes: 0,
				Tiers:              tc.tiers,
			}

			exitTime := baseEntry.Add(time.Duration(tc.billableMinutes) * time.Minute)

			cfgRepo.EXPECT().
				FindActiveByZoneAndVehicle(context.Background(), zoneID, vehicleTypeID).
				Return(cfg, nil)

			if tc.billableMinutes > 0 {
				holidayRepo.EXPECT().
					FindActiveForDate(context.Background(), baseEntry, &zoneID, &vehicleTypeID).
					Return(nil, nil)
			}

			svc := newService(t, cfgRepo, holidayRepo)
			result, err := svc.CalculateFee(context.Background(), zoneID, vehicleTypeID, baseEntry, exitTime)

			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCalculateFee_GracePeriod(t *testing.T) {
	cfgRepo := feemocks.NewMockFeeConfigRepositoryPort(t)
	holidayRepo := feemocks.NewMockHolidayRateRepositoryPort(t)

	cfg := &fee.FeeConfig{
		BaseFee:            2000,
		GracePeriodMinutes: 15,
		Tiers:              []fee.FeeTier{{DurationMinutes: 60, FeeAmount: 3000, IsLastTier: true}},
	}

	cfgRepo.EXPECT().
		FindActiveByZoneAndVehicle(context.Background(), zoneID, vehicleTypeID).
		Return(cfg, nil)

	// 10 minutes < 15 minute grace period → only base fee
	exitTime := baseEntry.Add(10 * time.Minute)
	svc := newService(t, cfgRepo, holidayRepo)
	result, err := svc.CalculateFee(context.Background(), zoneID, vehicleTypeID, baseEntry, exitTime)

	require.NoError(t, err)
	assert.Equal(t, 2000, result)
}

func TestCalculateFee_ZeroDuration(t *testing.T) {
	cfgRepo := feemocks.NewMockFeeConfigRepositoryPort(t)
	holidayRepo := feemocks.NewMockHolidayRateRepositoryPort(t)

	cfgRepo.EXPECT().
		FindActiveByZoneAndVehicle(context.Background(), zoneID, vehicleTypeID).
		Return(&fee.FeeConfig{BaseFee: 5000}, nil)

	svc := newService(t, cfgRepo, holidayRepo)
	result, err := svc.CalculateFee(context.Background(), zoneID, vehicleTypeID, baseEntry, baseEntry)

	require.NoError(t, err)
	assert.Equal(t, 0, result)
}

func TestCalculateFee_NoActiveConfig(t *testing.T) {
	cfgRepo := feemocks.NewMockFeeConfigRepositoryPort(t)
	holidayRepo := feemocks.NewMockHolidayRateRepositoryPort(t)

	cfgRepo.EXPECT().
		FindActiveByZoneAndVehicle(context.Background(), zoneID, vehicleTypeID).
		Return(nil, fmt.Errorf("no active fee config"))

	svc := newService(t, cfgRepo, holidayRepo)
	_, err := svc.CalculateFee(context.Background(), zoneID, vehicleTypeID, baseEntry, baseEntry.Add(time.Hour))

	require.Error(t, err)
}

func TestCalculateFee_HolidayRateMultiplier(t *testing.T) {
	cfgRepo := feemocks.NewMockFeeConfigRepositoryPort(t)
	holidayRepo := feemocks.NewMockHolidayRateRepositoryPort(t)

	cfg := &fee.FeeConfig{
		BaseFee:            2000,
		GracePeriodMinutes: 0,
		Tiers:              []fee.FeeTier{{DurationMinutes: 60, FeeAmount: 3000, IsLastTier: true}},
	}

	multiplier := decimal.NewFromFloat(1.5)
	holidayRates := []fee.HolidayRate{
		{RateType: types.HolidayRateMultiplier, Multiplier: &multiplier},
	}

	exitTime := baseEntry.Add(60 * time.Minute)

	cfgRepo.EXPECT().
		FindActiveByZoneAndVehicle(context.Background(), zoneID, vehicleTypeID).
		Return(cfg, nil)
	holidayRepo.EXPECT().
		FindActiveForDate(context.Background(), baseEntry, &zoneID, &vehicleTypeID).
		Return(holidayRates, nil)

	svc := newService(t, cfgRepo, holidayRepo)
	result, err := svc.CalculateFee(context.Background(), zoneID, vehicleTypeID, baseEntry, exitTime)

	require.NoError(t, err)
	// rawFee = 2000 + 3000 = 5000, multiplier 1.5 → 7500
	assert.Equal(t, 7500, result)
}

func TestCalculateFee_HolidayRateOverride(t *testing.T) {
	cfgRepo := feemocks.NewMockFeeConfigRepositoryPort(t)
	holidayRepo := feemocks.NewMockHolidayRateRepositoryPort(t)

	cfg := &fee.FeeConfig{
		BaseFee:            2000,
		GracePeriodMinutes: 0,
		Tiers:              []fee.FeeTier{{DurationMinutes: 60, FeeAmount: 3000, IsLastTier: true}},
	}

	overrideFee := 10000
	holidayRates := []fee.HolidayRate{
		{RateType: types.HolidayRateOverride, OverrideFee: &overrideFee},
	}

	exitTime := baseEntry.Add(60 * time.Minute)

	cfgRepo.EXPECT().
		FindActiveByZoneAndVehicle(context.Background(), zoneID, vehicleTypeID).
		Return(cfg, nil)
	holidayRepo.EXPECT().
		FindActiveForDate(context.Background(), baseEntry, &zoneID, &vehicleTypeID).
		Return(holidayRates, nil)

	svc := newService(t, cfgRepo, holidayRepo)
	result, err := svc.CalculateFee(context.Background(), zoneID, vehicleTypeID, baseEntry, exitTime)

	require.NoError(t, err)
	assert.Equal(t, 10000, result)
}

func TestCalculateFee_MultipleOverrides_Averaged(t *testing.T) {
	cfgRepo := feemocks.NewMockFeeConfigRepositoryPort(t)
	holidayRepo := feemocks.NewMockHolidayRateRepositoryPort(t)

	cfg := &fee.FeeConfig{BaseFee: 2000, GracePeriodMinutes: 0}
	exitTime := baseEntry.Add(30 * time.Minute)

	fee1 := 8000
	fee2 := 12000
	holidayRates := []fee.HolidayRate{
		{RateType: types.HolidayRateOverride, OverrideFee: &fee1},
		{RateType: types.HolidayRateOverride, OverrideFee: &fee2},
	}

	cfgRepo.EXPECT().
		FindActiveByZoneAndVehicle(context.Background(), zoneID, vehicleTypeID).
		Return(cfg, nil)
	holidayRepo.EXPECT().
		FindActiveForDate(context.Background(), baseEntry, &zoneID, &vehicleTypeID).
		Return(holidayRates, nil)

	svc := newService(t, cfgRepo, holidayRepo)
	result, err := svc.CalculateFee(context.Background(), zoneID, vehicleTypeID, baseEntry, exitTime)

	require.NoError(t, err)
	assert.Equal(t, 10000, result) // (8000 + 12000) / 2
}
