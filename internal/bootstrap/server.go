package bootstrap

import (
	"errors"
	"log"
	"net/http"

	"github.com/gofiber/contrib/otelfiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	recover2 "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"parkieee/internal/modules/audit"
	"parkieee/internal/modules/auth"
	"parkieee/internal/modules/dashboard"
	"parkieee/internal/modules/fee"
	"parkieee/internal/modules/gate"
	"parkieee/internal/modules/kiosk"
	"parkieee/internal/modules/notification"
	"parkieee/internal/modules/override"
	"parkieee/internal/modules/payment"
	"parkieee/internal/modules/rfid"
	"parkieee/internal/modules/transaction"
	"parkieee/internal/modules/vehicle"
	"parkieee/internal/modules/zone"
	pkgerrors "parkieee/pkg/errors"
	"parkieee/pkg/response"
)

func NewServer(container *Container) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:           container.Config.App.Name,
		ReadTimeout:       container.Config.Server.ReadTimeout,
		WriteTimeout:      container.Config.Server.WriteTimeout,
		IdleTimeout:       container.Config.Server.IdleTimeout,
		EnablePrintRoutes: container.Config.Server.PrintRoutes,
		ErrorHandler:      errorHandler,
		StrictRouting:     false,
	})

	app.Use(func(c *fiber.Ctx) error {
		log.Printf("[REQ] %s %s | Origin: %s", c.Method(), c.Path(), c.Get("Origin"))
		return c.Next()
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     container.Config.App.AllowOrigins,
		AllowMethods:     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, Cache-Control, X-Requested-With",
		ExposeHeaders:    "Content-Length",
		AllowCredentials: container.Config.App.AllowOrigins != "*",
	}))

	app.Use(recover2.New())
	app.Use(requestid.New())

	initObservability(app, container)
	app.Use(otelfiber.Middleware())
	app.Get("/metrics", monitor.New())

	if container.Config.IsProduction() {
		app.Use(etag.New())
		app.Use(limiter.New(limiter.Config{Max: 100}))
	}

	app.Use("/storage", filesystem.New(filesystem.Config{
		Root:   http.Dir("storage"),
		Browse: false,
	}))

	app.Get("/health", func(c *fiber.Ctx) error {
		return response.Success(c, "ok", fiber.Map{
			"status": "ok",
			"env":    container.Config.App.Env,
		})
	})

	api := app.Group("/api/v1")
	auth.RegisterRoutes(api, container.AuthService, container.Validator)
	zone.RegisterRoutes(api, container.ZoneService, container.AuthService, container.Validator)
	vehicle.RegisterRoutes(api, container.VehicleService, container.AuthService, container.Validator)
	rfid.RegisterRoutes(api, container.RFIDService, container.AuthService, container.Validator)
	fee.RegisterRoutes(api, container.FeeService, container.AuthService, container.Validator)
	transaction.RegisterRoutes(api, container.TransactionService, container.AuthService, container.GateService, container.Validator,
		container.Config.S3, *container.Config)
	payment.RegisterRoutes(api, container.PaymentService, container.AuthService, container.GateService,
		container.GateCashierAssignmentRepo, container.Validator, container.Config.Midtrans.Env == "sandbox")
	gate.RegisterRoutes(api, container.GateService, container.AuthService, container.Validator)
	kiosk.RegisterRoutes(api, container.FeeService, container.VehicleService, container.ZoneService, container.GateService)
	override.RegisterRoutes(api, container.OverrideService, container.AuthService, container.Validator)
	dashboard.RegisterRoutes(api, container.DashboardService, container.AuthService)
	notification.NewModule(container.DB, container.Log).RegisterRoutes(api, container.AuthService)
	audit.RegisterRoutes(api, container.AuditService, container.AuthService)

	return app
}

func errorHandler(c *fiber.Ctx, err error) error {
	var appErr *pkgerrors.AppError
	var fiberErr *fiber.Error
	if !errors.As(err, &appErr) && !errors.As(err, &fiberErr) {
		log.Printf("[ERROR] %s %s → %T: %v", c.Method(), c.Path(), err, err)
	}
	return response.ErrorHandler(c, err)
}
