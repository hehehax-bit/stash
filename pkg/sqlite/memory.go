package sqlite

import (
	"context"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

type memoryStore struct{}

func NewMemoryStore() *memoryStore {
	return &memoryStore{}
}

func (s *memoryStore) FindAll(ctx context.Context) ([]*models.AIMemory, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, key, value, created_at, updated_at FROM ai_memories ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*models.AIMemory
	for rows.Next() {
		var m models.AIMemory
		if err := rows.Scan(&m.ID, &m.Key, &m.Value, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, &m)
	}
	return memories, rows.Err()
}

func (s *memoryStore) FindByKey(ctx context.Context, key string) (*models.AIMemory, error) {
	var m models.AIMemory
	err := dbWrapper.Get(ctx, &m, `SELECT id, key, value, created_at, updated_at FROM ai_memories WHERE key = ?`, key)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *memoryStore) Set(ctx context.Context, key string, value string) error {
	now := time.Now().Unix()
	_, err := dbWrapper.Exec(ctx,
		`INSERT INTO ai_memories (key, value, created_at, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now, now,
	)
	return err
}

func (s *memoryStore) Delete(ctx context.Context, key string) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_memories WHERE key = ?`, key)
	return err
}

func (s *memoryStore) DeleteAll(ctx context.Context) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_memories`)
	return err
}
