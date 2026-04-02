package types

type GateType string

const (
	GateTypeEntry GateType = "entry"
	GateTypeExit  GateType = "exit"
)

type GateMode string

const (
	GateModeManless     GateMode = "manless"
	GateModeWithCashier GateMode = "with_cashier"
)

type DeviceType string

const (
	DeviceTypeRFIDReader DeviceType = "rfid_reader"
	DeviceTypePrinter    DeviceType = "printer"
	DeviceTypeCamera     DeviceType = "camera"
	DeviceTypeBarrier    DeviceType = "barrier"
	DeviceTypeQRScanner  DeviceType = "qr_scanner"
)

type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
	DeviceStatusError   DeviceStatus = "error"
)

type ZoneEventType string

const (
	ZoneEventEntry ZoneEventType = "entry"
	ZoneEventExit  ZoneEventType = "exit"
)
