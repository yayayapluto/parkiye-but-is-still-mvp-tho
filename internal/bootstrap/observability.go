package bootstrap

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/grafana/pyroscope-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"parkieee/pkg/metrics"
	"parkieee/pkg/tracer"
)

func initObservability(app *fiber.App, container *Container) {
	initTracing(container)
	initProfiling(container)
	initMetrics(app)
}

func initTracing(container *Container) {
	if !container.Config.Tracing.Enabled {
		return
	}
	_, err := tracer.New(context.Background(), container.Config.App.Name, container.Config.Tracing.Endpoint)
	if err != nil {
		container.Log.Error(context.Background(), "failed to init tracing", "error", err)
		return
	}
	container.Log.Info(context.Background(), "tracing enabled", "backend", "tempo", "endpoint", container.Config.Tracing.Endpoint)
}

func initProfiling(container *Container) {
	if !container.Config.Profiling.Enabled {
		return
	}
	_, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: container.Config.App.Name,
		ServerAddress:   container.Config.Profiling.PyroscopeEndpoint,
		Tags:            map[string]string{"env": container.Config.App.Env},
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
		},
	})
	if err != nil {
		container.Log.Error(context.Background(), "failed to init pyroscope", "error", err)
		return
	}
	container.Log.Info(context.Background(), "profiling enabled", "backend", "pyroscope", "endpoint", container.Config.Profiling.PyroscopeEndpoint)
}

func initMetrics(app *fiber.App) {
	app.Use(metrics.Middleware())
	app.Get("/metrics/prometheus", func(c *fiber.Ctx) error {
		// promhttp butuh net/http.Request, bukan fasthttp.Request.
		// Buat dummy request dengan context supaya handler bisa jalan.
		req, _ := http.NewRequestWithContext(c.Context(), http.MethodGet, "/metrics/prometheus", nil)
		promhttp.Handler().ServeHTTP(&fiberResponseWriter{c}, req)
		return nil
	})
}

type fiberResponseWriter struct {
	c *fiber.Ctx
}

func (w *fiberResponseWriter) Header() http.Header  { return http.Header{} }
func (w *fiberResponseWriter) WriteHeader(code int) { w.c.Status(code) }
func (w *fiberResponseWriter) Write(b []byte) (int, error) {
	return w.c.Response().BodyWriter().Write(b)
}
