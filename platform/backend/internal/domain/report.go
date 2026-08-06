package domain

import (
	"encoding/json"
	"time"
)

// Report is a saved AI-generated operations report (today or day-comparison).
// Reports are permanent until manually deleted. Numbers holds the compact numeric
// payload gathered from telemetry; AiReport holds the structured AI output.
type Report struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	TenantID uint `gorm:"not null;uniqueIndex:idx_report_key" json:"tenant_id"`

	// "today" or "compare"
	Type string `gorm:"type:varchar(10);not null;uniqueIndex:idx_report_key" json:"type"`

	// YYYY-MM-DD (UTC). DateB is empty for "today" reports.
	DateA string `gorm:"type:varchar(10);not null;uniqueIndex:idx_report_key" json:"date_a"`
	DateB string `gorm:"type:varchar(10);not null;uniqueIndex:idx_report_key" json:"date_b"`

	// "generating" | "ready" | "error"
	Status string `gorm:"type:varchar(20);not null;default:'generating'" json:"status"`
	Error  string `gorm:"type:text" json:"error"`

	GeneratedBy uint `gorm:"not null" json:"generated_by"` // user id

	// JSONB payloads. json.RawMessage keeps them opaque; nil/null is valid JSONB.
	Numbers  json.RawMessage `gorm:"type:jsonb" json:"numbers"`
	AiReport json.RawMessage `gorm:"type:jsonb" json:"ai_report"`

	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	FinishedAt *time.Time `json:"finished_at"`
}
