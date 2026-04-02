package dashboard

// DashboardStats merupakan agregasi ringkas untuk tampilan dashboard.
type DashboardStats struct {
	ActiveTransactions  int64   `json:"active_transactions"`
	AvailableSlots      int64   `json:"available_slots"`
	RevenueToday        int64   `json:"revenue_today"`
	TotalVehiclesToday  int64   `json:"total_vehicles_today"`
}

// UserRoleSummary merupakan jumlah user aktif per role.
type UserRoleSummary struct {
	RoleName  string `json:"role_name"`
	UserCount int64  `json:"user_count"`
}
