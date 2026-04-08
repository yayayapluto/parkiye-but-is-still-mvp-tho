package dashboard

// DashboardStats merupakan agregasi ringkas untuk tampilan dashboard (Legacy/General).
type DashboardStats struct {
	ActiveTransactions int64 `json:"active_transactions"`
	AvailableSlots     int64 `json:"available_slots"`
	RevenueToday       int64 `json:"revenue_today"`
	TotalVehiclesToday int64 `json:"total_vehicles_today"`
}

// UserRoleSummary merupakan jumlah user aktif per role.
type UserRoleSummary struct {
	RoleName  string `json:"role_name"`
	UserCount int64  `json:"user_count"`
}

// OperatorDashboardData data khusus untuk petugas lapangan.
type OperatorDashboardData struct {
	TotalCapacity     int64             `json:"total_capacity"`
	OccupiedSlots     int64             `json:"occupied_slots"`
	AvailabilityPct   float64           `json:"availability_pct"`
	ActiveEntries     int64             `json:"active_entries"`
	ZoneStatus        []ZoneStatus      `json:"zone_status"`
}

type ZoneStatus struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	OccupiedCount int64   `json:"occupied_count"`
	Capacity      int64   `json:"capacity"`
	OccupancyPct  float64 `json:"occupancy_pct"`
	Status        string  `json:"status"` // "normal", "warning", "full"
}

// OwnerDashboardData data khusus untuk pemilik/manajemen.
type OwnerDashboardData struct {
	RevenueToday         int64            `json:"revenue_today"`
	TransactionCount     int64            `json:"transaction_count"`
	PaymentDistribution  []PaymentSummary `json:"payment_distribution"`
	PeakHourToday        string           `json:"peak_hour_today"`
	OccupancyEfficiency  float64          `json:"occupancy_efficiency"`
}

type PaymentSummary struct {
	Method string `json:"method"`
	Count  int64  `json:"count"`
	Amount int64  `json:"amount"`
}

// AdminDashboardData data khusus untuk administrator sistem.
type AdminDashboardData struct {
	TotalUsers     int64             `json:"total_users"`
	ActiveSessions int64             `json:"active_sessions"`
	RolesSummary   []UserRoleSummary `json:"roles_summary"`
	SystemHealth   string            `json:"system_health"` // "all_ok", "degraded", "failure"
}

// EngineerDashboardData data khusus untuk teknisi.
type EngineerDashboardData struct {
	TotalGates    int64 `json:"total_gates"`
	ActiveGates   int64 `json:"active_gates"`
	TotalKiosks   int64 `json:"total_kiosks"`
	ActiveKiosks  int64 `json:"active_kiosks"`
	RecentErrors  int64 `json:"recent_errors_24h"`
}

// CashierDashboardData data khusus untuk kasir.
type CashierDashboardData struct {
	AwaitingPaymentCount int64 `json:"awaiting_payment_count"`
	CompletedTodayCount  int64 `json:"completed_today_count"`
	AverageServiceTime   int64 `json:"average_service_time_seconds"`
}
