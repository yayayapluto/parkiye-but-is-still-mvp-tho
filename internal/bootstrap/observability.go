package bootstrap

import (
	"context"
	"fmt"
	//"github.com/gofiber/fiber/v2/middleware/adaptor"
	"parkieee/pkg/metrics"
	"parkieee/pkg/tracer"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// initObservability registers /metrics/prometheus, attaches metrics middleware,
// and optionally starts the OpenTelemetry tracer.
// Returns a shutdown function — call it on app exit.
func initObservability(app *fiber.App, container *Container) func(context.Context) {
	// Prometheus
	app.Get("/metrics/prometheus", adaptor.HTTPHandler(promhttp.Handler()))
	app.Use(metrics.Middleware())

	// Tracing (only when ENABLE_TRACING=true)
	if !container.Config.IsTracingEnabled() {
		return func(context.Context) {}
	}

	tempoEndpoint := fmt.Sprintf("localhost:%d", container.Config.Observability.TempoPort)
	tp, err := tracer.New(context.Background(), container.Config.App.Name, tempoEndpoint)
	if err != nil {
		container.Log.Error(context.Background(), "tracer init failed", "error", err)
		return func(context.Context) {}
	}

	container.Log.Info(context.Background(), "tracing enabled", "backend", "tempo", "endpoint", tempoEndpoint)
	return func(ctx context.Context) {
		if err := tp.Shutdown(ctx); err != nil {
			container.Log.Error(ctx, "tracer shutdown failed", "error", err)
		}
	}
}
