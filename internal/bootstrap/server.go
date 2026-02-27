package bootstrap

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"parkieee/pkg/response"
)

// NewServer creates and configures the Fiber application
func NewServer(container *Container) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:           container.Config.App.Name,
		ReadTimeout:       container.Config.Server.ReadTimeout,
		WriteTimeout:      container.Config.Server.WriteTimeout,
		IdleTimeout:       container.Config.Server.IdleTimeout,
		EnablePrintRoutes: true,
		ErrorHandler:      errorHandler,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(requestid.New())
	//app.Use(container.Log.Middleware())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	initObservability(app)
	app.Get("/metrics", monitor.New())

	// Production-only middleware
	if container.Config.IsProduction() {
		app.Use(etag.New())
		app.Use(limiter.New(limiter.Config{
			Max: 100, // 100 requests per IP per minute
		}))
	}

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return response.Success(c, "ok", fiber.Map{
			"status": "ok",
			"env":    container.Config.App.Env,
		})
	})
	// API v1 routes
	//api := app.Group("/api/v1")

	// Register module routes (in dependency order)
	//auth.RegisterRoutes(api, container)
	//zone.RegisterRoutes(api, container)
	//gate.RegisterRoutes(api, container)
	//vehicle.RegisterRoutes(api, container)
	//rfid.RegisterRoutes(api, container)
	//fee.RegisterRoutes(api, container)
	//transaction.RegisterRoutes(api, container)
	//payment.RegisterRoutes(api, container)
	//override.RegisterRoutes(api, container)
	//ocr.RegisterRoutes(api, container)
	//audit.RegisterRoutes(api, container)

	return app
}

// Global error handler
func errorHandler(c *fiber.Ctx, err error) error {
	return response.ErrorHandler(c, err)
}
