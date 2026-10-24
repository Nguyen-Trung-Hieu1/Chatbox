package domain

import "time"

type AdminAccount struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Plan      string     `json:"plan"`
	PaidUntil *time.Time `json:"paid_until,omitempty"`
	Balance   int64      `json:"balance"`
}

type AdminPlanChange struct {
	UserID string `json:"user_id"`
	Plan   string `json:"plan"`
}

type DailyUsage struct {
	Date      string `json:"date"`
	WebInput  int64  `json:"web_input"`
	WebOutput int64  `json:"web_output"`
	APIInput  int64  `json:"api_input"`
	APIOutput int64  `json:"api_output"`
}

type ModelUsage struct {
	ID     string `json:"id"`
	Tokens int64  `json:"tokens"`
}

type AdminUsageReport struct {
	Days        []DailyUsage `json:"days"`
	TopModel    *ModelUsage  `json:"top_model,omitempty"`
	RecordLimit int          `json:"record_limit"`
}
