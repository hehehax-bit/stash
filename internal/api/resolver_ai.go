package api

import (
	"context"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

type aiChatMessageResolver struct{ *Resolver }

func (r *aiChatMessageResolver) CreatedAt(ctx context.Context, obj *models.AIChatMessage) (*time.Time, error) {
	t := time.Unix(obj.CreatedAt, 0)
	return &t, nil
}

type aiChatSessionResolver struct{ *Resolver }

func (r *aiChatSessionResolver) CreatedAt(ctx context.Context, obj *models.AIChatSession) (*time.Time, error) {
	t := time.Unix(obj.CreatedAt, 0)
	return &t, nil
}

func (r *aiChatSessionResolver) UpdatedAt(ctx context.Context, obj *models.AIChatSession) (*time.Time, error) {
	t := time.Unix(obj.UpdatedAt, 0)
	return &t, nil
}
