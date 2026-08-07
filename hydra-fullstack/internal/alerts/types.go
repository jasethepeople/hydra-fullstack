package alerts

import "time"

// Severity levels
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
	SeverityResolved Severity = "RESOLVED"
)

// Alert category
type Category string

const (
	CategorySystem     Category = "System"
	CategoryPerformance Category = "Performance"
	CategoryError      Category = "Error"
	CategorySecurity   Category = "Security"
)

// Alert represents a single alert instance
type Alert struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Severity     Severity  `json:"severity"`
	Category     Category  `json:"category"`
	Message      string    `json:"message"`
	Value        float64   `json:"value,omitempty"`
	Threshold    float64   `json:"threshold,omitempty"`
	Unit         string    `json:"unit,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
	AckedBy      string    `json:"acked_by,omitempty"`
	AckedAt      time.Time `json:"acked_at,omitempty"`
}
