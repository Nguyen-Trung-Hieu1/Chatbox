package domain

import "time"

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	Email       string    `json:"email,omitempty"`
	FirstName   string    `json:"first_name,omitempty"`
	LastName    string    `json:"last_name,omitempty"`
	Password    string    `json:"-"`
	KeycloakSub string    `json:"-"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct {
	ID, UserID, TokenHash string
	ExpiresAt, CreatedAt  time.Time
}

type Conversation struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"-"`
	Title     string    `json:"title"`
	Summary   string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             string    `json:"id,omitempty"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Summarized     bool      `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIUsage struct {
	InputTokens  int  `json:"input_tokens"`
	OutputTokens int  `json:"output_tokens"`
	Reported     bool `json:"reported"`
}

type AIResult struct {
	Content string
	Usage   AIUsage
}

type ModelPolicy struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	InputRate       int64  `json:"input_rate"` // credit per 1000 tokens
	OutputRate      int64  `json:"output_rate"`
	MaxOutputTokens int    `json:"max_output_tokens"`
}

type Entitlement struct {
	Plan          string        `json:"plan"`
	PaidUntil     *time.Time    `json:"paid_until,omitempty"`
	CreditResetAt *time.Time    `json:"credit_reset_at,omitempty"`
	Balance       int64         `json:"balance"`
	Models        []ModelPolicy `json:"models"`
}

type UsageRecord struct {
	UserID       string    `json:"-"`
	Model        string    `json:"model"`
	Kind         string    `json:"kind"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Credits      int64     `json:"credits"`
	Reported     bool      `json:"reported"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type APIKey struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Hash       string     `json:"-"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}
