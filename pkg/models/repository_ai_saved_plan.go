package models

import "context"

type AISavedPlan struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SceneIDs  []int  `json:"scene_ids"`
	CreatedAt int64  `json:"created_at"`
}

type AISavedPlanReader interface {
	FindAll(ctx context.Context) ([]*AISavedPlan, error)
}

type AISavedPlanWriter interface {
	Create(ctx context.Context, plan *AISavedPlan) error
	DeleteByID(ctx context.Context, id int64) error
}

type AISavedPlanReaderWriter interface {
	AISavedPlanReader
	AISavedPlanWriter
}
