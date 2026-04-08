package zone

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"parkieee/pkg/errors"
	"parkieee/pkg/types"
)

type zoneRepository struct {
	db *gorm.DB
}

func NewZoneRepository(db *gorm.DB) ZoneRepositoryPort {
	return &zoneRepository{db: db}
}

func (r *zoneRepository) FindByID(ctx context.Context, id uuid.UUID) (*Zone, error) {
	var zone Zone
	err := r.db.WithContext(ctx).First(&zone, "id = ?", id).Error
	return &zone, errors.FromDB(err, "zone not found")
}

func (r *zoneRepository) FindAll(ctx context.Context, filter ListZoneFilter, page, pageSize int) ([]Zone, int64, error) {
	var zones []Zone
	var total int64

	q := r.db.WithContext(ctx).Model(&Zone{})
	if filter.Active != nil {
		q = q.Where("is_active = ?", *filter.Active)
	}
	if filter.Search != "" {
		q = q.Where("name ILIKE ? OR description ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	if filter.HasFee != nil && *filter.HasFee {
		q = q.Where("additional_fee > 0")
	}
	if filter.MinCapacity != nil {
		q = q.Where("capacity >= ?", *filter.MinCapacity)
	}
	if filter.MaxCapacity != nil {
		q = q.Where("capacity <= ?", *filter.MaxCapacity)
	}
	if filter.MinFee != nil {
		q = q.Where("additional_fee >= ?", *filter.MinFee)
	}
	if filter.MaxFee != nil {
		q = q.Where("additional_fee <= ?", *filter.MaxFee)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	sortCol := "name"
	sortOrder := "asc"
	if filter.SortBy != "" {
		sortCol = filter.SortBy
	}
	if strings.ToLower(filter.SortOrder) == "desc" {
		sortOrder = "desc"
	}

	offset := (page - 1) * pageSize
	err := q.Order(fmt.Sprintf("%s %s", sortCol, sortOrder)).Offset(offset).Limit(pageSize).Find(&zones).Error
	return zones, total, errors.FromDB(err, "")
}

func (r *zoneRepository) Create(ctx context.Context, zone *Zone) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(zone).Error, "")
}

func (r *zoneRepository) Update(ctx context.Context, zone *Zone) error {
	return errors.FromDB(
		r.db.WithContext(ctx).Model(zone).Select("*").Updates(zone).Error,
		"",
	)
}

func (r *zoneRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&Zone{}).
		Where("id = ?", id).
		Update("is_active", false)
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "zone not found")
	}
	return nil
}

type gateRepository struct {
	db *gorm.DB
}

func NewGateRepository(db *gorm.DB) GateRepositoryPort {
	return &gateRepository{db: db}
}

func (r *gateRepository) FindByID(ctx context.Context, id uuid.UUID) (*Gate, error) {
	var gate Gate
	err := r.db.WithContext(ctx).Preload("Zone").First(&gate, "id = ?", id).Error
	return &gate, errors.FromDB(err, "gate not found")
}

