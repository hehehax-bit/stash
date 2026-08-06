package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiSuggestionStore struct{}

func NewAISuggestionStore() *aiSuggestionStore {
	return &aiSuggestionStore{}
}

func (s *aiSuggestionStore) Create(ctx context.Context, suggestion *models.AISuggestion) error {
	performers, err := json.Marshal(suggestion.Performers)
	if err != nil {
		return err
	}
	tags, err := json.Marshal(suggestion.Tags)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	suggestion.CreatedAt = now
	suggestion.UpdatedAt = now
	if suggestion.Status == "" {
		suggestion.Status = models.SuggestionStatusPending
	}

	_, err = dbWrapper.Exec(ctx,
		`INSERT INTO ai_suggestions (entity_type, entity_id, title, details, performers, tags, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		suggestion.EntityType, suggestion.EntityID, suggestion.Title, suggestion.Details,
		string(performers), string(tags), suggestion.Status, now, now,
	)
	return err
}

func (s *aiSuggestionStore) scanRows(rows *sqlx.Rows) ([]*models.AISuggestion, error) {
	var suggestions []*models.AISuggestion
	for rows.Next() {
		var sg models.AISuggestion
		var performers, tags string
		if err := rows.Scan(&sg.ID, &sg.EntityType, &sg.EntityID, &sg.Title, &sg.Details, &performers, &tags, &sg.Status, &sg.CreatedAt, &sg.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(performers), &sg.Performers)
		_ = json.Unmarshal([]byte(tags), &sg.Tags)
		suggestions = append(suggestions, &sg)
	}
	return suggestions, nil
}

func (s *aiSuggestionStore) FindByID(ctx context.Context, id int64) (*models.AISuggestion, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, entity_type, entity_id, title, details, performers, tags, status, created_at, updated_at FROM ai_suggestions WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	suggestions, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(suggestions) == 0 {
		return nil, nil
	}
	return suggestions[0], nil
}

func (s *aiSuggestionStore) FindByStatus(ctx context.Context, status string) ([]*models.AISuggestion, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, entity_type, entity_id, title, details, performers, tags, status, created_at, updated_at FROM ai_suggestions WHERE status = ? ORDER BY created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *aiSuggestionStore) FindPendingByEntityType(ctx context.Context, entityType string) ([]*models.AISuggestion, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, entity_type, entity_id, title, details, performers, tags, status, created_at, updated_at FROM ai_suggestions WHERE entity_type = ? AND status = ? ORDER BY created_at DESC`, entityType, models.SuggestionStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *aiSuggestionStore) FindByEntity(ctx context.Context, entityType string, entityID int) ([]*models.AISuggestion, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, entity_type, entity_id, title, details, performers, tags, status, created_at, updated_at FROM ai_suggestions WHERE entity_type = ? AND entity_id = ? ORDER BY created_at DESC`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *aiSuggestionStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().Unix()
	_, err := dbWrapper.Exec(ctx,
		`UPDATE ai_suggestions SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id,
	)
	return err
}

func (s *aiSuggestionStore) Delete(ctx context.Context, id int64) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_suggestions WHERE id = ?`, id)
	return err
}
