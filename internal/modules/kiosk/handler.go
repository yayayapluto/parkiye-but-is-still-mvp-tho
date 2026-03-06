package kiosk

import (
	"parkieee/internal/modules/fee"
	"parkieee/internal/modules/vehicle"
	"parkieee/internal/modules/zone"
	"parkieee/pkg/middleware"
	"parkieee/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type handler struct {
	feeSvc     fee.ServicePort
	vehicleSvc vehicle.ServicePort
	zoneSvc    zone.ServicePort
}

func newHandler(feeSvc fee.ServicePort, vehicleSvc vehicle.ServicePort, zoneSvc zone.ServicePort) *handler {
	return &handler{feeSvc: feeSvc, vehicleSvc: vehicleSvc, zoneSvc: zoneSvc}
}

// getTariff returns vehicle types + active fee configs for the gate's zone.
// Protected by GateAuth — only authenticated gate screens can call this.
//
// GET /api/v1/kiosk/tariff
func (h *handler) getTariff(c *fiber.Ctx) error {
	zoneID := middleware.GetZoneID(c)

	// Fetch zone to get additional_fee surcharge
	z, err := h.zoneSvc.GetZone(c.Context(), zoneID)
	if err != nil {
		return err
	}

	vehicleTypes, err := h.vehicleSvc.ListVehicleTypes(c.Context())
	if err != nil {
		return err
	}

	configs, _, err := h.feeSvc.ListFeeConfigs(c.Context(), &zoneID, nil, 1, 100)
	if err != nil {
		return err
	}

	configByVehicle := make(map[string]*fee.FeeConfig, len(configs))
	for i := range configs {
		cfg := &configs[i]
		if cfg.IsActive {
			configByVehicle[cfg.VehicleTypeID.String()] = cfg
		}
	}

	result := make([]VehicleTypeTariff, 0, len(vehicleTypes))
	for _, vt := range vehicleTypes {
		// Kalau zone punya ForVehicleTypeID, hanya tampilkan tariff untuk vehicle type itu
		if z.ForVehicleTypeID != nil && vt.ID != *z.ForVehicleTypeID {
			continue
		}

		cfg, hasCfg := configByVehicle[vt.ID.String()]
		if !hasCfg {
			continue
		}

		tiers := make([]FeeTierItem, len(cfg.Tiers))
		for i, t := range cfg.Tiers {
			tiers[i] = FeeTierItem{
				TierOrder:       t.TierOrder,
				DurationMinutes: t.DurationMinutes,
				FeeAmount:       t.FeeAmount,
				IsLastTier:      t.IsLastTier,
			}
		}

		result = append(result, VehicleTypeTariff{
			ID:          vt.ID.String(),
			Name:        vt.Name,
			MinimumFee:  vt.MinimumFee,
			Description: vt.Description,
			FeeConfig: &FeeConfigItem{
				BaseFee:            cfg.BaseFee,
				GracePeriodMinutes: cfg.GracePeriodMinutes,
				AdditionalFee:      z.AdditionalFee,
				Tiers:              tiers,
			},
		})
	}

	return response.Success(c, "ok", result)
}
