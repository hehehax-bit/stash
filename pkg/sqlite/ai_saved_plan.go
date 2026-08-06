package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiSavedPlanStore struct{}

func NewAISavedPlanStore() *aiSavedPlanStore {
	return &aiSavedPlanStore{}
}

func (s *aiSavedPlanStore) FindAll(ctx context.Context) ([]*models.AISavedPlan, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, name, scene_ids, created_at FROM ai_saved_plans ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.AISavedPlan
	for rows.Next() {
		var p models.AISavedPlan
		var sceneIDs string
		if err := rows.Scan(&p.ID, &p.Name, &sceneIDs, &p.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(sceneIDs), &p.SceneIDs)
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (s *aiSavedPlanStore) Create(ctx context.Context, plan *models.AISavedPlan) error {
	plan.CreatedAt = time.Now().Unix()
	sceneIDs, err := json.Marshal(plan.SceneIDs)
	if err != nil {
		return err
	}
	_, err = dbWrapper.Exec(ctx, `INSERT INTO ai_saved_plans (name, scene_ids, created_at) VALUES (?, ?, ?)`,
		plan.Name, string(sceneIDs), plan.CreatedAt)
	if err != nil {
		return err
	}
	return dbWrapper.Get(ctx, &plan.ID, `SELECT id FROM ai_saved_plans WHERE rowid = last_insert_rowid()`)
}

func (s *aiSavedPlanStore) DeleteByID(ctx context.Context, id int64) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_saved_plans WHERE id = ?`, id)
	return err
}

var _ = sqlx.Rows{}
