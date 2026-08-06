package api

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// initDuplicateManager installs a manager with the embedding model configured.
func initDuplicateManager() {
	mgr := initTestManager()
	mgr.Config.SetAIEmbeddingModel("nomic-embed-text")
}

func TestSemanticDuplicates_ThresholdOverride(t *testing.T) {
	initDuplicateManager()

	ctx := context.Background()
	input := SemanticDuplicateInput{
		EntityTypes:  []string{"scene"},
		Threshold:    float64Ptr(0.95),
		MinGroupSize: intPtr(3),
	}

	db := mocks.NewDatabase()
	db.Embedding.On("FindNearDuplicates", mock.Anything, "scene", mock.Anything, 0.95, 3, 10000).Return([]models.DuplicateGroup{}, nil)

	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	groups, err := r.SemanticDuplicates(ctx, input)

	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestSemanticDuplicates_DefaultThreshold(t *testing.T) {
	initDuplicateManager()

	ctx := context.Background()
	input := SemanticDuplicateInput{EntityTypes: []string{"scene"}}

	db := mocks.NewDatabase()
	db.Embedding.On("FindNearDuplicates", mock.Anything, "scene", mock.Anything, 0.9, 2, 10000).Return([]models.DuplicateGroup{}, nil)

	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	_, err := r.SemanticDuplicates(ctx, input)

	require.NoError(t, err)
}

func TestSemanticDuplicates_MultipleEntityTypes(t *testing.T) {
	initDuplicateManager()

	ctx := context.Background()
	input := SemanticDuplicateInput{
		EntityTypes: []string{"scene", "performer", "image"},
	}

	db := mocks.NewDatabase()
	db.Embedding.On("FindNearDuplicates", mock.Anything, "scene", mock.Anything, 0.9, 2, 10000).Return([]models.DuplicateGroup{}, nil).Once()
	db.Embedding.On("FindNearDuplicates", mock.Anything, "performer", mock.Anything, 0.9, 2, 10000).Return([]models.DuplicateGroup{}, nil).Once()
	db.Embedding.On("FindNearDuplicates", mock.Anything, "image", mock.Anything, 0.9, 2, 10000).Return([]models.DuplicateGroup{}, nil).Once()

	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	_, err := r.SemanticDuplicates(ctx, input)

	require.NoError(t, err)
}

func TestSemanticDuplicates_CustomModel(t *testing.T) {
	initDuplicateManager()

	ctx := context.Background()
	input := SemanticDuplicateInput{
		EntityTypes: []string{"scene"},
		Model:       "custom-embed-model",
	}

	db := mocks.NewDatabase()
	db.Embedding.On("FindNearDuplicates", mock.Anything, "scene", "custom-embed-model", 0.9, 2, 10000).Return([]models.DuplicateGroup{}, nil)

	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	_, err := r.SemanticDuplicates(ctx, input)

	require.NoError(t, err)
}

func TestSemanticDuplicates_NoModelConfigured(t *testing.T) {
	mgr := initTestManager()
	mgr.Config.SetAIEmbeddingModel("")
	mgr.Config.SetAIModel("")

	ctx := context.Background()
	input := SemanticDuplicateInput{EntityTypes: []string{"scene"}}

	db := mocks.NewDatabase()
	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	_, err := r.SemanticDuplicates(ctx, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no embedding model configured")
}

func TestSemanticDuplicates_ReturnsGroups(t *testing.T) {
	initDuplicateManager()

	ctx := context.Background()
	input := SemanticDuplicateInput{EntityTypes: []string{"scene"}}

	db := mocks.NewDatabase()
	db.Embedding.On("FindNearDuplicates", mock.Anything, "scene", mock.Anything, 0.9, 2, 10000).Return([]models.DuplicateGroup{
		{EntityIDs: []int{1, 2}},
		{EntityIDs: []int{3, 4, 5}},
	}, nil)
	db.Scene.On("Find", mock.Anything, mock.Anything).Return(nil, nil)

	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	groups, err := r.SemanticDuplicates(ctx, input)

	require.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Len(t, groups[0].Entities, 2)
	assert.Len(t, groups[1].Entities, 3)
	assert.Equal(t, "scene", groups[0].EntityType)
}

func float64Ptr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
