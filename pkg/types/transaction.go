package types

type TransactionStatus string

const (
	TransactionStatusOpen            TransactionStatus = "open"
	TransactionStatusAwaitingPayment TransactionStatus = "awaiting_payment"
	TransactionStatusPaid            TransactionStatus = "paid"
	TransactionStatusExited          TransactionStatus = "exited"
	TransactionStatusOverridden      TransactionStatus = "overridden"
	TransactionStatusCancelled       TransactionStatus = "cancelled"
)

type EntryMethod string

const (
	EntryMethodRFID EntryMethod = "rfid"
	EntryMethodQR   EntryMethod = "qr"
)

type ExitMethod string

const (
	ExitMethodRFID     ExitMethod = "rfid"
	ExitMethodQR       ExitMethod = "qr"
	ExitMethodOverride ExitMethod = "override"
)

type TransactionEvent string

const (
	EventEntryCreated     TransactionEvent = "entry_created"
	EventOCRCompleted     TransactionEvent = "ocr_completed"
	EventPaymentInitiated TransactionEvent = "payment_initiated"
	EventPaymentReceived  TransactionEvent = "payment_received"
	EventGateOpened       TransactionEvent = "gate_opened"
	EventExitRecorded     TransactionEvent = "exit_recorded"
	EventOverrideApplied  TransactionEvent = "override_applied"
	EventFlaggedUnclosed  TransactionEvent = "flagged_unclosed"
	EventReceiptPrinted   TransactionEvent = "receipt_printed"
)

type TriggeredBy string

const (
	TriggeredBySystem   TriggeredBy = "system"
	TriggeredByOperator TriggeredBy = "operator"
	TriggeredByCashier  TriggeredBy = "cashier"
	TriggeredByWebhook  TriggeredBy = "webhook"
)

type FlagType string

const (
	FlagTypeOvernight          FlagType = "overnight"
	FlagTypeMultiDay           FlagType = "multi_day"
	FlagTypeSuspiciousDuration FlagType = "suspicious_duration"
	FlagTypePlateMismatch      FlagType = "plate_mismatch"
)
