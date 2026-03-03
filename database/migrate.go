package database

import (
	"fmt"

	"gorm.io/gorm"

	auditDomain "parkieee/internal/modules/audit"
	authDomain "parkieee/internal/modules/auth"
	feeDomain "parkieee/internal/modules/fee"
	gateDomain "parkieee/internal/modules/gate"
	ocrDomain "parkieee/internal/modules/ocr"
	overrideDomain "parkieee/internal/modules/override"
	paymentDomain "parkieee/internal/modules/payment"
	rfidDomain "parkieee/internal/modules/rfid"
	txDomain "parkieee/internal/modules/transaction"
	vehicleDomain "parkieee/internal/modules/vehicle"
	zoneDomain "parkieee/internal/modules/zone"
)

// Migrate runs GORM AutoMigrate for all tables in FK-dependency order.
//
// Order rationale:
//  1. auth      — roles, permissions, users (users self-ref created_by after roles exist)
//  2. zone      — zones, gates, gate_devices, zone_capacity_logs
//  3. vehicle   — vehicle_types, vehicles
//  4. rfid      — rfid_cards (references vehicles)
//  5. fee       — fee_configs, fee_tiers, holiday_rates, ocr_configs, override_configs
//  6. transaction — transactions, transaction_logs, unclosed_transaction_flags
//  7. payment   — payments, midtrans_callbacks, refunds
//  8. override  — operator_overrides
//  9. ocr       — ocr_jobs, ocr_results, ocr_review_logs
//  10. audit     — audit_logs, audit_log_exports
func Migrate(db *gorm.DB) error {
	models := []interface{}{
		&authDomain.Role{},
		&authDomain.Permission{},
		&authDomain.User{},           // depends on roles
		&authDomain.RolePermission{}, // depends on roles + permissions + users
		&authDomain.UserSession{},    // depends on users
		&authDomain.UserLoginLog{},   // depends on users
		&authDomain.UserLoginStats{}, // depends on users

		&zoneDomain.Zone{},       // depends on users (created_by)
		&zoneDomain.Gate{},       // depends on zones + users
		&zoneDomain.GateDevice{}, // depends on gates

		&vehicleDomain.VehicleType{},
		&vehicleDomain.Vehicle{}, // depends on vehicle_types

		&rfidDomain.RFIDCard{}, // depends on vehicles + users

		&feeDomain.FeeConfig{},      // depends on zones + vehicle_types + users
		&feeDomain.FeeTier{},        // depends on fee_configs
		&feeDomain.HolidayRate{},    // depends on zones + vehicle_types + users
		&feeDomain.OCRConfig{},      // depends on users
		&feeDomain.OverrideConfig{}, // depends on users

		&txDomain.Transaction{},             // depends on gates, rfid_cards, vehicles, fee_configs, holiday_rates, zones
		&txDomain.TransactionLog{},          // depends on transactions + users
		&txDomain.UnclosedTransactionFlag{}, // depends on transactions + users

		&paymentDomain.Payment{},          // depends on transactions + users
		&paymentDomain.MidtransCallback{}, // depends on payments
		&paymentDomain.Refund{},           // depends on payments + transactions + users

		&overrideDomain.OperatorOverride{}, // depends on transactions + users

		&ocrDomain.OCRJob{},       // depends on transactions
		&ocrDomain.OCRResult{},    // depends on ocr_jobs + vehicles + users
		&ocrDomain.OCRReviewLog{}, // depends on ocr_results + users

		&auditDomain.AuditLog{},       // depends on users
		&auditDomain.AuditLogExport{}, // depends on users

		&zoneDomain.ZoneCapacityLog{},

		&gateDomain.GatePairingCode{}, // depends on zones (via gate_id)
	}

	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("automigrate failed: %w", err)
	}

	return applyManualConstraints(db)
}

