package zone_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"parkieee/internal/modules/zone"
	zonemocks "parkieee/internal/modules/zone/mocks"
	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/types"
)

type noopLogger struct{}

func (noopLogger) Debug(_ context.Context, _ string, _ ...any) {}
func (noopLogger) Info(_ context.Context, _ string, _ ...any)  {}
func (noopLogger) Warn(_ context.Context, _ string, _ ...any)  {}
func (noopLogger) Error(_ context.Context, _ string, _ ...any) {}
func (noopLogger) Fatal(_ context.Context, _ string, _ ...any) {}
func (n noopLogger) With(_ ...any) logger.Logger               { return n }
func (n noopLogger) WithGroup(_ string) logger.Logger          { return n }

var _ logger.Logger = noopLogger{}

func newService(t *testing.T, zoneRepo zone.ZoneRepositoryPort, capRepo zone.CapacityLogRepositoryPort) zone.ServicePort {
	gateRepo := zonemocks.NewMockGateRepositoryPort(t)
	return zone.NewService(zoneRepo, gateRepo, capRepo, nil, noopLogger{})
}

func TestNextOccupancy(t *testing.T) {
	tests := []struct {
		name             string
		current          *zone.ZoneCapacityLog
		capacity         int
		event            types.ZoneEventType
		expectedOccupied int
		expectedAvail    int
	}{
		{
			name:             "entry from empty zone",
			current:          nil,
			capacity:         10,
			event:            types.ZoneEventEntry,
			expectedOccupied: 1,
			expectedAvail:    9,
		},
		{
			name:             "entry increments occupied",
			current:          &zone.ZoneCapacityLog{OccupiedCount: 5, AvailableCount: 5},
			capacity:         10,
			event:            types.ZoneEventEntry,
			expectedOccupied: 6,
			expectedAvail:    4,
		},
		{
			name:             "exit decrements occupied",
			current:          &zone.ZoneCapacityLog{OccupiedCount: 3, AvailableCount: 7},
			capacity:         10,
			event:            types.ZoneEventExit,
			expectedOccupied: 2,
			expectedAvail:    8,
		},
		{
			name:             "exit from zero does not go negative",
			current:          &zone.ZoneCapacityLog{OccupiedCount: 0, AvailableCount: 10},
			capacity:         10,
			event:            types.ZoneEventExit,
			expectedOccupied: 0,
			expectedAvail:    10,
		},
		{
			name:             "entry at full capacity — available floors at 0",
			current:          &zone.ZoneCapacityLog{OccupiedCount: 10, AvailableCount: 0},
			capacity:         10,
			event:            types.ZoneEventEntry,
			expectedOccupied: 11,
			expectedAvail:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			occupied, avail := zone.NextOccupancy(tc.current, tc.capacity, tc.event)
			assert.Equal(t, tc.expectedOccupied, occupied)
			assert.Equal(t, tc.expectedAvail, avail)
		})
	}
}

func TestRecordCapacityEvent_ZoneNotFound(t *testing.T) {
	zoneRepo := zonemocks.NewMockZoneRepositoryPort(t)
	capRepo := zonemocks.NewMockCapacityLogRepositoryPort(t)
	zoneID := uuid.New()

	zoneRepo.EXPECT().
		FindByID(context.Background(), zoneID).
		Return(nil, errors.New(errors.ErrNotFound, "zone not found"))

	svc := newService(t, zoneRepo, capRepo)
	err := svc.RecordCapacityEvent(context.Background(), zoneID, uuid.New(), types.ZoneEventEntry)

	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrNotFound))
}

func TestRecordCapacityEvent_NoExistingLog_TreatedAsEmpty(t *testing.T) {
	zoneRepo := zonemocks.NewMockZoneRepositoryPort(t)
	capRepo := zonemocks.NewMockCapacityLogRepositoryPort(t)
	zoneID := uuid.New()
	txID := uuid.New()

	zoneRepo.EXPECT().
		FindByID(context.Background(), zoneID).
		Return(&zone.Zone{ID: zoneID, Capacity: 20}, nil)

	capRepo.EXPECT().
		LatestByZoneID(context.Background(), zoneID).
		Return(nil, errors.New(errors.ErrNotFound, "capacity log not found"))

	// Expect Append with occupied=1, available=19 (entry from zero)
	capRepo.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(log *zone.ZoneCapacityLog) bool {
			return log.ZoneID == zoneID &&
				log.TransactionID == txID &&
				log.EventType == types.ZoneEventEntry &&
				log.OccupiedCount == 1 &&
				log.AvailableCount == 19
		})).
		Return(nil)

	svc := newService(t, zoneRepo, capRepo)
	err := svc.RecordCapacityEvent(context.Background(), zoneID, txID, types.ZoneEventEntry)

	require.NoError(t, err)
}

