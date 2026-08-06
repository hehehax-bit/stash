package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiPerformerCareerStore struct{}

func NewAIPerformerCareerStore() *aiPerformerCareerStore {
	return &aiPerformerCareerStore{}
}

const aiPerformerCareerColumns = "id, performer_id, career_start, career_end, active_years, primary_niches, notable_studios, career_highlights, summary, created_at, updated_at"

func (s *aiPerformerCareerStore) FindByPerformerID(ctx context.Context, performerID int) (*models.AIPerformerCareer, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiPerformerCareerColumns+` FROM ai_performer_career WHERE performer_id = ?`, performerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	careers, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(careers) == 0 {
		return nil, nil
	}
	return careers[0], nil
}

func (s *aiPerformerCareerStore) FindAssessedPerformers(ctx context.Context) ([]int, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT performer_id FROM ai_performer_career`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *aiPerformerCareerStore) Upsert(ctx context.Context, career *models.AIPerformerCareer) error {
	now := time.Now().Unix()
	career.UpdatedAt = now
	if career.CreatedAt == 0 {
		career.CreatedAt = now
	}

	activeYears, err := json.Marshal(career.ActiveYears)
	if err != nil {
		return err
	}
	niches, err := json.Marshal(career.PrimaryNiches)
	if err != nil {
		return err
	}
	studios, err := json.Marshal(career.NotableStudios)
	if err != nil {
		return err
	}

	var start, end *int
	if career.CareerStart != nil {
		start = career.CareerStart
	}
	if career.CareerEnd != nil {
		end = career.CareerEnd
	}

	_, err = dbWrapper.Exec(ctx, `
		INSERT INTO ai_performer_career (performer_id, career_start, career_end, active_years, primary_niches, notable_studios, career_highlights, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(performer_id) DO UPDATE SET
			career_start = excluded.career_start,
			career_end = excluded.career_end,
			active_years = excluded.active_years,
			primary_niches = excluded.primary_niches,
			notable_studios = excluded.notable_studios,
			career_highlights = excluded.career_highlights,
			summary = excluded.summary,
			updated_at = excluded.updated_at
	`, career.PerformerID, start, end, string(activeYears), string(niches), string(studios),
		career.CareerHighlights, career.Summary, career.CreatedAt, career.UpdatedAt)

	if err == nil {
		_ = dbWrapper.Get(ctx, &career.ID, `SELECT id FROM ai_performer_career WHERE performer_id = ?`, career.PerformerID)
	}
	return err
}

func (s *aiPerformerCareerStore) Delete(ctx context.Context, id int64) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_performer_career WHERE id = ?`, id)
	return err
}

func (s *aiPerformerCareerStore) scanRows(rows *sqlx.Rows) ([]*models.AIPerformerCareer, error) {
	var out []*models.AIPerformerCareer
	for rows.Next() {
		var c models.AIPerformerCareer
		var activeYears, niches, studios string
		if err := rows.Scan(&c.ID, &c.PerformerID, &c.CareerStart, &c.CareerEnd, &activeYears, &niches, &studios, &c.CareerHighlights, &c.Summary, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(activeYears), &c.ActiveYears)
		_ = json.Unmarshal([]byte(niches), &c.PrimaryNiches)
		_ = json.Unmarshal([]byte(studios), &c.NotableStudios)
		out = append(out, &c)
	}
	return out, nil
}
