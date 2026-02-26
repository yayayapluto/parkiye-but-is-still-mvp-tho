package bootstrap

import (
	"parkieee/internal/modules/audit"
	"parkieee/internal/modules/auth"
	"parkieee/internal/modules/fee"
	"parkieee/internal/modules/gate"
	"parkieee/internal/modules/ocr"
	"parkieee/internal/modules/override"
	"parkieee/internal/modules/payment"
	"parkieee/internal/modules/rfid"
	"parkieee/internal/modules/transaction"
	"parkieee/internal/modules/vehicle"
	"parkieee/internal/modules/zone"
	"parkieee/pkg/config"
	"parkieee/pkg/logger"
	"parkieee/pkg/validator"

	"gorm.io/gorm"
)

// Container holds all application dependencies
type Container struct {
	// Core
	DB        *gorm.DB
	Log       logger.Logger
	Config    *config.Config
	Validator *validator.Validator

	// Auth Module
	AuthRepo    auth.RepositoryPort
	UserRepo    auth.UserRepositoryPort
	AuthService auth.ServicePort

	// Zone Module
	ZoneRepo    zone.RepositoryPort
	ZoneService zone.ServicePort

	// Gate Module
	GateRepo       gate.RepositoryPort
	GateDeviceRepo gate.DeviceRepositoryPort
	GateService    gate.ServicePort

	// Vehicle Module
	VehicleTypeRepo vehicle.TypeRepositoryPort
	VehicleRepo     vehicle.RepositoryPort
	VehicleService  vehicle.ServicePort

	// RFID Module
	RFIDCardRepo    rfid.CardRepositoryPort
	RFIDCardService rfid.CardServicePort

	// Fee Module
	FeeConfigRepo   fee.ConfigRepositoryPort
	FeeTierRepo     fee.TierRepositoryPort
	HolidayRateRepo fee.HolidayRateRepositoryPort
	FeeService      fee.ServicePort

	// Transaction Module
	TransactionRepo         transaction.RepositoryPort
	TransactionLogRepo      transaction.LogRepositoryPort
	UnclosedTransactionRepo transaction.UnclosedRepositoryPort
	TransactionService      transaction.ServicePort

	// Payment Module
	PaymentRepo        payment.RepositoryPort
	MidtransCallbackRepo payment.MidtransCallbackRepositoryPort
	RefundRepo         payment.RefundRepositoryPort
	PaymentService     payment.ServicePort

	// Override Module
	OverrideConfigRepo override.ConfigRepositoryPort
	OperatorOverrideRepo override.OperatorOverrideRepositoryPort
	OverrideService   override.ServicePort

	// OCR Module
	OCRConfigRepo    ocr.ConfigRepositoryPort
	OCRJobRepo       ocr.JobRepositoryPort
	OCRResultRepo    ocr.ResultRepositoryPort
	OCRReviewLogRepo ocr.ReviewLogRepositoryPort
	OCRService       ocr.ServicePort

	// Audit Module
	AuditLogRepo     audit.LogRepositoryPort
	AuditLogExportRepo audit.ExportRepositoryPort
	AuditService     audit.ServicePort
}

