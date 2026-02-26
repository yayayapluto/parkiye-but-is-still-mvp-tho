package override

import (
	"time"

	"github.com/google/uuid"
	"parkieee/pkg/types"
)

// OperatorOverride records a manual operator action on a transaction.
// Every override requires an approver — no override without approval.
type OperatorOverride struct {
	ID            uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TransactionID uuid.UUID          `gorm:"type:uuid;not null;index"`
	OperatorID    uuid.UUID          `gorm:"type:uuid;not null;index"`
	OverrideType  types.OverrideType `gorm:"type:varchar(30);not null"` // "lost_card_exit"|"no_qr_exit"|"fee_waive"|"fee_adjust"|"force_open_gate"|"manual_entry"
	Reason        string             `gorm:"type:text"`
	OriginalFee   *int
	AdjustedFee   *int
	ApprovedBy    uuid.UUID `gorm:"type:uuid;not null"` // required — no override without approval
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

func (OperatorOverride) TableName() string { return "operator_overrides" }
