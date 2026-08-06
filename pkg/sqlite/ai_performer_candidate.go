package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiPerformerCandidateStore struct{}

func NewAIPerformerCandidateStore() *aiPerformerCandidateStore {
	return &aiPerformerCandidateStore{}
}

const aiPerformerCandidateColumns = "id, name, description, member_count, entity_type, entity_id, member_ids, status, created_at, updated_at"

func (s *aiPerformerCandidateStore) FindByID(ctx context.Context, id int64) (*models.AIPerformerCandidate, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiPerformerCandidateColumns+` FROM ai_performer_candidates WHERE id = ?`, id)
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

func (s *aiPerformerCandidateStore) FindByStatus(ctx context.Context, status string) ([]*models.AIPerformerCandidate, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiPerformerCandidateColumns+` FROM ai_performer_candidates WHERE status = ? ORDER BY member_count DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRows(rows)
}

func (s *aiPerformerCandidateStore) Create(ctx context.Context, candidate *models.AIPerformerCandidate) error {
	now := time.Now().Unix()
	candidate.CreatedAt = now
	candidate.UpdatedAt = now
	if candidate.Status == "" {
		candidate.Status = models.SuggestionStatusPending
	}

	memberIDs, err := json.Marshal(candidate.MemberIDs)
	if err != nil {
		return err
	}

	_, err = dbWrapper.Exec(ctx, `
		INSERT INTO ai_performer_candidates (name, description, member_count, entity_type, entity_id, member_ids, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, candidate.Name, candidate.Description, candidate.MemberCount, candidate.EntityType,
		candidate.EntityID, string(memberIDs), candidate.Status, candidate.CreatedAt, candidate.UpdatedAt)
	if err != nil {
		return err
	}

	return dbWrapper.Get(ctx, &candidate.ID, `SELECT id FROM ai_performer_candidates WHERE rowid = last_insert_rowid()`)
}

func (s *aiPerformerCandidateStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().Unix()
	_, err := dbWrapper.Exec(ctx, `UPDATE ai_performer_candidates SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	return err
}

func (s *aiPerformerCandidateStore) UpdateStatusByStatus(ctx context.Context, fromStatus, toStatus string) (int64, error) {
	now := time.Now().Unix()
	res, err := dbWrapper.Exec(ctx, `UPDATE ai_performer_candidates SET status = ?, updated_at = ? WHERE status = ?`, toStatus, now, fromStatus)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *aiPerformerCandidateStore) DeleteByID(ctx context.Context, id int64) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_performer_candidates WHERE id = ?`, id)
	return err
}

func (s *aiPerformerCandidateStore) scanRows(rows *sqlx.Rows) ([]*models.AIPerformerCandidate, error) {
	var out []*models.AIPerformerCandidate
	for rows.Next() {
		var c models.AIPerformerCandidate
		var memberIDs string
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.MemberCount, &c.EntityType, &c.EntityID, &memberIDs, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(memberIDs), &c.MemberIDs)
		out = append(out, &c)
	}
	return out, rows.Err()
}
