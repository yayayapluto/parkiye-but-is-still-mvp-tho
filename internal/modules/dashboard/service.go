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
