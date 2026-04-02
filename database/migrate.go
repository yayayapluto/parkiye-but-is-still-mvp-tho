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
	notifDomain "parkieee/internal/modules/notification"
	rfidDomain "parkieee/internal/modules/rfid"
	txDomain "parkieee/internal/modules/transaction"
	vehicleDomain "parkieee/internal/modules/vehicle"
	zoneDomain "parkieee/internal/modules/zone"
)

// Migrate runs GORM AutoMigrate for all tables in FK-dependency order.
//
// Order rationale:
//  1. auth      — roles, permissions, users
//  2. zone      — zones, gates, gate_devices, gate_cashier_assignments, zone_capacity_logs
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
		&authDomain.User{},
		&authDomain.RolePermission{},
		&authDomain.UserSession{},
		&authDomain.RefreshToken{},
		&authDomain.UserLoginLog{},
		&authDomain.UserLoginStats{},

		&zoneDomain.Zone{},
		&zoneDomain.Gate{},                  // mode column added
		&zoneDomain.GateCashierAssignment{}, // new — 1 kasir per exit gate
		&zoneDomain.GateDevice{},

		&vehicleDomain.VehicleType{},
		&vehicleDomain.Vehicle{},

		&rfidDomain.RFIDCard{},

		&feeDomain.FeeConfig{},
		&feeDomain.FeeTier{},
		&feeDomain.HolidayRate{},
		&feeDomain.OCRConfig{},
		&feeDomain.OverrideConfig{},

		&txDomain.Transaction{},
		&txDomain.TransactionLog{},
		&txDomain.UnclosedTransactionFlag{},

		&paymentDomain.Payment{},
		&paymentDomain.MidtransCallback{},
		&paymentDomain.Refund{},

		&overrideDomain.OperatorOverride{},

		&ocrDomain.OCRJob{},
		&ocrDomain.OCRResult{},
		&ocrDomain.OCRReviewLog{},

		&auditDomain.AuditLog{},
		&auditDomain.AuditLogExport{},

		&zoneDomain.ZoneCapacityLog{},

		&gateDomain.GatePairingCode{},
		&notifDomain.Notification{},
	}

	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			return fmt.Errorf("automigrate %T failed: %w", m, err)
		}
	}

	return applyManualConstraints(db)
}

// applyManualConstraints adds constraints & indexes that GORM AutoMigrate
// cannot express via struct tags alone.
func applyManualConstraints(db *gorm.DB) error {
	// Backfill username dari email untuk rows lama yang belum punya username.
	// Format: localpart_6charhex agar tetap unik meski localpart sama.
	if err := db.Exec(`
		UPDATE users
		SET username = LOWER(split_part(email, '@', 1)) || '_' || SUBSTR(id::text, 1, 6)
		WHERE username = ''
	`).Error; err != nil {
		return fmt.Errorf("backfill username: %w", err)
	}

	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_users_username ON users (username)`,
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

		// gate mode hanya boleh nilai yang valid
		`ALTER TABLE gates
			DROP CONSTRAINT IF EXISTS chk_gates_mode,
			ADD CONSTRAINT chk_gates_mode CHECK (
				mode IN ('manless', 'with_cashier')
			)`,

		// exit-only enforcement is handled in the service layer (AssignCashier)
		// because PostgreSQL CHECK constraints do not support subqueries

		// with_cashier gate wajib punya assignment, manless tidak boleh punya
		// (ini enforced di application layer, bukan DB — terlalu kompleks untuk CHECK constraint)

		`CREATE INDEX IF NOT EXISTS idx_transactions_status
			ON transactions (status)`,

		`CREATE INDEX IF NOT EXISTS idx_transactions_zone_status
			ON transactions (zone_id, status)`,

		`CREATE INDEX IF NOT EXISTS idx_transactions_open_entry_at
			ON transactions (entry_at)
			WHERE status = 'open'`,

		`CREATE INDEX IF NOT EXISTS idx_transactions_paid_at
			ON transactions (updated_at)
			WHERE status = 'paid'`,

		`CREATE INDEX IF NOT EXISTS idx_ocr_jobs_status
			ON ocr_jobs (status)
			WHERE status IN ('queued', 'processing')`,

		`CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_created
			ON audit_logs (actor_id, created_at DESC)`,

		`CREATE INDEX IF NOT EXISTS idx_payments_transaction_status
			ON payments (transaction_id, status)`,

		`CREATE UNIQUE INDEX IF NOT EXISTS uidx_gates_token
			ON gates (gate_token)`,

		`CREATE INDEX IF NOT EXISTS idx_gate_cashier_assignments_user
			ON gate_cashier_assignments (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications (user_id, is_read) WHERE is_read = false`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("constraint/index migration failed:\nSQL: %s\nErr: %w", stmt, err)
		}
	}

	return nil
}
