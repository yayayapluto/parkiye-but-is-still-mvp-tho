package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	feeDomain "parkieee/internal/modules/fee"
	ocrDomain "parkieee/internal/modules/ocr"
	rfidDomain "parkieee/internal/modules/rfid"
	vehicleDomain "parkieee/internal/modules/vehicle"
	zoneDomain "parkieee/internal/modules/zone"
	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
	"parkieee/pkg/qr"
	"parkieee/pkg/types"
)

type service struct {
	db        *gorm.DB
	txRepo    TransactionRepositoryPort
	logRepo   TransactionLogRepositoryPort
	log       logger.Logger
	baseURL   string
	placeName string
	qrSecret  string

	// Cross-module service ports injected for reads only.
	// Writes that must be atomic go through raw tx via the zone domain struct.
	zoneSvc       zoneDomain.ServicePort
	zoneGateRepo  zoneDomain.GateRepositoryPort
	zoneRepo      zoneDomain.ZoneRepositoryPort
	rfidSvc       rfidDomain.ServicePort
	feeSvc        feeDomain.ServicePort
	vehicleSvc    vehicleDomain.ServicePort
	ocrSvc        ocrDomain.ServicePort
	ocrResultRepo ocrDomain.OCRResultRepositoryPort
}

func NewService(
	db *gorm.DB,
	txRepo TransactionRepositoryPort,
	logRepo TransactionLogRepositoryPort,
	zoneSvc zoneDomain.ServicePort,
	zoneGateRepo zoneDomain.GateRepositoryPort,
	zoneRepo zoneDomain.ZoneRepositoryPort,
	rfidSvc rfidDomain.ServicePort,
	feeSvc feeDomain.ServicePort,
	vehicleSvc vehicleDomain.ServicePort,
	ocrSvc ocrDomain.ServicePort,
	ocrResultRepo ocrDomain.OCRResultRepositoryPort,
	log logger.Logger,
	baseURL string,
	placeName string,
	qrSecret string,
) ServicePort {
	return &service{
		db:            db,
		txRepo:        txRepo,
		logRepo:       logRepo,
		log:           log,
		baseURL:       baseURL,
		placeName:     placeName,
		qrSecret:      qrSecret,
		zoneSvc:       zoneSvc,
		zoneGateRepo:  zoneGateRepo,
		zoneRepo:      zoneRepo,
		rfidSvc:       rfidSvc,
		feeSvc:        feeSvc,
		vehicleSvc:    vehicleSvc,
		ocrSvc:        ocrSvc,
		ocrResultRepo: ocrResultRepo,
	}
}

func (s *service) RecordEntry(ctx context.Context, req RecordEntryRequest, operatorID uuid.UUID) (*Transaction, error) {
	gate, zone, err := s.validateEntryGate(ctx, req.EntryGateID)
	if err != nil {
		return nil, err
	}

	rfidCardID, err := s.resolveRFIDForEntry(ctx, req)
	if err != nil {
		return nil, err
	}

	capacity, err := s.checkZoneCapacity(ctx, gate, zone)
	if err != nil {
		return nil, err
	}

	var created *Transaction
	now := time.Now()

	dbErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		code, err := s.generateTransactionCode(ctx, tx, now)
		if err != nil {
			return err
		}

		t, err := buildEntryTransaction(ctx, s.log, req, code, rfidCardID, gate, now, s.baseURL, s.placeName, s.qrSecret)
		if err != nil {
			return err
		}

		if err := s.txRepo.Create(ctx, tx, t); err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		if err := s.logRepo.Append(ctx, tx, &TransactionLog{
			TransactionID:     t.ID,
			ToStatus:          string(types.TransactionStatusOpen),
			Event:             types.EventEntryCreated,
			TriggeredBy:       types.TriggeredByOperator,
			TriggeredByUserID: &operatorID,
			Note: fmt.Sprintf("entry via %s at gate %s (zone: %s)",
				req.EntryMethod, gate.Name, zone.Name),
		}); err != nil {
			return fmt.Errorf("append entry log: %w", err)
		}

		if err := appendCapacityLog(ctx, tx, t.ID, gate.ZoneID, types.ZoneEventEntry, capacity.OccupiedCount, zone.Capacity); err != nil {
			return fmt.Errorf("append capacity log: %w", err)
		}

		created = t
		return nil
	})

	if dbErr != nil {
		s.log.Error(ctx, "entry transaction rolled back",
			"error", dbErr,
			"gate_id", req.EntryGateID,
			"entry_method", req.EntryMethod,
		)
		return nil, errors.Wrap(dbErr, errors.ErrDatabaseError, "failed to record entry")
	}

	s.log.Info(ctx, "entry recorded",
		"tx_id", created.ID,
		"tx_code", created.TransactionCode,
		"gate_id", req.EntryGateID,
		"gate_name", gate.Name,
		"zone_id", gate.ZoneID,
		"zone_name", zone.Name,
		"entry_method", req.EntryMethod,
		"operator_id", operatorID,
	)

	// Dispatch OCR asynchronously — does not block the HTTP response.
	// gate.ZoneID is passed so OCR can resolve the default vehicle type from zone.for_vehicle_type_id.
	if req.EntryPhotoPath != "" {
		go s.ocrSvc.DispatchOCRJob(context.Background(), created.ID, req.EntryPhotoPath, gate.ZoneID, types.OCRPhotoTypeEntry)
	}

	return s.txRepo.FindByID(ctx, created.ID)
}

