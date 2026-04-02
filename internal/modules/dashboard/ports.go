package dashboard

import "context"

// ServicePort mendefinisikan kontrak untuk dashboard service.
type ServicePort interface {
	GetStats(ctx context.Context) (*DashboardStats, error)
	GetUserRoleSummary(ctx context.Context) ([]UserRoleSummary, error)
}
