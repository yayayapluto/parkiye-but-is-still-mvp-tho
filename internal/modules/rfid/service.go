package rfid

import (
	"context"
	"time"

	"github.com/google/uuid"

	"parkieee/pkg/errors"
	"parkieee/pkg/logger"
)

type service struct {
	repo RFIDCardRepositoryPort
	log  logger.Logger
}

func NewService(repo RFIDCardRepositoryPort, log logger.Logger) ServicePort {
	return &service{repo: repo, log: log}
}

func (s *service) ListCards(ctx context.Context, onlyActive bool, page, pageSize int) ([]RFIDCard, int64, error) {
	cards, total, err := s.repo.FindAll(ctx, onlyActive, page, pageSize)
	if err != nil {
		s.log.Error(ctx, "list rfid cards failed", "error", err)
		return nil, 0, err
	}
	s.log.Debug(ctx, "rfid cards listed", "count", len(cards), "total", total, "only_active", onlyActive)
	return cards, total, nil
}

func (s *service) GetCard(ctx context.Context, id uuid.UUID) (*RFIDCard, error) {
	card, err := s.repo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "get rfid card failed: not found", "card_id", id)
		return nil, err
	}
	s.log.Debug(ctx, "rfid card fetched", "card_id", id, "card_uid", card.CardUID)
	return card, nil
}

func (s *service) GetCardByUID(ctx context.Context, cardUID string) (*RFIDCard, error) {
	card, err := s.repo.FindByUID(ctx, cardUID)
	if err != nil {
		s.log.Warn(ctx, "get rfid card by uid failed: not found", "card_uid", cardUID)
		return nil, err
	}
	s.log.Debug(ctx, "rfid card fetched by uid", "card_id", card.ID, "card_uid", cardUID)
	return card, nil
}

func (s *service) RegisterOrGet(ctx context.Context, cardUID string) (*RFIDCard, error) {
	existing, err := s.repo.FindByUID(ctx, cardUID)
	if err == nil {
		s.log.Debug(ctx, "rfid card already exists, returning existing", "card_id", existing.ID, "card_uid", cardUID)
		return existing, nil
	}
	if !errors.IsCode(err, errors.ErrNotFound) {
		s.log.Error(ctx, "failed to look up rfid card", "card_uid", cardUID, "error", err)
		return nil, err
	}

	card := &RFIDCard{
		ID:        uuid.New(),
		CardUID:   cardUID,
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, card); err != nil {
		s.log.Error(ctx, "failed to register rfid card", "card_uid", cardUID, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "rfid card registered", "id", card.ID, "card_uid", cardUID)
	return s.repo.FindByID(ctx, card.ID)
}

func (s *service) LinkVehicle(ctx context.Context, id uuid.UUID, vehicleID uuid.UUID) (*RFIDCard, error) {
	card, err := s.repo.FindByID(ctx, id)
	if err != nil {
		s.log.Warn(ctx, "link vehicle failed: rfid card not found", "card_id", id)
		return nil, err
	}

	if !card.IsActive {
		s.log.Warn(ctx, "link vehicle failed: card inactive", "card_id", id)
		return nil, errors.New(errors.ErrForbidden, "cannot link vehicle to an inactive card")
	}

	card.VehicleID = &vehicleID
	if err := s.repo.Update(ctx, card); err != nil {
		s.log.Error(ctx, "failed to link vehicle to rfid card", "card_id", id, "vehicle_id", vehicleID, "error", err)
		return nil, err
	}

	s.log.Info(ctx, "rfid card linked to vehicle", "card_id", id, "vehicle_id", vehicleID)
	return s.repo.FindByID(ctx, id)
}

func (s *service) Deactivate(ctx context.Context, id uuid.UUID, operatorID uuid.UUID) error {
	if err := s.repo.Deactivate(ctx, id, operatorID); err != nil {
		s.log.Error(ctx, "failed to deactivate rfid card", "card_id", id, "operator_id", operatorID, "error", err)
		return err
	}
	s.log.Info(ctx, "rfid card deactivated", "card_id", id, "operator_id", operatorID)
	return nil
}