func TestRecordCapacityEvent_EntryIncrementsCount(t *testing.T) {
	zoneRepo := zonemocks.NewMockZoneRepositoryPort(t)
	capRepo := zonemocks.NewMockCapacityLogRepositoryPort(t)
	zoneID := uuid.New()
	txID := uuid.New()

	zoneRepo.EXPECT().
		FindByID(context.Background(), zoneID).
		Return(&zone.Zone{ID: zoneID, Capacity: 10}, nil)

	capRepo.EXPECT().
		LatestByZoneID(context.Background(), zoneID).
		Return(&zone.ZoneCapacityLog{OccupiedCount: 4, AvailableCount: 6}, nil)

	capRepo.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(log *zone.ZoneCapacityLog) bool {
			return log.OccupiedCount == 5 && log.AvailableCount == 5 &&
				log.ZoneID == zoneID && log.TransactionID == txID &&
				log.EventType == types.ZoneEventEntry
		})).
		Return(nil)

	svc := newService(t, zoneRepo, capRepo)
	err := svc.RecordCapacityEvent(context.Background(), zoneID, txID, types.ZoneEventEntry)

	require.NoError(t, err)
}

func TestRecordCapacityEvent_ExitDecrementsCount(t *testing.T) {
	zoneRepo := zonemocks.NewMockZoneRepositoryPort(t)
	capRepo := zonemocks.NewMockCapacityLogRepositoryPort(t)
	zoneID := uuid.New()
	txID := uuid.New()

	zoneRepo.EXPECT().
		FindByID(context.Background(), zoneID).
		Return(&zone.Zone{ID: zoneID, Capacity: 10}, nil)

	capRepo.EXPECT().
		LatestByZoneID(context.Background(), zoneID).
		Return(&zone.ZoneCapacityLog{OccupiedCount: 3, AvailableCount: 7}, nil)

	capRepo.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(log *zone.ZoneCapacityLog) bool {
			return log.OccupiedCount == 2 && log.AvailableCount == 8 &&
				log.EventType == types.ZoneEventExit
		})).
		Return(nil)

	svc := newService(t, zoneRepo, capRepo)
	err := svc.RecordCapacityEvent(context.Background(), zoneID, txID, types.ZoneEventExit)

	require.NoError(t, err)
}

func TestRecordCapacityEvent_LatestLogDBError(t *testing.T) {
	zoneRepo := zonemocks.NewMockZoneRepositoryPort(t)
	capRepo := zonemocks.NewMockCapacityLogRepositoryPort(t)
	zoneID := uuid.New()

	zoneRepo.EXPECT().
		FindByID(context.Background(), zoneID).
		Return(&zone.Zone{ID: zoneID, Capacity: 10}, nil)

	capRepo.EXPECT().
		LatestByZoneID(context.Background(), zoneID).
		Return(nil, fmt.Errorf("connection refused"))

	svc := newService(t, zoneRepo, capRepo)
	err := svc.RecordCapacityEvent(context.Background(), zoneID, uuid.New(), types.ZoneEventEntry)

	require.Error(t, err)
}

func TestRecordCapacityEvent_AppendFails(t *testing.T) {
	zoneRepo := zonemocks.NewMockZoneRepositoryPort(t)
	capRepo := zonemocks.NewMockCapacityLogRepositoryPort(t)
	zoneID := uuid.New()

	zoneRepo.EXPECT().
		FindByID(context.Background(), zoneID).
		Return(&zone.Zone{ID: zoneID, Capacity: 10}, nil)

	capRepo.EXPECT().
		LatestByZoneID(context.Background(), zoneID).
		Return(&zone.ZoneCapacityLog{OccupiedCount: 2, AvailableCount: 8}, nil)

	capRepo.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(_ *zone.ZoneCapacityLog) bool { return true })).
		Return(fmt.Errorf("db write failed"))

	svc := newService(t, zoneRepo, capRepo)
	err := svc.RecordCapacityEvent(context.Background(), zoneID, uuid.New(), types.ZoneEventEntry)

	require.Error(t, err)
}
