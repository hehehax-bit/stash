package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiSavedMomentStore struct{}

func NewAISavedMomentStore() *aiSavedMomentStore {
	return &aiSavedMomentStore{}
}

func (s *aiSavedMomentStore) FindAll(ctx context.Context) ([]*models.AISavedMoment, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, marker_id, created_at FROM ai_saved_moments ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.AISavedMoment
	for rows.Next() {
		var m models.AISavedMoment
		if err := rows.Scan(&m.ID, &m.MarkerID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (s *aiSavedMomentStore) Create(ctx context.Context, saved *models.AISavedMoment) error {
	saved.CreatedAt = time.Now().Unix()
	_, err := dbWrapper.Exec(ctx, `INSERT OR IGNORE INTO ai_saved_moments (marker_id, created_at) VALUES (?, ?)`,
		saved.MarkerID, saved.CreatedAt)
	return err
}

func (s *aiSavedMomentStore) DeleteByMarkerID(ctx context.Context, markerID int) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_saved_moments WHERE marker_id = ?`, markerID)
	return err
}

var _ = sqlx.Rows{}
