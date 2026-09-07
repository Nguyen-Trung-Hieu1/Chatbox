package domain

import "context"

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
	Complete(context.Context, []AIMessage) (string, error)
	Stream(context.Context, []AIMessage, func(string) error) (string, error)
}