// applyManualConstraints adds constraints & indexes that GORM AutoMigrate
// cannot express via struct tags alone (composite unique, partial indexes,
// check constraints, etc.).
func applyManualConstraints(db *gorm.DB) error {
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_fee_configs_zone_vtype_active
			ON fee_configs (zone_id, vehicle_type_id)
			WHERE is_active = TRUE AND effective_until IS NULL`,

		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_ocr_configs_active
			ON ocr_configs (is_active)
			WHERE is_active = TRUE`,

		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_override_configs_active
			ON override_configs (is_active)
			WHERE is_active = TRUE`,

		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_role_permissions_role_perm
			ON role_permissions (role_id, permission_id)`,

		`ALTER TABLE holiday_rates
			DROP CONSTRAINT IF EXISTS chk_holiday_rates_date_range,
			ADD CONSTRAINT chk_holiday_rates_date_range
			CHECK (date_end >= date_start)`,

		`ALTER TABLE holiday_rates
			DROP CONSTRAINT IF EXISTS chk_holiday_rates_rate_fields,
			ADD CONSTRAINT chk_holiday_rates_rate_fields CHECK (
				(rate_type = 'multiplier' AND multiplier IS NOT NULL) OR
				(rate_type = 'override'   AND override_fee IS NOT NULL)
			)`,

		`ALTER TABLE payments
			DROP CONSTRAINT IF EXISTS chk_payments_cash_fields,
			ADD CONSTRAINT chk_payments_cash_fields CHECK (
				(method = 'cash' AND cash_tendered IS NOT NULL) OR
				(method != 'cash')
			)`,

		`ALTER TABLE payments
			DROP CONSTRAINT IF EXISTS chk_payments_qris_fields,
			ADD CONSTRAINT chk_payments_qris_fields CHECK (
				(method = 'qris' AND midtrans_order_id IS NOT NULL) OR
				(method != 'qris')
			)`,

		`ALTER TABLE transactions
			DROP CONSTRAINT IF EXISTS chk_transactions_entry_rfid,
			ADD CONSTRAINT chk_transactions_entry_rfid CHECK (
				(entry_method = 'rfid' AND rfid_card_id IS NOT NULL) OR
				(entry_method != 'rfid')
			)`,

		`ALTER TABLE transactions
			DROP CONSTRAINT IF EXISTS chk_transactions_entry_qr,
			ADD CONSTRAINT chk_transactions_entry_qr CHECK (
				(entry_method = 'qr' AND entry_qr_code IS NOT NULL) OR
				(entry_method != 'qr')
			)`,

		`ALTER TABLE fee_tiers
			DROP CONSTRAINT IF EXISTS chk_fee_tiers_order_positive,
			ADD CONSTRAINT chk_fee_tiers_order_positive CHECK (tier_order > 0)`,

		`ALTER TABLE fee_tiers
			DROP CONSTRAINT IF EXISTS chk_fee_tiers_duration_positive,
			ADD CONSTRAINT chk_fee_tiers_duration_positive CHECK (duration_minutes > 0)`,

		`ALTER TABLE ocr_configs
			DROP CONSTRAINT IF EXISTS chk_ocr_configs_threshold,
			ADD CONSTRAINT chk_ocr_configs_threshold CHECK (
				auto_accept_threshold >= 0 AND auto_accept_threshold <= 1
			)`,

		`ALTER TABLE ocr_results
			DROP CONSTRAINT IF EXISTS chk_ocr_results_confidence,
			ADD CONSTRAINT chk_ocr_results_confidence CHECK (
				confidence >= 0 AND confidence <= 1
			)`,

		`CREATE INDEX IF NOT EXISTS idx_transactions_status
			ON transactions (status)`,

		`CREATE INDEX IF NOT EXISTS idx_transactions_zone_status
			ON transactions (zone_id, status)`,

		`CREATE INDEX IF NOT EXISTS idx_transactions_open_entry_at
			ON transactions (entry_at)
			WHERE status = 'open'`,

		`CREATE INDEX IF NOT EXISTS idx_ocr_jobs_status
			ON ocr_jobs (status)
			WHERE status IN ('queued', 'processing')`,

		`CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_created
			ON audit_logs (actor_id, created_at DESC)`,

		`CREATE INDEX IF NOT EXISTS idx_payments_transaction_status
			ON payments (transaction_id, status)`,

		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_gates_token
			ON gates (gate_token)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("constraint/index migration failed:\nSQL: %s\nErr: %w", stmt, err)
		}
	}

	return nil
}