func (s *service) RecordExit(ctx context.Context, id uuid.UUID, req RecordExitRequest, operatorID uuid.UUID) (*Transaction, error) {
	existing, err := s.txRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Status != types.TransactionStatusOpen {
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf(
			"transaction is not open (current status: %s)", existing.Status,
		))
	}

	exitGate, err := s.validateExitGate(ctx, req.ExitGateID, existing.ZoneID)
	if err != nil {
		return nil, err
	}

	if err := s.validateRFIDForExit(ctx, req, existing); err != nil {
		return nil, err
	}

	vehicleTypeID, err := s.resolveVehicleTypeForFee(ctx, existing, req)
	if err != nil {
		return nil, err
	}

	exitAt := time.Now()

	calculatedFee, err := s.calculateExitFee(ctx, existing, vehicleTypeID, exitAt)
	if err != nil {
		return nil, err
	}

	zone, capacity, err := s.loadZoneAndCapacity(ctx, existing.ZoneID)
	if err != nil {
		return nil, err
	}

	durationMinutes := int(exitAt.Sub(existing.EntryAt).Minutes())

	if err := s.commitExitWrites(ctx, existing, req, exitGate, operatorID, exitAt, calculatedFee, durationMinutes, zone, capacity); err != nil {
		s.log.Error(ctx, "exit transaction rolled back",
			"tx_id", id,
			"exit_gate_id", req.ExitGateID,
			"error", err,
		)
		return nil, errors.Wrap(err, errors.ErrDatabaseError, "failed to record exit")
	}

	s.log.Info(ctx, "exit recorded",
		"tx_id", id,
		"tx_code", existing.TransactionCode,
		"exit_gate_id", req.ExitGateID,
		"exit_gate_name", exitGate.Name,
		"zone_id", existing.ZoneID,
		"zone_name", zone.Name,
		"exit_method", req.ExitMethod,
		"duration_minutes", durationMinutes,
		"calculated_fee", calculatedFee,
		"vehicle_type_id", vehicleTypeID,
		"operator_id", operatorID,
	)

	// Dispatch OCR asynchronously for exit photo — helps verify/correct vehicle identity.
	if req.ExitPhotoPath != "" {
		go s.ocrSvc.DispatchOCRJob(context.Background(), existing.ID, req.ExitPhotoPath, existing.ZoneID, types.OCRPhotoTypeExit)
	}

	return s.txRepo.FindByID(ctx, id)
}

