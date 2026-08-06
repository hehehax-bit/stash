package models

import "context"

type AIPerformerCareer struct {
	ID               int64    `json:"id"`
	PerformerID      int      `json:"performer_id"`
	CareerStart      *int     `json:"career_start"`
	CareerEnd        *int     `json:"career_end"`
	ActiveYears      []string `json:"active_years"`
	PrimaryNiches    []string `json:"primary_niches"`
	NotableStudios   []string `json:"notable_studios"`
	CareerHighlights string   `json:"career_highlights"`
	Summary          string   `json:"summary"`
	CreatedAt        int64    `json:"created_at"`
	UpdatedAt        int64    `json:"updated_at"`
}

type AIPerformerCareerReader interface {
	FindByPerformerID(ctx context.Context, performerID int) (*AIPerformerCareer, error)
	FindAssessedPerformers(ctx context.Context) ([]int, error)
}

type AIPerformerCareerWriter interface {
	Upsert(ctx context.Context, career *AIPerformerCareer) error
	Delete(ctx context.Context, id int64) error
}

type AIPerformerCareerReaderWriter interface {
	AIPerformerCareerReader
	AIPerformerCareerWriter
}
