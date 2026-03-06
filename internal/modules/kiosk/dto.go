package kiosk

// VehicleTypeTariff adalah response untuk satu vehicle type beserta fee config-nya.
type VehicleTypeTariff struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	MinimumFee  int            `json:"minimum_fee"`
	Description string         `json:"description"`
	FeeConfig   *FeeConfigItem `json:"fee_config"` // null jika belum ada fee config untuk zone ini
}

// FeeConfigItem adalah ringkasan fee config yang relevan untuk display kiosk.
type FeeConfigItem struct {
	BaseFee            int           `json:"base_fee"`
	GracePeriodMinutes int           `json:"grace_period_minutes"`
	AdditionalFee      int           `json:"additional_fee"`
	Tiers              []FeeTierItem `json:"tiers"`
}

// FeeTierItem adalah satu tier dalam fee config.
type FeeTierItem struct {
	TierOrder       int  `json:"tier_order"`
	DurationMinutes int  `json:"duration_minutes"`
	FeeAmount       int  `json:"fee_amount"`
	IsLastTier      bool `json:"is_last_tier"`
}