func (s *service) Cancel(ctx context.Context, id uuid.UUID, reason string, operatorID uuid.UUID) (*Transaction, error) {
	existing, err := s.txRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Only open transactions can be cancelled; other statuses are terminal or already paid.
	if existing.Status != types.TransactionStatusOpen {
		return nil, errors.New(errors.ErrValidation, fmt.Sprintf(
			"only open transactions can be cancelled (current status: %s)", existing.Status,
		))
	}

	zone, err := s.zoneRepo.FindByID(ctx, existing.ZoneID)
	if err != nil {
		return nil, err
	}

	capacity, err := s.zoneSvc.GetCapacity(ctx, existing.ZoneID)
	if err != nil {
		return nil, err
	}

	dbErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fromStatus := string(existing.Status)
		existing.Status = types.TransactionStatusCancelled

		if err := s.txRepo.Update(ctx, tx, existing); err != nil {
			return fmt.Errorf("update transaction: %w", err)
		}

		if err := s.logRepo.Append(ctx, tx, &TransactionLog{
			TransactionID:     existing.ID,
			FromStatus:        &fromStatus,
			ToStatus:          string(types.TransactionStatusCancelled),
			Event:             types.EventCancelled,
			TriggeredBy:       types.TriggeredByOperator,
			TriggeredByUserID: &operatorID,
			Note:              "cancelled: " + reason,
		}); err != nil {
			return fmt.Errorf("append cancel log: %w", err)
		}

		// Release the occupied slot back to the zone.
		if err := appendCapacityLog(ctx, tx, existing.ID, existing.ZoneID, types.ZoneEventExit, capacity.OccupiedCount, zone.Capacity); err != nil {
			return fmt.Errorf("append capacity log: %w", err)
		}

		return nil
	})

	if dbErr != nil {
		s.log.Error(ctx, "cancel transaction rolled back",
			"tx_id", id,
			"error", dbErr,
		)
		return nil, errors.Wrap(dbErr, errors.ErrDatabaseError, "failed to cancel transaction")
	}

	s.log.Info(ctx, "transaction cancelled",
		"tx_id", id,
		"tx_code", existing.TransactionCode,
		"reason", reason,
		"operator_id", operatorID,
	)

	return s.txRepo.FindByID(ctx, id)
}

func (s *service) MarkPaid(ctx context.Context, txID uuid.UUID, triggeredBy types.TriggeredBy, handledByUserID *uuid.UUID) error {
	existing, err := s.txRepo.FindByID(ctx, txID)
	if err != nil {
		s.log.Warn(ctx, "mark paid: transaction not found", "tx_id", txID)
		return err
	}
	if existing.Status != types.TransactionStatusAwaitingPayment {
		return errors.New(errors.ErrValidation, fmt.Sprintf(
			"transaction is not awaiting payment (current status: %s)", existing.Status,
		))
	}

	dbErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fromStatus := string(existing.Status)
		existing.Status = types.TransactionStatusPaid
		if err := s.txRepo.Update(ctx, tx, existing); err != nil {
			return fmt.Errorf("update transaction: %w", err)
		}
		return s.logRepo.Append(ctx, tx, &TransactionLog{
			TransactionID:     existing.ID,
			FromStatus:        &fromStatus,
			ToStatus:          string(types.TransactionStatusPaid),
			Event:             types.EventPaymentReceived,
			TriggeredBy:       triggeredBy,
			TriggeredByUserID: handledByUserID,
		})
	})
	if dbErr != nil {
		s.log.Error(ctx, "mark paid: transaction rolled back", "tx_id", txID, "error", dbErr)
		return errors.Wrap(dbErr, errors.ErrDatabaseError, "failed to mark transaction as paid")
	}
	s.log.Info(ctx, "transaction marked paid", "tx_id", txID, "triggered_by", triggeredBy)
	return nil
}

func (s *service) MarkExited(ctx context.Context, txID uuid.UUID, triggeredBy types.TriggeredBy) error {
	existing, err := s.txRepo.FindByID(ctx, txID)
	if err != nil {
		s.log.Warn(ctx, "mark exited: transaction not found", "tx_id", txID)
		return err
	}
	if existing.Status != types.TransactionStatusPaid {
		return errors.New(errors.ErrValidation, fmt.Sprintf(
			"transaction is not paid (current status: %s)", existing.Status,
		))
	}

	dbErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fromStatus := string(existing.Status)
		existing.Status = types.TransactionStatusExited
		if err := s.txRepo.Update(ctx, tx, existing); err != nil {
			return fmt.Errorf("update transaction: %w", err)
		}
		return s.logRepo.Append(ctx, tx, &TransactionLog{
			TransactionID: existing.ID,
			FromStatus:    &fromStatus,
			ToStatus:      string(types.TransactionStatusExited),
			Event:         types.EventExitRecorded,
			TriggeredBy:   triggeredBy,
		})
	})
	if dbErr != nil {
		s.log.Error(ctx, "mark exited: transaction rolled back", "tx_id", txID, "error", dbErr)
		return errors.Wrap(dbErr, errors.ErrDatabaseError, "failed to mark transaction as exited")
	}
	s.log.Info(ctx, "transaction marked exited", "tx_id", txID, "triggered_by", triggeredBy)
	return nil
}

