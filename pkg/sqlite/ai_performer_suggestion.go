package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiPerformerSuggestionStore struct{}

func NewAIPerformerSuggestionStore() *aiPerformerSuggestionStore {
	return &aiPerformerSuggestionStore{}
}

const aiPerformerSuggestionColumns = "id, source_performer_id, target_performer_id, confidence, status, created_at, updated_at"

func (s *aiPerformerSuggestionStore) FindByID(ctx context.Context, id int64) (*models.AIPerformerSuggestion, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiPerformerSuggestionColumns+` FROM ai_performer_suggestions WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out[0], nil
}

func (s *aiPerformerSuggestionStore) FindByStatus(ctx context.Context, status string) ([]*models.AIPerformerSuggestion, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiPerformerSuggestionColumns+` FROM ai_performer_suggestions WHERE status = ? ORDER BY confidence DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRows(rows)
}

func (s *aiPerformerSuggestionStore) FindPair(ctx context.Context, sourceID, targetID int) (*models.AIPerformerSuggestion, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiPerformerSuggestionColumns+` FROM ai_performer_suggestions WHERE source_performer_id = ? AND target_performer_id = ?`, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out[0], nil
}

func (s *aiPerformerSuggestionStore) Create(ctx context.Context, suggestion *models.AIPerformerSuggestion) error {
	now := time.Now().Unix()
	suggestion.CreatedAt = now
	suggestion.UpdatedAt = now
	if suggestion.Status == "" {
		suggestion.Status = models.SuggestionStatusPending
	}

	_, err := dbWrapper.Exec(ctx, `
		INSERT INTO ai_performer_suggestions (source_performer_id, target_performer_id, confidence, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_performer_id, target_performer_id) DO UPDATE SET
			confidence = excluded.confidence,
			updated_at = excluded.updated_at
	`, suggestion.SourcePerformerID, suggestion.TargetPerformerID, suggestion.Confidence,
		suggestion.Status, suggestion.CreatedAt, suggestion.UpdatedAt)
	if err != nil {
		return err
	}

	return dbWrapper.Get(ctx, &suggestion.ID, `SELECT id FROM ai_performer_suggestions WHERE source_performer_id = ? AND target_performer_id = ?`, suggestion.SourcePerformerID, suggestion.TargetPerformerID)
}

func (s *aiPerformerSuggestionStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().Unix()
	_, err := dbWrapper.Exec(ctx, `UPDATE ai_performer_suggestions SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	return err
}

func (s *aiPerformerSuggestionStore) UpdateStatusByStatus(ctx context.Context, fromStatus, toStatus string) (int64, error) {
	now := time.Now().Unix()
	res, err := dbWrapper.Exec(ctx, `UPDATE ai_performer_suggestions SET status = ?, updated_at = ? WHERE status = ?`, toStatus, now, fromStatus)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *aiPerformerSuggestionStore) DeleteByPerformerID(ctx context.Context, performerID int) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_performer_suggestions WHERE source_performer_id = ? OR target_performer_id = ?`, performerID, performerID)
	return err
}

func (s *aiPerformerSuggestionStore) scanRows(rows *sqlx.Rows) ([]*models.AIPerformerSuggestion, error) {
	var out []*models.AIPerformerSuggestion
	for rows.Next() {
		var a models.AIPerformerSuggestion
		if err := rows.Scan(&a.ID, &a.SourcePerformerID, &a.TargetPerformerID, &a.Confidence, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}
