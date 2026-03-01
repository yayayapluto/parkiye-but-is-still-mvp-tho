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
	return s.repo.FindAll(ctx, onlyActive, page, pageSize)
}

func (s *service) GetCard(ctx context.Context, id uuid.UUID) (*RFIDCard, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) GetCardByUID(ctx context.Context, cardUID string) (*RFIDCard, error) {
	return s.repo.FindByUID(ctx, cardUID)
}

func (s *service) RegisterOrGet(ctx context.Context, cardUID string) (*RFIDCard, error) {
	existing, err := s.repo.FindByUID(ctx, cardUID)
	if err == nil {
		return existing, nil
	}
	if !errors.IsCode(err, errors.ErrNotFound) {
		return nil, err
	}

	card := &RFIDCard{
		ID:        uuid.New(),
		CardUID:   cardUID,
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, card); err != nil {
		return nil, err
	}

	s.log.Info(ctx, "rfid card registered", "id", card.ID, "card_uid", cardUID)
	return s.repo.FindByID(ctx, card.ID)
}

func (s *service) LinkVehicle(ctx context.Context, id uuid.UUID, vehicleID uuid.UUID) (*RFIDCard, error) {
	card, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !card.IsActive {
		return nil, errors.New(errors.ErrForbidden, "cannot link vehicle to an inactive card")
	}

	card.VehicleID = &vehicleID
	if err := s.repo.Update(ctx, card); err != nil {
		return nil, err
	}

	s.log.Info(ctx, "rfid card linked to vehicle", "card_id", id, "vehicle_id", vehicleID)
	return s.repo.FindByID(ctx, id)
}

func (s *service) Deactivate(ctx context.Context, id uuid.UUID, operatorID uuid.UUID) error {
	if err := s.repo.Deactivate(ctx, id, operatorID); err != nil {
		return err
	}
	s.log.Info(ctx, "rfid card deactivated", "card_id", id, "operator_id", operatorID)
	return nil
}