func (s *service) GetTransaction(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	tx, err := s.txRepo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "get transaction: not found", "tx_id", id)
		return nil, err
	}
	s.log.Debug(ctx, "get transaction", "tx_id", id, "status", tx.Status)
	return tx, nil
}

func (s *service) GetByCode(ctx context.Context, code string) (*Transaction, error) {
	tx, err := s.txRepo.FindByCode(ctx, code)
	if err != nil {
		s.log.Warn(ctx, "get transaction by code: not found", "code", code)
		return nil, err
	}
	s.log.Debug(ctx, "get transaction by code", "code", code, "tx_id", tx.ID)
	return tx, nil
}

func (s *service) GetOpenByRFIDUID(ctx context.Context, uid string) (*Transaction, error) {
	card, err := s.rfidSvc.GetCardByUID(ctx, uid)
	if err != nil {
		s.log.Warn(ctx, "get open by rfid: card not found", "uid", uid)
		return nil, errors.New(errors.ErrNotFound, "rfid card not found")
	}
	// Cari open dulu, kalau tidak ada cari awaiting_payment
	// (kendaraan tempel kartu ulang setelah exit tercatat)
	tx, err := s.txRepo.FindOpenByRFIDCard(ctx, card.ID)
	if err != nil {
		tx, err = s.txRepo.FindAwaitingPaymentByRFIDCard(ctx, card.ID)
		if err != nil {
			s.log.Warn(ctx, "get open by rfid: no active transaction", "uid", uid, "card_id", card.ID)
			return nil, errors.New(errors.ErrNotFound, "no active transaction for this rfid card")
		}
	}
	s.log.Debug(ctx, "get open by rfid", "uid", uid, "card_id", card.ID, "tx_id", tx.ID, "status", tx.Status)
	return tx, nil
}

func (s *service) LoadOCRSummary(ctx context.Context, txID uuid.UUID) []ocrDomain.OCRResultWithJob {
	results, err := s.ocrResultRepo.FindSummaryByTransactionID(ctx, txID)
	if err != nil || len(results) == 0 {
		s.log.Debug(ctx, "load ocr summary: empty", "tx_id", txID)
		return nil
	}
	s.log.Debug(ctx, "load ocr summary", "tx_id", txID, "count", len(results))
	return results
}

func (s *service) ListTransactions(ctx context.Context, filter ListFilter, page, pageSize int) ([]Transaction, int64, error) {
	txs, total, err := s.txRepo.FindAll(ctx, filter, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list transactions: db error", "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "list transactions", "count", len(txs), "total", total, "page", page)
	return txs, total, nil
}

func (s *service) SimulateEntryTime(ctx context.Context, id uuid.UUID, minutesAgo int) (*Transaction, error) {
	existing, err := s.txRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Status != types.TransactionStatusOpen {
		return nil, errors.New(errors.ErrValidation, "only open transactions can be simulated")
	}
	newEntryAt := time.Now().Add(-time.Duration(minutesAgo) * time.Minute)
	existing.EntryAt = newEntryAt
	dbErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.txRepo.Update(ctx, tx, existing)
	})
	if dbErr != nil {
		return nil, errors.Wrap(dbErr, errors.ErrDatabaseError, "failed to simulate entry time")
	}
	s.log.Info(ctx, "[SIM] entry_at backdated", "tx_id", id, "minutes_ago", minutesAgo, "new_entry_at", newEntryAt)
	return s.txRepo.FindByID(ctx, id)
}