// NewContainer initializes all dependencies
func NewContainer(cfg *config.Config, log logger.Logger) (*Container, error) {
	// Initialize database connection
	db, err := initDatabase(cfg)
	if err != nil {
		return nil, err
	}

	// Initialize validator
	v := validator.New()
	if cfg.IsDevelopment() {
		// In development, include more detailed validation errors
		v = v.WithTranslator(validator.DevelopmentTranslator)
	}

	// Initialize repositories and services
	container := &Container{
		DB:        db,
		Log:       log,
		Config:    cfg,
		Validator: v,
	}

	// Initialize all modules
	if err := container.initAuthModule(); err != nil {
		return nil, err
	}
	if err := container.initZoneModule(); err != nil {
		return nil, err
	}
	if err := container.initGateModule(); err != nil {
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
	if err := container.initTransactionModule(); err != nil {
		return nil, err
	}
	if err := container.initPaymentModule(); err != nil {
		return nil, err
	}
	if err := container.initOverrideModule(); err != nil {
		return nil, err
	}
	if err := container.initOCRModule(); err != nil {
		return nil, err
	}
	if err := container.initAuditModule(); err != nil {
		return nil, err
	}

	return container, nil
}

func (c *Container) initAuthModule() error {
	// Initialize repositories
	c.AuthRepo = auth.NewRepository(c.DB)
	c.UserRepo = auth.NewUserRepository(c.DB)

	// Initialize service
	c.AuthService = auth.NewService(
		c.AuthRepo,
		c.UserRepo,
		c.Log,
		c.Config,
		c.Validator,
	)

	return nil
}

func (c *Container) initZoneModule() error {
	c.ZoneRepo = zone.NewRepository(c.DB)
	c.ZoneService = zone.NewService(
		c.ZoneRepo,
		c.Log,
		c.Validator,
	)
	return nil
}

func (c *Container) initGateModule() error {
	c.GateRepo = gate.NewRepository(c.DB)
	c.GateDeviceRepo = gate.NewDeviceRepository(c.DB)
	c.GateService = gate.NewService(
		c.GateRepo,
		c.GateDeviceRepo,
		c.ZoneRepo, // Depends on zone
		c.Log,
		c.Validator,
	)
	return nil
}

func (c *Container) initVehicleModule() error {
	c.VehicleTypeRepo = vehicle.NewTypeRepository(c.DB)
	c.VehicleRepo = vehicle.NewRepository(c.DB)
	c.VehicleService = vehicle.NewService(
		c.VehicleTypeRepo,
		c.VehicleRepo,
		c.Log,
		c.Validator,
	)
	return nil
}

func (c *Container) initRFIDModule() error {
	c.RFIDCardRepo = rfid.NewCardRepository(c.DB)
	c.RFIDCardService = rfid.NewCardService(
		c.RFIDCardRepo,
		c.Log,
		c.Validator,
	)
	return nil
}

func (c *Container) initFeeModule() error {
	c.FeeConfigRepo = fee.NewConfigRepository(c.DB)
	c.FeeTierRepo = fee.NewTierRepository(c.DB)
	c.HolidayRateRepo = fee.NewHolidayRateRepository(c.DB)
	c.FeeService = fee.NewService(
		c.FeeConfigRepo,
		c.FeeTierRepo,
		c.HolidayRateRepo,
		c.ZoneRepo,      // Depends on zone
		c.VehicleTypeRepo, // Depends on vehicle type
		c.Log,
		c.Validator,
	)
	return nil
}

func (c *Container) initTransactionModule() error {
	c.TransactionRepo = transaction.NewRepository(c.DB)
	c.TransactionLogRepo = transaction.NewLogRepository(c.DB)
	c.UnclosedTransactionRepo = transaction.NewUnclosedRepository(c.DB)
	c.TransactionService = transaction.NewService(
		c.TransactionRepo,
		c.TransactionLogRepo,
		c.UnclosedTransactionRepo,
		c.VehicleRepo,     // Depends on vehicle
		c.RFIDCardRepo,    // Depends on RFID
		c.FeeService,      // Depends on fee calculation
		c.Log,
		c.Validator,
	)
	return nil
}

func (c *Container) initPaymentModule() error {
	c.PaymentRepo = payment.NewRepository(c.DB)
	c.MidtransCallbackRepo = payment.NewMidtransCallbackRepository(c.DB)
	c.RefundRepo = payment.NewRefundRepository(c.DB)
	c.PaymentService = payment.NewService(
		c.PaymentRepo,
		c.MidtransCallbackRepo,
		c.RefundRepo,
		c.TransactionRepo, // Depends on transaction
		c.Log,
		c.Config, // For Midtrans config
		c.Validator,
	)
	return nil
}

func (c *Container) initOverrideModule() error {
	c.OverrideConfigRepo = override.NewConfigRepository(c.DB)
	c.OperatorOverrideRepo = override.NewOperatorOverrideRepository(c.DB)
	c.OverrideService = override.NewService(
		c.OverrideConfigRepo,
		c.OperatorOverrideRepo,
		c.GateRepo,        // Depends on gate
		c.TransactionRepo, // Depends on transaction
		c.UserRepo,        // Depends on user (operator)
		c.Log,
		c.Validator,
	)
	return nil
}

func (c *Container) initOCRModule() error {
	c.OCRConfigRepo = ocr.NewConfigRepository(c.DB)
	c.OCRJobRepo = ocr.NewJobRepository(c.DB)
	c.OCRResultRepo = ocr.NewResultRepository(c.DB)
	c.OCRReviewLogRepo = ocr.NewReviewLogRepository(c.DB)
	c.OCRService = ocr.NewService(
		c.OCRConfigRepo,
		c.OCRJobRepo,
		c.OCRResultRepo,
		c.OCRReviewLogRepo,
		c.TransactionRepo, // Depends on transaction
		c.Log,
		c.Validator,
	)
	return nil
}

func (c *Container) initAuditModule() error {
	c.AuditLogRepo = audit.NewLogRepository(c.DB)
	c.AuditLogExportRepo = audit.NewExportRepository(c.DB)
	c.AuditService = audit.NewService(
		c.AuditLogRepo,
		c.AuditLogExportRepo,
		c.UserRepo, // Depends on user
		c.Log,
		c.Validator,
	)
	return nil
}

// Close gracefully shuts down all resources
func (c *Container) Close() error {
	sqlDB, err := c.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}