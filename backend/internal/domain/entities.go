package domain

import "time"

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
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
