package dashboard

import (
	"context"
	"time"

	"gorm.io/gorm"

	"parkieee/pkg/errors"
)

type service struct {
	db *gorm.DB
}

// NewService membuat dashboard service baru.
// Menerima *gorm.DB langsung karena dashboard query bersifat agregat lintas modul
// dan tidak perlu bergantung pada repo individual tiap modul.
func NewService(db *gorm.DB) ServicePort {
	return &service{db: db}
}

// GetStats mengembalikan statistik ringkas dashboard:
//   - active_transactions : jumlah transaksi dengan status 'open'
//   - available_slots     : total slot tersedia dari semua zona aktif
//   - revenue_today       : total calculated_fee transaksi yang exited/paid hari ini
//   - total_vehicles_today: jumlah transaksi yang entry_at = hari ini
func (s *service) GetStats(ctx context.Context) (*DashboardStats, error) {
	var stats DashboardStats

	// 1. Transaksi aktif (status = open)
	if err := s.db.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM transactions WHERE status = 'open'`).
		Scan(&stats.ActiveTransactions).Error; err != nil {
		return nil, errors.FromDB(err, "")
	}

	// 2. Slot tersedia (sum available_count dari snapshot terbaru tiap zona aktif)
	if err := s.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(latest.available_count), 0)
		FROM zones z
		LEFT JOIN LATERAL (
			SELECT available_count
			FROM zone_capacity_logs
			WHERE zone_id = z.id
			ORDER BY recorded_at DESC
			LIMIT 1
		) latest ON true
		WHERE z.is_active = true
	`).Scan(&stats.AvailableSlots).Error; err != nil {
		return nil, errors.FromDB(err, "")
	}

	// 3. Pendapatan hari ini (sum calculated_fee transaksi paid/exited hari ini)
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)
	if err := s.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(calculated_fee), 0)
		FROM transactions
		WHERE status IN ('paid', 'exited')
		  AND updated_at >= ? AND updated_at < ?
	`, today, tomorrow).Scan(&stats.RevenueToday).Error; err != nil {
		return nil, errors.FromDB(err, "")
	}

	// 4. Total kendaraan masuk hari ini (entry_at = hari ini, semua status kecuali cancelled)
	if err := s.db.WithContext(ctx).Raw(`
		SELECT COUNT(*)
		FROM transactions
		WHERE entry_at >= ? AND entry_at < ?
		  AND status != 'cancelled'
	`, today, tomorrow).Scan(&stats.TotalVehiclesToday).Error; err != nil {
		return nil, errors.FromDB(err, "")
	}

	return &stats, nil
}

// GetOperatorDashboard data untuk petugas lapangan.
func (s *service) GetOperatorDashboard(ctx context.Context) (*OperatorDashboardData, error) {
	var data OperatorDashboardData

	// Get overall stats
	stats, err := s.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Basic Stats
	if err := s.db.WithContext(ctx).Raw(`SELECT COALESCE(SUM(capacity), 0) FROM zones WHERE is_active = true`).Scan(&data.TotalCapacity).Error; err != nil {
		return nil, errors.FromDB(err, "")
	}
	data.OccupiedSlots = stats.ActiveTransactions
	data.ActiveEntries = stats.ActiveTransactions
	if data.TotalCapacity > 0 {
		data.AvailabilityPct = float64(data.TotalCapacity-data.OccupiedSlots) / float64(data.TotalCapacity) * 100
	}

	// 2. Zone Breakdown
	if err := s.db.WithContext(ctx).Raw(`
		SELECT z.id, z.name, z.capacity,
			COALESCE(latest.occupied_count, 0) as occupied_count
		FROM zones z
		LEFT JOIN LATERAL (
			SELECT occupied_count
			FROM zone_capacity_logs
			WHERE zone_id = z.id
			ORDER BY recorded_at DESC
			LIMIT 1
		) latest ON true
		WHERE z.is_active = true
	`).Scan(&data.ZoneStatus).Error; err != nil {
		return nil, errors.FromDB(err, "")
	}

	// Calculate percentages and statuses for each zone
	for i := range data.ZoneStatus {
		z := &data.ZoneStatus[i]
		if z.Capacity > 0 {
			z.OccupancyPct = float64(z.OccupiedCount) / float64(z.Capacity) * 100
		}
		if z.OccupancyPct >= 95 {
			z.Status = "full"
		} else if z.OccupancyPct >= 80 {
			z.Status = "warning"
		} else {
			z.Status = "normal"
		}
	}

	return &data, nil
}

// GetOwnerDashboard data untuk pemilik/manajemen.
func (s *service) GetOwnerDashboard(ctx context.Context) (*OwnerDashboardData, error) {
	var data OwnerDashboardData
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	// 1. Overall stats
	stats, err := s.GetStats(ctx)
	if err != nil {
		return nil, err
	}
	data.RevenueToday = stats.RevenueToday
	data.TransactionCount = stats.TotalVehiclesToday

	// 2. Payment Distribution
	if err := s.db.WithContext(ctx).Raw(`
		SELECT method, COUNT(*) as count, SUM(amount) as amount
		FROM payments
		WHERE status = 'completed' AND paid_at >= ? AND paid_at < ?
		GROUP BY method
	`, today, tomorrow).Scan(&data.PaymentDistribution).Error; err != nil {
		return nil, errors.FromDB(err, "")
	}

	// 3. Efficiency (simplistic placeholder)
	var totalCapacity int64
	s.db.WithContext(ctx).Raw(`SELECT SUM(capacity) FROM zones WHERE is_active = true`).Scan(&totalCapacity)
	if totalCapacity > 0 {
		data.OccupancyEfficiency = (float64(stats.ActiveTransactions) / float64(totalCapacity)) * 100
	}

	return &data, nil
}

// GetAdminDashboard data untuk administrator sistem.
func (s *service) GetAdminDashboard(ctx context.Context) (*AdminDashboardData, error) {
	var data AdminDashboardData

	// 1. User Summary
	if err := s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM users WHERE is_active = true`).Scan(&data.TotalUsers).Error; err != nil {
		return nil, errors.FromDB(err, "")
	}

	// 2. Roles
	roles, err := s.GetUserRoleSummary(ctx)
	if err != nil {
		return nil, err
	}
	data.RolesSummary = roles

	// 3. Health
	data.SystemHealth = "all_ok"

	return &data, nil
}

