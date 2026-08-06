package models

import "context"

type AISavedMoment struct {
	ID        int64 `json:"id"`
	MarkerID  int   `json:"marker_id"`
	CreatedAt int64 `json:"created_at"`
}

type AISavedMomentReader interface {
	FindAll(ctx context.Context) ([]*AISavedMoment, error)
}

type AISavedMomentWriter interface {
	Create(ctx context.Context, saved *AISavedMoment) error
	DeleteByMarkerID(ctx context.Context, markerID int) error
}

type AISavedMomentReaderWriter interface {
	AISavedMomentReader
	AISavedMomentWriter
}
