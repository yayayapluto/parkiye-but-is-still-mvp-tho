package bootstrap

import (
	"parkieee/internal/modules/auth"
	"parkieee/internal/modules/zone"
	"parkieee/pkg/config"
	"parkieee/pkg/logger"
	"parkieee/pkg/validator"

	"gorm.io/gorm"
)

type Container struct {
	DB        *gorm.DB
	Log       logger.Logger
	Config    *config.Config
	Validator *validator.Validator

	AuthUserRepo       auth.UserRepositoryPort
	AuthRoleRepo       auth.RoleRepositoryPort
	AuthPermRepo       auth.PermissionRepositoryPort
	AuthRolePermRepo   auth.RolePermissionRepositoryPort
	AuthSessionRepo    auth.SessionRepositoryPort
	AuthLoginLogRepo   auth.LoginLogRepositoryPort
	AuthLoginStatsRepo auth.LoginStatsRepositoryPort
	AuthService        auth.ServicePort

	ZoneRepo        zone.ZoneRepositoryPort
	GateRepo        zone.GateRepositoryPort
	GateDeviceRepo  zone.GateDeviceRepositoryPort
	CapacityLogRepo zone.CapacityLogRepositoryPort
	ZoneService     zone.ServicePort
}

func NewContainer(cfg *config.Config, log logger.Logger) (*Container, error) {
	db, err := initDatabase(cfg)
	if err != nil {
		return nil, err
	}

	container := &Container{
		DB:        db,
		Log:       log,
		Config:    cfg,
		Validator: validator.New(),
	}

	if err := container.initAuthModule(); err != nil {
		return nil, err
	}

	if err := container.initZoneModule(); err != nil {
		return nil, err
	}

	return container, nil
}

func (c *Container) initAuthModule() error {
	c.AuthUserRepo = auth.NewUserRepository(c.DB)
	c.AuthRoleRepo = auth.NewRoleRepository(c.DB)
	c.AuthPermRepo = auth.NewPermissionRepository(c.DB)
	c.AuthRolePermRepo = auth.NewRolePermissionRepository(c.DB)
	c.AuthSessionRepo = auth.NewSessionRepository(c.DB)
	c.AuthLoginLogRepo = auth.NewLoginLogRepository(c.DB)
	c.AuthLoginStatsRepo = auth.NewLoginStatsRepository(c.DB)
	c.AuthService = auth.NewService(
		c.AuthUserRepo,
		c.AuthRoleRepo,
		c.AuthPermRepo,
		c.AuthRolePermRepo,
		c.AuthSessionRepo,
		c.AuthLoginLogRepo,
		c.AuthLoginStatsRepo,
		c.Config,
		c.Log,
	)
	return nil
}

func (c *Container) initZoneModule() error {
	c.ZoneRepo = zone.NewZoneRepository(c.DB)
	c.GateRepo = zone.NewGateRepository(c.DB)
	c.GateDeviceRepo = zone.NewGateDeviceRepository(c.DB)
	c.CapacityLogRepo = zone.NewCapacityLogRepository(c.DB)
	c.ZoneService = zone.NewService(
		c.ZoneRepo,
		c.GateRepo,
		c.CapacityLogRepo,
		c.DB,
		c.Log,
	)
	return nil
}

func (c *Container) Close() error {
	sqlDB, err := c.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
