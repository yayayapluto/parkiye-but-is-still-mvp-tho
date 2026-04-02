package include

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ParseInclude parses the "include" query parameter into a map for fast lookup.
// Example: ?include=zone,vehicle -> {"zone": true, "vehicle": true}
func ParseInclude(c *fiber.Ctx) map[string]bool {
	includeStr := c.Query("include")
	if includeStr == "" {
		return nil
	}

	includes := make(map[string]bool)
	parts := strings.Split(includeStr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			includes[strings.ToLower(p)] = true
		}
	}

	return includes
}
