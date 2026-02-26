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
	PermGateOverride PermissionNode = "gate.override"
	PermFeeEdit      PermissionNode = "fee.edit"
	PermReportView   PermissionNode = "report.view"
	PermUserManage   PermissionNode = "user.manage"
	PermZoneManage   PermissionNode = "zone.manage"
	PermRFIDManage   PermissionNode = "rfid.manage"
	PermAuditRead    PermissionNode = "audit.read"
	PermConfigEdit   PermissionNode = "config.edit"
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
