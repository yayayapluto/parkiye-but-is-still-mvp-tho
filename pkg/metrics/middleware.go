package metrics

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Middleware records HTTP metrics for every request.
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip /metrics endpoint itself to avoid noise
		if c.Path() == "/metrics/prom" {
			return c.Next()
		}

		start := time.Now()
		HTTPActiveRequests.Inc()

		err := c.Next()

		HTTPActiveRequests.Dec()
		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Response().StatusCode())
		path := c.Route().Path // use route pattern, not raw path (avoids high cardinality)

		HTTPRequestsTotal.WithLabelValues(c.Method(), path, status).Inc()
		HTTPRequestDuration.WithLabelValues(c.Method(), path).Observe(duration)

		return err
	}
}
