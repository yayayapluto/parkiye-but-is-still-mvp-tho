package bootstrap

import (
	"github.com/gofiber/contrib/otelfiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"parkieee/internal/modules/auth"
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
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	initObservability(app, container)
	app.Use(otelfiber.Middleware())
	app.Get("/metrics", monitor.New())

	if container.Config.IsProduction() {
		app.Use(etag.New())
		app.Use(limiter.New(limiter.Config{
			Max: 100,
		}))
	}

	app.Get("/health", func(c *fiber.Ctx) error {
		return response.Success(c, "ok", fiber.Map{
			"status": "ok",
			"env":    container.Config.App.Env,
		})
	})

	api := app.Group("/api/v1")
	auth.RegisterRoutes(api, container.AuthService, container.Validator)

	return app
}

func errorHandler(c *fiber.Ctx, err error) error {
	return response.ErrorHandler(c, err)
}