// GetEngineerDashboard data untuk teknisi.
func (s *service) GetEngineerDashboard(ctx context.Context) (*EngineerDashboardData, error) {
	var data EngineerDashboardData

	s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM gates`).Scan(&data.TotalGates)
	s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM gates WHERE is_active = true`).Scan(&data.ActiveGates)
	s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM kiosks`).Scan(&data.TotalKiosks)
	s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM kiosks WHERE is_active = true`).Scan(&data.ActiveKiosks)

	yesterday := time.Now().Add(-24 * time.Hour)
	s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM audit_logs WHERE level = 'error' AND created_at > ?`, yesterday).Scan(&data.RecentErrors)

	return &data, nil
}

// GetCashierDashboard data untuk kasir.
func (s *service) GetCashierDashboard(ctx context.Context) (*CashierDashboardData, error) {
	var data CashierDashboardData
	today := time.Now().Truncate(24 * time.Hour)

	s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM transactions WHERE status = 'awaiting_payment'`).Scan(&data.AwaitingPaymentCount)
	s.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM transactions WHERE status IN ('paid', 'exited') AND updated_at >= ?`, today).Scan(&data.CompletedTodayCount)

	return &data, nil
}

// GetUserRoleSummary mengembalikan jumlah user aktif per role.
func (s *service) GetUserRoleSummary(ctx context.Context) ([]UserRoleSummary, error) {
	var result []UserRoleSummary

	err := s.db.WithContext(ctx).Raw(`
		SELECT r.name AS role_name, COUNT(u.id) AS user_count
		FROM roles r
		LEFT JOIN users u ON u.role_id = r.id
			AND u.is_active = true
			AND u.deleted_at IS NULL
		GROUP BY r.name
		ORDER BY r.name ASC
	`).Scan(&result).Error

	if err != nil {
		return nil, errors.FromDB(err, "")
	}
	return result, nil
}
