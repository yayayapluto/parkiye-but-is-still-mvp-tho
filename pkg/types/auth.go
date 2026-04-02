package types

type RoleName string

const (
	RoleOperator RoleName = "operator"
	RoleAdmin    RoleName = "admin"
	RoleOwner    RoleName = "owner"
	RoleEngineer RoleName = "engineer"
)

type PermissionNode string

const (
	// Gate operations
	PermGateOverride PermissionNode = "gate.override" // force open gate, override exit
	PermGateManage   PermissionNode = "gate.manage"   // create, edit, deactivate gate; regenerate token
	PermGatePair     PermissionNode = "gate.pair"     // confirm pairing + assign kasir ke exit gate
	PermGateView     PermissionNode = "gate.view"     // lihat daftar gate dan statusnya

	// Zone operations
	PermZoneManage PermissionNode = "zone.manage" // create, edit, deactivate zone
	PermZoneView   PermissionNode = "zone.view"   // lihat daftar zone dan kapasitas

	// Fee operations
	PermFeeEdit PermissionNode = "fee.edit" // create/edit fee config, tiers, holiday rates
	PermFeeView PermissionNode = "fee.view" // lihat fee config dan tarif

	// User & role operations
	PermUserManage    PermissionNode = "user.manage"    // create, deactivate, ganti role user
	PermUserView      PermissionNode = "user.view"      // lihat daftar user
	PermCashierAssign PermissionNode = "cashier.assign" // assign/unassign kasir ke exit gate

	// Cashier operations
	PermCashierAbility PermissionNode = "cashier.ability" // akses halaman kasir, terima SSE dari kiosk
	PermPaymentCash    PermissionNode = "payment.cash"    // proses pembayaran tunai
	PermPaymentQRIS    PermissionNode = "payment.qris"    // proses dan handle QRIS
	PermPaymentRefund  PermissionNode = "payment.refund"  // request dan approve/reject refund

	// Transaction operations
	PermTransactionView   PermissionNode = "transaction.view"   // lihat detail transaksi
	PermTransactionCancel PermissionNode = "transaction.cancel" // cancel transaksi yang masih open

	// Override operations
	PermOverridePerform PermissionNode = "override.perform" // lakukan override (lost card, no qr, dll)
	PermOverrideConfig  PermissionNode = "override.config"  // atur limit override harian/mingguan

	// RFID operations
	PermRFIDManage PermissionNode = "rfid.manage" // deactivate kartu, link/unlink ke kendaraan
	PermRFIDView   PermissionNode = "rfid.view"   // lihat daftar kartu RFID

	// Report & analytics
	PermReportView PermissionNode = "report.view" // lihat laporan pendapatan dan statistik

	// System & config
	PermConfigEdit PermissionNode = "config.edit" // edit OCR threshold, system config
	PermAuditRead  PermissionNode = "audit.read"  // baca audit log, export audit data

	// Internal (dipakai service-to-service, bukan user)
	PermInternalAccess PermissionNode = "internal.access" // Python service → Go API (mark-exited, dll)
)

type LoginAttemptType string

const (
	LoginAttemptLogin  LoginAttemptType = "login"
	LoginAttemptLogout LoginAttemptType = "logout"
)

type FailureReason string

const (
	FailureWrongPassword   FailureReason = "wrong_password"
	FailureUserNotFound    FailureReason = "user_not_found"
	FailureAccountLocked   FailureReason = "account_locked"
	FailureAccountInactive FailureReason = "account_inactive"
)

type LockedReason string

const (
	LockedReasonTooManyFailedAttempts LockedReason = "too_many_failed_attempts"
	LockedReasonManualLockByAdmin     LockedReason = "manual_lock_by_admin"
)
