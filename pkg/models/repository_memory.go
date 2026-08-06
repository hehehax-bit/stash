package models

import "context"

type AIMemory struct {
	ID        int64  `json:"id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type AIMemoryReader interface {
	FindAll(ctx context.Context) ([]*AIMemory, error)
	FindByKey(ctx context.Context, key string) (*AIMemory, error)
}

type AIMemoryWriter interface {
	Set(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
	DeleteAll(ctx context.Context) error
}

type AIMemoryReaderWriter interface {
	AIMemoryReader
	AIMemoryWriter
}
