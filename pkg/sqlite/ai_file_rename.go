package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiFileRenameStore struct{}

func NewAIFileRenameStore() *aiFileRenameStore {
	return &aiFileRenameStore{}
}

const aiFileRenameColumns = "id, entity_type, entity_id, current_name, suggested_name, status, created_at, updated_at"

func (s *aiFileRenameStore) FindByID(ctx context.Context, id int64) (*models.AIFileRename, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiFileRenameColumns+` FROM ai_file_rename WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	renames, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(renames) == 0 {
		return nil, nil
	}
	return renames[0], nil
}

func (s *aiFileRenameStore) FindByStatus(ctx context.Context, status string) ([]*models.AIFileRename, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiFileRenameColumns+` FROM ai_file_rename WHERE status = ? ORDER BY created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *aiFileRenameStore) FindPendingByEntityType(ctx context.Context, entityType string) ([]*models.AIFileRename, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiFileRenameColumns+` FROM ai_file_rename WHERE entity_type = ? AND status = ? ORDER BY created_at DESC`, entityType, models.FileRenameStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanRows(rows)
}

func (s *aiFileRenameStore) FindByEntity(ctx context.Context, entityType string, entityID int) (*models.AIFileRename, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiFileRenameColumns+` FROM ai_file_rename WHERE entity_type = ? AND entity_id = ?`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	renames, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(renames) == 0 {
		return nil, nil
	}
	return renames[0], nil
}

func (s *aiFileRenameStore) Upsert(ctx context.Context, rename *models.AIFileRename) error {
	now := time.Now().Unix()
	rename.UpdatedAt = now
	if rename.CreatedAt == 0 {
		rename.CreatedAt = now
	}

	_, err := dbWrapper.Exec(ctx, `
		INSERT INTO ai_file_rename (entity_type, entity_id, current_name, suggested_name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_id) DO UPDATE SET
			current_name = excluded.current_name,
			suggested_name = excluded.suggested_name,
			status = excluded.status,
			updated_at = excluded.updated_at
	`, rename.EntityType, rename.EntityID, rename.CurrentName, rename.SuggestedName, rename.Status,
		rename.CreatedAt, rename.UpdatedAt)

	if err == nil {
		_ = dbWrapper.Get(ctx, &rename.ID, `SELECT id FROM ai_file_rename WHERE entity_type = ? AND entity_id = ?`, rename.EntityType, rename.EntityID)
	}
	return err
}

func (s *aiFileRenameStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().Unix()
	_, err := dbWrapper.Exec(ctx,
		`UPDATE ai_file_rename SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id,
	)
	return err
}

func (s *aiFileRenameStore) Delete(ctx context.Context, id int64) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_file_rename WHERE id = ?`, id)
	return err
}

func (s *aiFileRenameStore) scanRows(rows *sqlx.Rows) ([]*models.AIFileRename, error) {
	var out []*models.AIFileRename
	for rows.Next() {
		var r models.AIFileRename
		if err := rows.Scan(&r.ID, &r.EntityType, &r.EntityID, &r.CurrentName, &r.SuggestedName, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	return out, nil
}
