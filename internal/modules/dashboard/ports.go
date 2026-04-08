package dashboard

import "context"

// ServicePort mendefinisikan kontrak untuk dashboard service.
type ServicePort interface {
	GetStats(ctx context.Context) (*DashboardStats, error)
	GetUserRoleSummary(ctx context.Context) ([]UserRoleSummary, error)

	GetOperatorDashboard(ctx context.Context) (*OperatorDashboardData, error)
	GetOwnerDashboard(ctx context.Context) (*OwnerDashboardData, error)
	GetAdminDashboard(ctx context.Context) (*AdminDashboardData, error)
	GetEngineerDashboard(ctx context.Context) (*EngineerDashboardData, error)
	GetCashierDashboard(ctx context.Context) (*CashierDashboardData, error)
}