func (r *gateRepository) FindByZoneID(ctx context.Context, filter ListGateFilter, page, pageSize int) ([]Gate, int64, error) {
	var gates []Gate
	var total int64

	q := r.db.WithContext(ctx).Model(&Gate{})
	if filter.ZoneID != nil {
		q = q.Where("zone_id = ?", *filter.ZoneID)
	}
	if filter.Active != nil {
		q = q.Where("is_active = ?", *filter.Active)
	}
	if filter.Search != "" {
		q = q.Where("name ILIKE ? OR location_desc ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	sortCol := "name"
	sortOrder := "asc"
	if filter.SortBy != "" {
		sortCol = filter.SortBy
	}
	if strings.ToLower(filter.SortOrder) == "desc" {
		sortOrder = "desc"
	}

	offset := (page - 1) * pageSize
	err := q.Order(fmt.Sprintf("%s %s", sortCol, sortOrder)).Offset(offset).Limit(pageSize).Find(&gates).Error
	return gates, total, errors.FromDB(err, "")
}

func (r *gateRepository) FindAll(ctx context.Context, filter ListGateFilter, page, pageSize int) ([]Gate, int64, error) {
	var gates []Gate
	var total int64

	q := r.db.WithContext(ctx).Model(&Gate{})
	if filter.ZoneID != nil {
		q = q.Where("zone_id = ?", *filter.ZoneID)
	}
	if filter.GateType != nil {
		q = q.Where("gate_type = ?", *filter.GateType)
	}
	if filter.Active != nil {
		q = q.Where("is_active = ?", *filter.Active)
	}
	if filter.Search != "" {
		q = q.Where("name ILIKE ? OR location_desc ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	sortCol := "name"
	sortOrder := "asc"
	if filter.SortBy != "" {
		sortCol = filter.SortBy
	}
	if strings.ToLower(filter.SortOrder) == "desc" {
		sortOrder = "desc"
	}

	offset := (page - 1) * pageSize
	err := q.Preload("Zone").Order(fmt.Sprintf("%s %s", sortCol, sortOrder)).Offset(offset).Limit(pageSize).Find(&gates).Error
	return gates, total, errors.FromDB(err, "")
}

func (r *gateRepository) FindByToken(ctx context.Context, token string) (*Gate, error) {
	var gate Gate
	err := r.db.WithContext(ctx).Preload("Zone").First(&gate, "gate_token = ? AND is_active = true", token).Error
	return &gate, errors.FromDB(err, "gate not found")
}

func (r *gateRepository) Create(ctx context.Context, gate *Gate) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(gate).Error, "")
}

func (r *gateRepository) Update(ctx context.Context, gate *Gate) error {
	return errors.FromDB(
		r.db.WithContext(ctx).Model(gate).Select("*").Updates(gate).Error,
		"",
	)
}

func (r *gateRepository) UpdateTokenLastUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return errors.FromDB(
		r.db.WithContext(ctx).Model(&Gate{}).Where("id = ?", id).Update("token_last_used_at", now).Error,
		"",
	)
}

func (r *gateRepository) UpdateMode(ctx context.Context, id uuid.UUID, mode types.GateMode) error {
	result := r.db.WithContext(ctx).
		Model(&Gate{}).
		Where("id = ?", id).
		Update("mode", mode)
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "gate not found")
	}
	return nil
}

func (r *gateRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&Gate{}).
		Where("id = ?", id).
		Update("is_active", false)
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "gate not found")
	}
	return nil
}

type gateCashierAssignmentRepository struct {
	db *gorm.DB
}

func NewGateCashierAssignmentRepository(db *gorm.DB) GateCashierAssignmentRepositoryPort {
	return &gateCashierAssignmentRepository{db: db}
}

func (r *gateCashierAssignmentRepository) Upsert(ctx context.Context, a *GateCashierAssignment) error {
	return errors.FromDB(
		r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "gate_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"user_id",
					"assigned_by",
					"assigned_at",
				}),
			}).
			Create(a).Error,
		"",
	)
}

func (r *gateCashierAssignmentRepository) FindByGateID(ctx context.Context, gateID uuid.UUID) (*GateCashierAssignment, error) {
	var a GateCashierAssignment
	err := r.db.WithContext(ctx).First(&a, "gate_id = ?", gateID).Error
	return &a, errors.FromDB(err, "cashier assignment not found")
}

func (r *gateCashierAssignmentRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*GateCashierAssignment, error) {
	var a GateCashierAssignment
	err := r.db.WithContext(ctx).First(&a, "user_id = ?", userID).Error
	return &a, errors.FromDB(err, "cashier assignment not found")
}

func (r *gateCashierAssignmentRepository) DeleteByGateID(ctx context.Context, gateID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("gate_id = ?", gateID).
		Delete(&GateCashierAssignment{})
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "cashier assignment not found")
	}
	return nil
}

type gateDeviceRepository struct {
	db *gorm.DB
}

func NewGateDeviceRepository(db *gorm.DB) GateDeviceRepositoryPort {
	return &gateDeviceRepository{db: db}
}

func (r *gateDeviceRepository) FindByID(ctx context.Context, id uuid.UUID) (*GateDevice, error) {
	var device GateDevice
	err := r.db.WithContext(ctx).First(&device, "id = ?", id).Error
	return &device, errors.FromDB(err, "gate device not found")
}

func (r *gateDeviceRepository) FindByGateID(ctx context.Context, gateID uuid.UUID) ([]GateDevice, error) {
	var devices []GateDevice
	err := r.db.WithContext(ctx).
		Where("gate_id = ?", gateID).
		Order("device_type ASC").
		Find(&devices).Error
	return devices, errors.FromDB(err, "")
}

