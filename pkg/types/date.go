package types

import (
	"fmt"
	"strings"
	"time"
)

// DateOnly wraps time.Time to marshal/unmarshal JSON as "YYYY-MM-DD" instead of RFC3339.
// Stored as midnight UTC internally.
type DateOnly struct {
	time.Time
}

func (d *DateOnly) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("date must be in YYYY-MM-DD format, got: %s", s)
	}
	d.Time = t
	return nil
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Format("2006-01-02") + `"`), nil
}
