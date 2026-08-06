package sqlite

import (
	"context"
	"crypto/rand"
	hexutil "encoding/hex"
	"fmt"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

type aiChatStore struct{}

func NewAIChatStore() *aiChatStore {
	return &aiChatStore{}
}

func (s *aiChatStore) CreateSession(ctx context.Context, session *models.AIChatSession) error {
	if session.ID == "" {
		id, err := generateID()
		if err != nil {
			return err
		}
		session.ID = id
	}

	session.CreatedAt = time.Now().Unix()
	session.UpdatedAt = session.CreatedAt

	_, err := dbWrapper.Exec(ctx,
		`INSERT INTO ai_chat_sessions (id, created_at, updated_at, title) VALUES (?, ?, ?, ?)`,
		session.ID, session.CreatedAt, session.UpdatedAt, session.Title,
	)
	return err
}

func (s *aiChatStore) Touch(ctx context.Context, sessionID string) error {
	now := time.Now().Unix()
	_, err := dbWrapper.Exec(ctx,
		`UPDATE ai_chat_sessions SET updated_at = ? WHERE id = ?`,
		now, sessionID,
	)
	return err
}

func (s *aiChatStore) SetTitle(ctx context.Context, sessionID string, title string) error {
	_, err := dbWrapper.Exec(ctx,
		`UPDATE ai_chat_sessions SET title = ? WHERE id = ?`,
		title, sessionID,
	)
	return err
}

func (s *aiChatStore) FindAll(ctx context.Context) ([]*models.AIChatSession, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, created_at, updated_at, title FROM ai_chat_sessions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.AIChatSession
	for rows.Next() {
		var sess models.AIChatSession
		if err := rows.Scan(&sess.ID, &sess.CreatedAt, &sess.UpdatedAt, &sess.Title); err != nil {
			return nil, err
		}
		sessions = append(sessions, &sess)
	}

	return sessions, rows.Err()
}

func (s *aiChatStore) CreateMessage(ctx context.Context, msg *models.AIChatMessage) error {
	if msg.ID == "" {
		id, err := generateID()
		if err != nil {
			return err
		}
		msg.ID = id
	}

	msg.CreatedAt = time.Now().Unix()

	_, err := dbWrapper.Exec(ctx,
		`INSERT INTO ai_chat_messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.CreatedAt,
	)
	return err
}

func (s *aiChatStore) FindBySessionID(ctx context.Context, sessionID string) ([]*models.AIChatMessage, error) {
	rows, err := dbWrapper.Queryx(ctx,
		`SELECT id, session_id, role, content, created_at FROM ai_chat_messages WHERE session_id = ? ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.AIChatMessage
	for rows.Next() {
		var msg models.AIChatMessage
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}

	return messages, rows.Err()
}

func (s *aiChatStore) DeleteByID(ctx context.Context, messageID string) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_chat_messages WHERE id = ?`, messageID)
	return err
}

func (s *aiChatStore) DeleteBySessionID(ctx context.Context, sessionID string) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_chat_messages WHERE session_id = ?`, sessionID)
	if err != nil {
		return err
	}

	_, err = dbWrapper.Exec(ctx, `DELETE FROM ai_chat_sessions WHERE id = ?`, sessionID)
	return err
}

func generateID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("generating id: %w", err)
	}
	return hexutil.EncodeToString(b), nil
}
