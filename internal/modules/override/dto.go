package override

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

type OverrideResponse struct {
	ID            uuid.UUID          `json:"id"`
	TransactionID uuid.UUID          `json:"transaction_id"`
	OperatorID    uuid.UUID          `json:"operator_id"`
	OverrideType  types.OverrideType `json:"override_type"`
	Reason        string             `json:"reason"`
	OriginalFee   *int               `json:"original_fee"`
	AdjustedFee   *int               `json:"adjusted_fee"`
	ApprovedBy    uuid.UUID          `json:"approved_by"`
	CreatedAt     time.Time          `json:"created_at"`
}

func toResponse(o *OperatorOverride) OverrideResponse {
	return OverrideResponse{
		ID:            o.ID,
		TransactionID: o.TransactionID,
		OperatorID:    o.OperatorID,
		OverrideType:  o.OverrideType,
		Reason:        o.Reason,
		OriginalFee:   o.OriginalFee,
		AdjustedFee:   o.AdjustedFee,
		ApprovedBy:    o.ApprovedBy,
		CreatedAt:     o.CreatedAt,
	}
}