func (s *service) MarkPaidAndExited(ctx context.Context, txID uuid.UUID, triggeredBy types.TriggeredBy, handledByUserID *uuid.UUID) error {
	existing, err := s.txRepo.FindByID(ctx, txID)
	if err != nil {
		s.log.Warn(ctx, "mark paid and exited: transaction not found", "tx_id", txID)
		return err
	}
	if existing.Status != types.TransactionStatusAwaitingPayment {
		return errors.New(errors.ErrValidation, fmt.Sprintf(
			"transaction is not awaiting payment (current status: %s)", existing.Status,
		))
	}

	dbErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Step 1: awaiting_payment → paid
		fromPaid := string(existing.Status)
		existing.Status = types.TransactionStatusPaid
		if err := s.txRepo.Update(ctx, tx, existing); err != nil {
			return fmt.Errorf("mark paid: %w", err)
		}
		if err := s.logRepo.Append(ctx, tx, &TransactionLog{
			TransactionID:     existing.ID,
			FromStatus:        &fromPaid,
			ToStatus:          string(types.TransactionStatusPaid),
			Event:             types.EventPaymentReceived,
			TriggeredBy:       triggeredBy,
			TriggeredByUserID: handledByUserID,
		}); err != nil {
			return fmt.Errorf("log paid: %w", err)
		}

		// Step 2: paid → exited
		fromExited := string(types.TransactionStatusPaid)
		existing.Status = types.TransactionStatusExited
		if err := s.txRepo.Update(ctx, tx, existing); err != nil {
			return fmt.Errorf("mark exited: %w", err)
		}
		if err := s.logRepo.Append(ctx, tx, &TransactionLog{
			TransactionID: existing.ID,
			FromStatus:    &fromExited,
			ToStatus:      string(types.TransactionStatusExited),
			Event:         types.EventExitRecorded,
			TriggeredBy:   triggeredBy,
		}); err != nil {
			return fmt.Errorf("log exited: %w", err)
		}
		return nil
	})
	if dbErr != nil {
		s.log.Error(ctx, "mark paid and exited: transaction rolled back", "tx_id", txID, "error", dbErr)
		return errors.Wrap(dbErr, errors.ErrDatabaseError, "failed to mark transaction paid and exited")
	}
	s.log.Info(ctx, "transaction marked paid and exited", "tx_id", txID, "triggered_by", triggeredBy)
	return nil
}

func (s *service) GetLogs(ctx context.Context, txID uuid.UUID) ([]TransactionLog, error) {
	if _, err := s.txRepo.FindByID(ctx, txID); err != nil {
		s.log.Warn(ctx, "get logs: transaction not found", "tx_id", txID)
		return nil, err
	}
	logs, err := s.logRepo.FindByTransactionID(ctx, txID)
	if err != nil {
		s.log.Error(ctx, "get logs: db error", "tx_id", txID, "error", err)
		return nil, err
	}
	s.log.Debug(ctx, "get logs", "tx_id", txID, "count", len(logs))
	return logs, nil
}

func (s *service) StampPlateMismatch(ctx context.Context, txID uuid.UUID, mismatch bool) error {
	existing, err := s.txRepo.FindByID(ctx, txID)
	if err != nil {
		s.log.Warn(ctx, "stamp plate mismatch: transaction not found", "tx_id", txID)
		return err
	}
	if existing.Status == types.TransactionStatusCancelled || existing.Status == types.TransactionStatusExited {
		s.log.Debug(ctx, "stamp plate mismatch: skipped (terminal status)", "tx_id", txID, "status", existing.Status)
		return nil
	}
	if err := s.db.WithContext(ctx).Model(existing).Update("plate_mismatch", mismatch).Error; err != nil {
		s.log.Error(ctx, "stamp plate mismatch: db error", "tx_id", txID, "error", err)
		return err
	}
	s.log.Info(ctx, "plate mismatch stamped", "tx_id", txID, "mismatch", mismatch)
	return nil
}

