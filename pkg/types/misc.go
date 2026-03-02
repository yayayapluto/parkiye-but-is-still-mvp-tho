package types

type HolidayRateType string

const (
	HolidayRateMultiplier HolidayRateType = "multiplier"
	HolidayRateOverride   HolidayRateType = "override"
)

type OCRJobStatus string

const (
	OCRJobQueued     OCRJobStatus = "queued"
	OCRJobProcessing OCRJobStatus = "processing"
	OCRJobCompleted  OCRJobStatus = "completed"
	OCRJobFailed     OCRJobStatus = "failed"
	OCRJobSkipped    OCRJobStatus = "skipped"
)

type OCRPhotoType string

const (
	OCRPhotoTypeEntry OCRPhotoType = "entry"
	OCRPhotoTypeExit  OCRPhotoType = "exit"
)

type OverrideType string

const (
	OverrideLostCardExit  OverrideType = "lost_card_exit"
	OverrideNoQRExit      OverrideType = "no_qr_exit"
	OverrideFeeWaive      OverrideType = "fee_waive"
	OverrideFeeAdjust     OverrideType = "fee_adjust"
	OverrideForceOpenGate OverrideType = "force_open_gate"
	OverrideManualEntry   OverrideType = "manual_entry"
)

type AuditEventType string

const (
	AuditFeeConfigChanged      AuditEventType = "fee_config_changed"
	AuditRoleAssigned          AuditEventType = "role_assigned"
	AuditRoleRevoked           AuditEventType = "role_revoked"
	AuditPermissionChanged     AuditEventType = "permission_changed"
	AuditOverridePerformed     AuditEventType = "override_performed"
	AuditCardDeactivated       AuditEventType = "card_deactivated"
	AuditUserCreated           AuditEventType = "user_created"
	AuditUserDeactivated       AuditEventType = "user_deactivated"
	AuditHolidayRateChanged    AuditEventType = "holiday_rate_changed"
	AuditOCRConfigChanged      AuditEventType = "ocr_config_changed"
	AuditOverrideConfigChanged AuditEventType = "override_config_changed"
	AuditRefundApproved        AuditEventType = "refund_approved"
)

type AuditTargetType string

const (
	AuditTargetFeeConfig      AuditTargetType = "fee_config"
	AuditTargetUser           AuditTargetType = "user"
	AuditTargetRole           AuditTargetType = "role"
	AuditTargetRFIDCard       AuditTargetType = "rfid_card"
	AuditTargetHolidayRate    AuditTargetType = "holiday_rate"
	AuditTargetOverrideConfig AuditTargetType = "override_config"
	AuditTargetOCRConfig      AuditTargetType = "ocr_config"
	AuditTargetTransaction    AuditTargetType = "transaction"
	AuditTargetRefund         AuditTargetType = "refund"
)

type ExportFormat string

const (
	ExportFormatXLSX ExportFormat = "xlsx"
	ExportFormatCSV  ExportFormat = "csv"
)

type ExportStatus string

const (
	ExportStatusPending   ExportStatus = "pending"
	ExportStatusCompleted ExportStatus = "completed"
	ExportStatusFailed    ExportStatus = "failed"
)

type VehicleSource string

const (
	VehicleSourceOCR            VehicleSource = "ocr"
	VehicleSourceManualOverride VehicleSource = "manual_override"
)
