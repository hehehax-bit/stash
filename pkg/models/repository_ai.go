package models

import "context"

type AIChatSession struct {
	ID        string  `json:"id"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
	Title     *string `json:"title"`
}

type AIChatMessage struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
}

type AIChatSessionReader interface {
	FindAll(ctx context.Context) ([]*AIChatSession, error)
}

type AIChatSessionWriter interface {
	CreateSession(ctx context.Context, session *AIChatSession) error
	Touch(ctx context.Context, sessionID string) error
	SetTitle(ctx context.Context, sessionID string, title string) error
}

type AIChatMessageReader interface {
	FindBySessionID(ctx context.Context, sessionID string) ([]*AIChatMessage, error)
}

type AIChatMessageWriter interface {
	CreateMessage(ctx context.Context, msg *AIChatMessage) error
	DeleteBySessionID(ctx context.Context, sessionID string) error
	DeleteByID(ctx context.Context, messageID string) error
}

type AIChatReaderWriter interface {
	AIChatSessionReader
	AIChatSessionWriter
	AIChatMessageReader
	AIChatMessageWriter
}