func (s *service) validateEntryGate(ctx context.Context, gateID uuid.UUID) (*zoneDomain.Gate, *zoneDomain.Zone, error) {
	gate, err := s.zoneGateRepo.FindByID(ctx, gateID)
	if err != nil {
		return nil, nil, err
	}
	if gate.GateType != types.GateTypeEntry {
		return nil, nil, errors.New(errors.ErrValidation, "gate is not an entry gate")
	}
	if !gate.IsActive {
		return nil, nil, errors.New(errors.ErrValidation, "entry gate is inactive")
	}

	zone, err := s.zoneRepo.FindByID(ctx, gate.ZoneID)
	if err != nil {
		return nil, nil, err
	}
	if !zone.IsActive {
		return nil, nil, errors.New(errors.ErrValidation, "zone is inactive")
	}

	return gate, zone, nil
}

func (s *service) validateExitGate(ctx context.Context, gateID uuid.UUID, expectedZoneID uuid.UUID) (*zoneDomain.Gate, error) {
	gate, err := s.zoneGateRepo.FindByID(ctx, gateID)
	if err != nil {
		return nil, errors.New(errors.ErrNotFound, "exit gate not found")
	}
	if gate.GateType != types.GateTypeExit {
		return nil, errors.New(errors.ErrValidation, "gate is not an exit gate")
	}
	if !gate.IsActive {
		return nil, errors.New(errors.ErrValidation, "exit gate is inactive")
	}
	if gate.ZoneID != expectedZoneID {
		return nil, errors.New(errors.ErrValidation, "exit gate does not belong to the same zone as the entry gate")
	}
	return gate, nil
}

// resolveRFIDForEntry returns the card ID for RFID entries, or nil for QR entries.
// Also guards against double-entry on the same card.
func (s *service) resolveRFIDForEntry(ctx context.Context, req RecordEntryRequest) (*uuid.UUID, error) {
	if req.EntryMethod == types.EntryMethodQR {
		return nil, nil
	}

	if req.RFIDCardUID == "" {
		return nil, errors.New(errors.ErrValidation, "rfid_card_uid is required for rfid entry method")
	}
	card, err := s.rfidSvc.RegisterOrGet(ctx, req.RFIDCardUID)
	if err != nil {
		return nil, errors.New(errors.ErrInternal, "failed to register rfid card")
	}
	if !card.IsActive {
		return nil, errors.New(errors.ErrValidation, "rfid card is inactive")
	}

	if existing, err := s.txRepo.FindOpenByRFIDCard(ctx, card.ID); err == nil && existing != nil {
		s.log.Info(ctx, "entry rejected: rfid card already has open transaction",
			"card_id", card.ID,
			"card_uid", req.RFIDCardUID,
			"existing_tx_id", existing.ID,
			"existing_tx_code", existing.TransactionCode,
		)
		return nil, errors.New(errors.ErrConflict, fmt.Sprintf(
			"rfid card already has an open transaction: %s", existing.TransactionCode,
		))
	}

	return &card.ID, nil
}

// validateRFIDForExit checks that the presented card matches the card used at entry.
func (s *service) validateRFIDForExit(ctx context.Context, req RecordExitRequest, existing *Transaction) error {
	if req.ExitMethod != types.ExitMethodRFID {
		return nil
	}
	if req.RFIDCardUID == "" {
		return errors.New(errors.ErrValidation, "rfid_card_uid is required for rfid exit method")
	}
	card, err := s.rfidSvc.GetCardByUID(ctx, req.RFIDCardUID)
	if err != nil {
		return errors.New(errors.ErrNotFound, "rfid card not found")
	}
	if !card.IsActive {
		return errors.New(errors.ErrValidation, "rfid card is inactive")
	}
	if existing.RFIDCardID == nil || *existing.RFIDCardID != card.ID {
		s.log.Info(ctx, "exit rejected: rfid card mismatch",
			"tx_id", existing.ID,
			"expected_card_id", existing.RFIDCardID,
			"presented_card_id", card.ID,
			"presented_card_uid", req.RFIDCardUID,
		)
		return errors.New(errors.ErrValidation, "rfid card does not match the card used at entry")
	}
	return nil
}

