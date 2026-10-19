package domain

import (
	"context"
	"time"
)

type UserRepository interface {
	Create(context.Context, *User) error
	ByUsername(context.Context, string) (*User, error)
	ByID(context.Context, string) (*User, error)
}

type SessionRepository interface {
	Create(context.Context, *Session) error
	ActiveByHash(context.Context, string) (*Session, error)
	DeleteByHash(context.Context, string) error
}

type ConversationRepository interface {
	Create(context.Context, *Conversation) error
	OwnedByID(context.Context, string, string) (*Conversation, error)
	ListOwned(context.Context, string) ([]Conversation, error)
	UpdateActivity(context.Context, string, string) error
	UpdateSummary(context.Context, string, string) error
	Rename(context.Context, string, string) error
	Delete(context.Context, string) error
}

type MessageRepository interface {
	Create(context.Context, *Message) error
	List(context.Context, string) ([]Message, error)
	Unsummarized(context.Context, string) ([]Message, error)
	MarkSummarized(context.Context, []string) error
	DeleteByConversation(context.Context, string) error
}

type AIClient interface {
	Complete(context.Context, string, []AIMessage, int) (AIResult, error)
	Stream(context.Context, string, []AIMessage, int, func(string) error) (AIResult, error)
}

type BillingRepository interface {
	Ensure(context.Context, string, int64) error
	Balance(context.Context, string) (int64, error)
	Plan(context.Context, string) (string, *time.Time, error)
	ActivatePaid(context.Context, string, time.Time, time.Time) error
	Reserve(context.Context, string, int64) error
	Settle(context.Context, string, int64, int64, UsageRecord) error
	History(context.Context, string) ([]UsageRecord, error)
}

type APIKeyRepository interface {
	Create(context.Context, *APIKey) error
	List(context.Context, string) ([]APIKey, error)
	ActiveByHash(context.Context, string) (*APIKey, error)
	Revoke(context.Context, string, string) error
	Touch(context.Context, string) error
}
