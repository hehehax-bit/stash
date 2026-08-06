package ai

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateTag_CreatesNewTag(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "New Tag", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "New Tag", true).Return(nil, nil)
	db.Tag.On("FindByName", mock.Anything, "Anal Sex", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "Anal Sex", true).Return(nil, nil)
	db.Tag.On("Create", mock.Anything, mock.MatchedBy(func(input *models.CreateTagInput) bool {
		return input.Tag != nil && input.Tag.Name == "New Tag"
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.CreateTagInput).Tag.ID = 9
	}).Return(nil)

	result, err := createTag(context.Background(), db.Repository(), []byte(`{"name": "New Tag", "aliases": ["Anal Sex"], "description": "A new tag"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "Created [Tag #9 - New Tag]")
	db.Tag.AssertExpectations(t)
}

func TestCreateTag_EmptyName(t *testing.T) {
	db := mocks.NewDatabase()

	result, err := createTag(context.Background(), db.Repository(), []byte(`{"name": "  "}`))

	require.NoError(t, err)
	assert.Equal(t, "name is required.", result)
}

func TestCreateTag_MissingParentTag(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "New Tag", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "New Tag", true).Return(nil, nil)
	db.Tag.On("FindByName", mock.Anything, "Missing Parent", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "Missing Parent", true).Return(nil, nil)

	result, err := createTag(context.Background(), db.Repository(), []byte(`{"name": "New Tag", "parent_names": ["Missing Parent"]}`))

	require.NoError(t, err)
	assert.Contains(t, result, "Parent tag \"Missing Parent\" not found")
}

func TestMergeTags_ConfirmedCreatesDestination(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Anal", true).Return(&models.Tag{ID: 5, Name: "Anal"}, nil)
	db.Tag.On("FindByName", mock.Anything, "New Dest", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "New Dest", true).Return(nil, nil)
	db.Tag.On("Create", mock.Anything, mock.MatchedBy(func(input *models.CreateTagInput) bool {
		return input.Tag != nil && input.Tag.Name == "New Dest"
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.CreateTagInput).Tag.ID = 77
	}).Return(nil)
	db.Tag.On("Merge", mock.Anything, []int{5}, 77).Return(nil)

	result, err := mergeTags(context.Background(), db.Repository(), []byte(`{"source_names": ["Anal"], "destination_name": "New Dest", "confirmed": true}`))

	require.NoError(t, err)
	assert.Contains(t, result, "Merged tags into [Tag #77 - New Dest]")
	db.Tag.AssertExpectations(t)
}

func TestMergeTags_UnconfirmedReturnsSummary(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Anal", true).Return(&models.Tag{ID: 5, Name: "Anal"}, nil)
	db.Tag.On("FindByName", mock.Anything, "New Dest", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "New Dest", true).Return(nil, nil)
	db.Scene.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
	db.Scene.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mocks.SceneQueryResult([]*models.Scene{}, 0), nil)
	db.Image.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
	db.Image.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mocks.ImageQueryResult([]*models.Image{}, 0), nil)

	result, err := mergeTags(context.Background(), db.Repository(), []byte(`{"source_names": ["Anal"], "destination_name": "New Dest", "confirmed": false}`))

	require.NoError(t, err)
	assert.Contains(t, result, "This merge would move")
	assert.Contains(t, result, "Call again with `\"confirmed\": true` to execute.")
}

func TestRecommendScene(t *testing.T) {
	db := mocks.NewDatabase()

	// embedding server
	embedServer := newEmbeddingServer(t, []float32{0.5, 0.5})
	defer embedServer.Close()

	cfg := ToolConfig{
		LLMBaseURL:     embedServer.URL,
		LLMModel:       "llm",
		EmbeddingModel: "embed",
	}

	db.Embedding.On("SearchSimilar", mock.Anything, "scene", "embed", mock.Anything, 9).
		Return([]models.SimilarityResult{
			{EntityID: 1, Score: 0.9},
			{EntityID: 2, Score: 0.8},
		}, nil)
	db.Scene.On("Find", mock.Anything, 1).Return(&models.Scene{ID: 1, Title: "Bondage night"}, nil)
	db.Scene.On("Find", mock.Anything, 2).Return(&models.Scene{ID: 2, Title: "Slow evening"}, nil)
	db.AISceneAudio.On("FindBySceneID", mock.Anything, 1).Return(&models.AISceneAudio{SceneID: 1, Moans: true}, nil)
	db.AISceneAudio.On("FindBySceneID", mock.Anything, 2).Return(&models.AISceneAudio{SceneID: 2, Moans: false}, nil)
	db.Scene.On("LoadPrimaryFile", mock.Anything, mock.Anything).Return(nil)

	result, err := recommendScene(context.Background(), db.Repository(), []byte(`{"vibe": "bondage and rough"}`), cfg)

	require.NoError(t, err)
	// the moaning scene is boosted and listed first
	posBondage := indexOfStr(result, "Bondage night")
	posSlow := indexOfStr(result, "Slow evening")
	require.GreaterOrEqual(t, posBondage, 0)
	assert.True(t, posBondage < posSlow, "moaning scene should be ranked first")
	assert.Contains(t, result, "/scenes/1")
	db.Embedding.AssertExpectations(t)
	db.AISceneAudio.AssertExpectations(t)
}

func indexOfStr(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