// resolveVehicleTypeForFee returns vehicle type ID for fee calculation.
// Priority: vehicle linked to tx > vehicle_type_id in request > zone default vehicle type.
func (s *service) resolveVehicleTypeForFee(ctx context.Context, existing *Transaction, req RecordExitRequest) (uuid.UUID, error) {
	if existing.VehicleID != nil {
		vehicle, err := s.vehicleSvc.GetVehicle(ctx, *existing.VehicleID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("get vehicle: %w", err)
		}
		return vehicle.VehicleTypeID, nil
	}
	if req.VehicleTypeID != nil {
		return *req.VehicleTypeID, nil
	}
	// Fallback: pakai default vehicle type dari zone (set saat konfigurasi zona)
	zone, err := s.zoneRepo.FindByID(ctx, existing.ZoneID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get zone for vehicle type fallback: %w", err)
	}
	if zone.ForVehicleTypeID != nil {
		return *zone.ForVehicleTypeID, nil
	}
	return uuid.Nil, errors.New(errors.ErrValidation,
		"vehicle_type_id is required: no vehicle linked, no vehicle_type_id in request, and zone has no default vehicle type")
}

func (s *service) checkZoneCapacity(ctx context.Context, gate *zoneDomain.Gate, zone *zoneDomain.Zone) (*zoneDomain.ZoneCapacityResponse, error) {
	capacity, err := s.zoneSvc.GetCapacity(ctx, gate.ZoneID)
	if err != nil {
		return nil, err
	}
	if capacity.AvailableCount <= 0 {
		s.log.Info(ctx, "entry rejected: zone at full capacity",
			"zone_id", gate.ZoneID,
			"zone_name", zone.Name,
			"capacity", capacity.Capacity,
			"occupied", capacity.OccupiedCount,
		)
		return nil, errors.New(errors.ErrConflict, fmt.Sprintf(
			"zone '%s' is at full capacity (%d/%d)", zone.Name, capacity.OccupiedCount, capacity.Capacity,
		))
	}
	return capacity, nil
}

func (s *service) generateTransactionCode(ctx context.Context, tx *gorm.DB, now time.Time) (string, error) {
	dateStr := now.Format("20060102")
	prefix := "PKR-" + dateStr + "-"
	count, err := s.txRepo.CountByDatePrefix(ctx, tx, prefix)
	if err != nil {
		return "", fmt.Errorf("count transactions: %w", err)
	}
	return fmt.Sprintf("PKR-%s-%05d", dateStr, count+1), nil
}

// calculateExitFee wraps CalculateFee with structured error logging.
func (s *service) calculateExitFee(ctx context.Context, existing *Transaction, vehicleTypeID uuid.UUID, exitAt time.Time) (int, error) {
	fee, err := s.feeSvc.CalculateFee(ctx, existing.ZoneID, vehicleTypeID, existing.EntryAt, exitAt)
	if err != nil {
		s.log.Error(ctx, "fee calculation failed",
			"tx_id", existing.ID,
			"zone_id", existing.ZoneID,
			"vehicle_type_id", vehicleTypeID,
			"entry_at", existing.EntryAt,
			"exit_at", exitAt,
			"error", err,
		)
		return 0, errors.New(errors.ErrInternal, "failed to calculate fee: "+err.Error())
	}
	return fee, nil
}

// buildEntryTransaction constructs the Transaction struct for a new entry.
// For QR entries, the QR code text is derived from the transaction code so it is
// always unique, and a PNG image is generated server-side.
func buildEntryTransaction(ctx context.Context, log logger.Logger, req RecordEntryRequest, code string, rfidCardID *uuid.UUID, gate *zoneDomain.Gate, now time.Time, baseURL, placeName, qrSecret string) (*Transaction, error) {
	var entryQR *string
	var entryQRImage *string
	if req.EntryMethod == types.EntryMethodQR {
		qrText := "PARKIEEE-" + code
		entryQR = &qrText
		imgURL, err := qr.SaveTicket(ctx, log, baseURL, qrSecret, qr.TicketData{
			TransactionCode: code,
			PlaceName:       placeName,
			EntryAt:         now,
			QRText:          qrText,
		})
		if err != nil {
			return nil, fmt.Errorf("save ticket image: %w", err)
		}
		entryQRImage = &imgURL
	}
	var photoURL *string
	if req.EntryPhotoURL != "" {
		photoURL = &req.EntryPhotoURL
	}
	var photoPath *string
	if req.EntryPhotoPath != "" {
		photoPath = &req.EntryPhotoPath
	}
	return &Transaction{
		ID:               uuid.New(),
		TransactionCode:  code,
		EntryGateID:      req.EntryGateID,
		EntryMethod:      req.EntryMethod,
		RFIDCardID:       rfidCardID,
		EntryQRCode:      entryQR,
		EntryQRCodeImage: entryQRImage,
		EntryAt:          now,
		EntryPhotoURL:    photoURL,
		EntryPhotoPath:   photoPath,
		ZoneID:           gate.ZoneID,
		Status:           types.TransactionStatusOpen,
	}, nil
}