func (r *gateDeviceRepository) Upsert(ctx context.Context, device *GateDevice) error {
	return errors.FromDB(
		r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"status",
					"last_ping_at",
					"error_message",
					"updated_at",
				}),
			}).
			Create(device).Error,
		"",
	)
}

func (r *gateDeviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&GateDevice{}, "id = ?", id)
	if result.Error != nil {
		return errors.FromDB(result.Error, "")
	}
	if result.RowsAffected == 0 {
		return errors.New(errors.ErrNotFound, "gate device not found")
	}
	return nil
}

type capacityLogRepository struct {
	db *gorm.DB
}

func NewCapacityLogRepository(db *gorm.DB) CapacityLogRepositoryPort {
	return &capacityLogRepository{db: db}
}

func (r *capacityLogRepository) Append(ctx context.Context, log *ZoneCapacityLog) error {
	return errors.FromDB(r.db.WithContext(ctx).Create(log).Error, "")
}

func (r *capacityLogRepository) LatestByZoneID(ctx context.Context, zoneID uuid.UUID) (*ZoneCapacityLog, error) {
	var log ZoneCapacityLog
	err := r.db.WithContext(ctx).
		Where("zone_id = ?", zoneID).
		Order("recorded_at DESC").
		First(&log).Error
	if err != nil {
		return nil, errors.FromDB(err, "capacity log not found")
	}
	return &log, nil
}

// AllCapacities returns the latest capacity snapshot for every active zone in a
// single aggregated query. Zones with no log entry yet return occupied=0,
// available=capacity derived from the zones table.
//
// Strategy: LEFT JOIN zones ON latest log per zone (DISTINCT ON zone_id),
// fall back to capacity from zones table when no log exists.
func (r *capacityLogRepository) AllCapacities(ctx context.Context) ([]ZoneCapacityResponse, error) {
	type row struct {
		ZoneID         uuid.UUID `gorm:"column:zone_id"`
		ZoneName       string    `gorm:"column:zone_name"`
		Capacity       int       `gorm:"column:capacity"`
		OccupiedCount  int       `gorm:"column:occupied_count"`
		AvailableCount int       `gorm:"column:available_count"`
	}

	var rows []row

	// DISTINCT ON is PostgreSQL-specific — fine since the project uses Postgres.
	// For zones with no log, COALESCE falls back to 0 / capacity.
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			z.id          AS zone_id,
			z.name        AS zone_name,
			z.capacity    AS capacity,
			COALESCE(latest.occupied_count,  0)          AS occupied_count,
			COALESCE(latest.available_count, z.capacity) AS available_count
		FROM zones z
		LEFT JOIN LATERAL (
			SELECT occupied_count, available_count
			FROM zone_capacity_logs
			WHERE zone_id = z.id
			ORDER BY recorded_at DESC
			LIMIT 1
		) latest ON true
		WHERE z.is_active = true
		ORDER BY z.name ASC
	`).Scan(&rows).Error

	if err != nil {
		return nil, errors.FromDB(err, "")
	}

	result := make([]ZoneCapacityResponse, 0, len(rows))
	for _, r := range rows {
		result = append(result, ZoneCapacityResponse{
			ZoneID:         r.ZoneID,
			ZoneName:       r.ZoneName,
			Capacity:       r.Capacity,
			OccupiedCount:  r.OccupiedCount,
			AvailableCount: r.AvailableCount,
		})
	}
	return result, nil
}

func GetCurrentOccupancy(ctx context.Context, db *gorm.DB, zoneID uuid.UUID, capacity int) (occupied int, available int, err error) {
	repo := &capacityLogRepository{db: db}
	log, err := repo.LatestByZoneID(ctx, zoneID)
	if err != nil {
		if errors.IsCode(err, errors.ErrNotFound) {
			return 0, capacity, nil
		}
		return 0, 0, err
	}
	return log.OccupiedCount, log.AvailableCount, nil
}

func NextOccupancy(current *ZoneCapacityLog, capacity int, event types.ZoneEventType) (occupied int, available int) {
	var base int
	if current != nil {
		base = current.OccupiedCount
	}
	if event == types.ZoneEventEntry {
		occupied = base + 1
	} else {
		occupied = base - 1
		if occupied < 0 {
			occupied = 0
		}
	}
	available = capacity - occupied
	if available < 0 {
		available = 0
	}
	return occupied, available
}
