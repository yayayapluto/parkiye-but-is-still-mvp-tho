package bootstrap

import (
	"gorm.io/gorm"

	"parkieee/internal/modules/auth"
	"parkieee/internal/modules/fee"
	"parkieee/internal/modules/gate"
	"parkieee/internal/modules/ocr"
	"parkieee/internal/modules/payment"
	"parkieee/internal/modules/rfid"
	"parkieee/internal/modules/transaction"
	"parkieee/internal/modules/vehicle"
	"parkieee/internal/modules/zone"
	"parkieee/pkg/config"
	"parkieee/pkg/logger"
	"parkieee/pkg/validator"
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

	VehicleTypeRepo vehicle.VehicleTypeRepositoryPort
	VehicleRepo     vehicle.VehicleRepositoryPort
	VehicleService  vehicle.ServicePort

	RFIDCardRepo rfid.RFIDCardRepositoryPort
	RFIDService  rfid.ServicePort

	FeeConfigRepo   fee.FeeConfigRepositoryPort
	FeeTierRepo     fee.FeeTierRepositoryPort
	HolidayRateRepo fee.HolidayRateRepositoryPort
	FeeService      fee.ServicePort

	TransactionRepo    transaction.TransactionRepositoryPort
	TransactionLogRepo transaction.TransactionLogRepositoryPort
	TransactionService transaction.ServicePort

	PaymentRepo    payment.RepositoryPort
	PaymentService payment.ServicePort

	OCRJobRepo       ocr.OCRJobRepositoryPort
	OCRResultRepo    ocr.OCRResultRepositoryPort
	OCRReviewLogRepo ocr.OCRReviewLogRepositoryPort
	OCRService       ocr.ServicePort

	GatePairingRepo gate.PairingRepositoryPort
	GateService     gate.ServicePort
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
	if err := container.initVehicleModule(); err != nil {
		return nil, err
	}
	if err := container.initRFIDModule(); err != nil {
		return nil, err
	}
	if err := container.initFeeModule(); err != nil {
		return nil, err
	}
	if err := container.initOCRModule(); err != nil {
		return nil, err
	}
	if err := container.initTransactionModule(); err != nil {
		return nil, err
	}
	if err := container.initPaymentModule(); err != nil {
		return nil, err
	}

	// Wire OCR → Transaction stamper post-init to avoid circular dependency.
	container.OCRService.SetTransactionStamper(container.TransactionService)
	if err := container.initGateModule(); err != nil {
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

func (c *Container) initVehicleModule() error {
	c.VehicleTypeRepo = vehicle.NewVehicleTypeRepository(c.DB)
	c.VehicleRepo = vehicle.NewVehicleRepository(c.DB)
	c.VehicleService = vehicle.NewService(c.VehicleTypeRepo, c.VehicleRepo, c.Log)
	return nil
}

func (c *Container) initRFIDModule() error {
	c.RFIDCardRepo = rfid.NewRFIDCardRepository(c.DB)
	c.RFIDService = rfid.NewService(c.RFIDCardRepo, c.Log)
	return nil
}

func (c *Container) initFeeModule() error {
	c.FeeConfigRepo = fee.NewFeeConfigRepository(c.DB)
	c.FeeTierRepo = fee.NewFeeTierRepository(c.DB)
	c.HolidayRateRepo = fee.NewHolidayRateRepository(c.DB)
	c.FeeService = fee.NewService(c.FeeConfigRepo, c.FeeTierRepo, c.HolidayRateRepo, c.Log)
	return nil
}

func (c *Container) initOCRModule() error {
	c.OCRJobRepo = ocr.NewOCRJobRepository(c.DB)
	c.OCRResultRepo = ocr.NewOCRResultRepository(c.DB)
	c.OCRReviewLogRepo = ocr.NewOCRReviewLogRepository(c.DB)
	c.OCRService = ocr.NewService(
		c.DB,
		c.OCRJobRepo,
		c.OCRResultRepo,
		c.OCRReviewLogRepo,
		c.VehicleService,
		c.ZoneRepo,
		c.Log,
		c.Config.OCR.APIURL,
		c.Config.OCR.Timeout,
		c.Config.OCR.MaxRetries,
		c.Config.OCR.Enabled,
		c.Config.OCR.AutoAcceptThreshold,
	)
	return nil
}

func (c *Container) initTransactionModule() error {
	c.TransactionRepo = transaction.NewTransactionRepository(c.DB)
	c.TransactionLogRepo = transaction.NewTransactionLogRepository(c.DB)
	c.TransactionService = transaction.NewService(
		c.DB,
		c.TransactionRepo,
		c.TransactionLogRepo,
		c.ZoneService,
		c.GateRepo,
		c.ZoneRepo,
		c.RFIDService,
		c.FeeService,
		c.VehicleService,
		c.OCRService,
		c.OCRResultRepo,
		c.Log,
		c.Config.BaseURL(),
		c.Config.App.PlaceName,
		c.Config.App.QRSecret,
	)
	return nil
}

func (c *Container) initPaymentModule() error {
	c.PaymentRepo = payment.NewRepository(c.DB)
	c.PaymentService = payment.NewService(
		c.PaymentRepo,
		c.TransactionService,
		c.Config.Midtrans,
		c.Log,
	)
	return nil
}

func (c *Container) initGateModule() error {
	c.GatePairingRepo = gate.NewPairingRepository(c.DB)
	c.GateService = gate.NewService(
		c.GateRepo,
		c.GatePairingRepo,
		c.Config,
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