func (s *service) loadZoneAndCapacity(ctx context.Context, zoneID uuid.UUID) (*zoneDomain.Zone, *zoneDomain.ZoneCapacityResponse, error) {
	zone, err := s.zoneRepo.FindByID(ctx, zoneID)
	if err != nil {
		return nil, nil, err
	}
	capacity, err := s.zoneSvc.GetCapacity(ctx, zoneID)
	if err != nil {
		return nil, nil, err
	}
	return zone, capacity, nil
}

func (s *service) commitExitWrites(
	ctx context.Context,
	existing *Transaction,
	req RecordExitRequest,
	exitGate *zoneDomain.Gate,
	operatorID uuid.UUID,
	exitAt time.Time,
	calculatedFee int,
	durationMinutes int,
	zone *zoneDomain.Zone,
	capacity *zoneDomain.ZoneCapacityResponse,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		applyExitFields(existing, req, exitAt, calculatedFee)

		if err := s.txRepo.Update(ctx, tx, existing); err != nil {
			return fmt.Errorf("update transaction: %w", err)
		}

		fromStatus := string(types.TransactionStatusOpen)
		if err := s.logRepo.Append(ctx, tx, &TransactionLog{
			TransactionID:     existing.ID,
			FromStatus:        &fromStatus,
			ToStatus:          string(types.TransactionStatusAwaitingPayment),
			Event:             types.EventPaymentInitiated,
			TriggeredBy:       types.TriggeredByOperator,
			TriggeredByUserID: &operatorID,
			Note: fmt.Sprintf(
				"exit via %s at gate %s | duration: %d min | fee: Rp%d",
				req.ExitMethod, exitGate.Name, durationMinutes, calculatedFee,
			),
		}); err != nil {
			return fmt.Errorf("append exit log: %w", err)
		}

		if err := appendCapacityLog(ctx, tx, existing.ID, existing.ZoneID, types.ZoneEventExit, capacity.OccupiedCount, zone.Capacity); err != nil {
			return fmt.Errorf("append capacity log: %w", err)
		}

		return nil
	})
}

// applyExitFields mutates existing with exit data before persisting.
func applyExitFields(existing *Transaction, req RecordExitRequest, exitAt time.Time, calculatedFee int) {
	existing.ExitGateID = &req.ExitGateID
	existing.ExitMethod = &req.ExitMethod
	existing.ExitAt = &exitAt
	existing.CalculatedFee = &calculatedFee
	existing.Status = types.TransactionStatusAwaitingPayment
	if req.ExitPhotoURL != "" {
		existing.ExitPhotoURL = &req.ExitPhotoURL
	}
	if req.ExitPhotoPath != "" {
		existing.ExitPhotoPath = &req.ExitPhotoPath
	}
}

// appendCapacityLog writes a zone_capacity_logs row within the given tx.
// Uses zone.NextOccupancy for count calculation to keep logic in one place.
func appendCapacityLog(
	ctx context.Context,
	tx *gorm.DB,
	transactionID uuid.UUID,
	zoneID uuid.UUID,
	event types.ZoneEventType,
	currentOccupied int,
	zoneCapacity int,
) error {
	current := &zoneDomain.ZoneCapacityLog{OccupiedCount: currentOccupied}
	occupied, available := zoneDomain.NextOccupancy(current, zoneCapacity, event)

	log := zoneDomain.ZoneCapacityLog{
		ID:             uuid.New(),
		ZoneID:         zoneID,
		TransactionID:  transactionID,
		EventType:      event,
		OccupiedCount:  occupied,
		AvailableCount: available,
		RecordedAt:     time.Now(),
	}
	return tx.WithContext(ctx).Create(&log).Error
}
