package zone

import (
	"context"

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

func (r *zoneRepository) FindAll(ctx context.Context, onlyActive bool, page, pageSize int) ([]Zone, int64, error) {
	var zones []Zone
	var total int64

	q := r.db.WithContext(ctx).Model(&Zone{})
	if onlyActive {
		q = q.Where("is_active = ?", true)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	offset := (page - 1) * pageSize
	err := q.Order("name ASC").Offset(offset).Limit(pageSize).Find(&zones).Error
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

func (r *gateRepository) FindByZoneID(ctx context.Context, zoneID uuid.UUID, onlyActive bool, page, pageSize int) ([]Gate, int64, error) {
	var gates []Gate
	var total int64

	q := r.db.WithContext(ctx).Model(&Gate{}).Where("zone_id = ?", zoneID)
	if onlyActive {
		q = q.Where("is_active = ?", true)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.FromDB(err, "")
	}

	offset := (page - 1) * pageSize
	err := q.Order("name ASC").Offset(offset).Limit(pageSize).Find(&gates).Error
	return gates, total, errors.FromDB(err, "")
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
	// Conflict on PK: update mutable fields saja, bukan gate_id atau device_type.
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

// GetCurrentOccupancy returns occupied and available counts for a zone.
// Falls back to (0, capacity) when no log exists yet.
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

// NextOccupancy computes the new counts after an entry or exit event.
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
