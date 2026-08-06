package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestResolveAITag_ExistingTag(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "AI Tagged", true).Return(&models.Tag{ID: 42, Name: "AI Tagged"}, nil)
	initClusterInstance(db)

	id, err := resolveAITag(context.Background(), db.Repository())

	require.NoError(t, err)
	assert.Equal(t, 42, id)
	db.Tag.AssertExpectations(t)
}

func TestResolveAITag_CreatesMissingTag(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "AI Tagged", true).Return(nil, nil)
	db.Tag.On("Create", mock.Anything, mock.MatchedBy(func(input *models.CreateTagInput) bool {
		return input.Tag != nil && input.Tag.Name == "AI Tagged"
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.CreateTagInput).Tag.ID = 42
	}).Return(nil)
	initClusterInstance(db)

	id, err := resolveAITag(context.Background(), db.Repository())

	require.NoError(t, err)
	assert.Equal(t, 42, id)
	db.Tag.AssertExpectations(t)
}

func TestResolveAITag_FindError(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "AI Tagged", true).Return(nil, errors.New("db error"))
	initClusterInstance(db)

	_, err := resolveAITag(context.Background(), db.Repository())

	require.Error(t, err)
}

func TestCoverFallback_UsesSceneCover(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("GetCover", mock.Anything, 1).Return([]byte("\xff\xd8\xff\xe0fake-jpeg"), nil)

	j := &AISceneTagJob{}
	images, err := j.coverFallback(context.Background(), &models.Scene{ID: 1}, db.Repository())

	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "image/jpeg", images[0].MediaType)
}

func TestCoverFallback_NoCover(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("GetCover", mock.Anything, 1).Return([]byte{}, nil)

	j := &AISceneTagJob{}
	_, err := j.coverFallback(context.Background(), &models.Scene{ID: 1}, db.Repository())

	require.Error(t, err)
}

func TestTagSingleScene_ReusesExistingEntities(t *testing.T) {
	server := newAIChatServer(t, `{
		"title": "Jane Doe blowjob",
		"performers": [{"name": "Jane Doe", "gender": "Female", "ethnicity": "Caucasian", "hair_color": "Blonde", "eye_color": "Blue", "details": "blonde woman"}],
		"tags": ["Anal", "Blonde"],
		"details": "Jane Doe performs a blowjob."
	}`)
	defer server.Close()

	db := mocks.NewDatabase()
	db.Scene.On("GetCover", mock.Anything, 1).Return([]byte("cover-data"), nil)
	db.Performer.On("FindByNames", mock.Anything, []string{"Jane Doe"}, true).Return([]*models.Performer{
		{ID: 5, Name: "Jane Doe"},
	}, nil)
	db.Tag.On("FindByName", mock.Anything, "Anal", true).Return(&models.Tag{ID: 7, Name: "Anal"}, nil)
	db.Tag.On("FindByName", mock.Anything, "Blonde", true).Return(&models.Tag{ID: 8, Name: "Blonde"}, nil)

	var updatedScene models.ScenePartial
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).
		Run(func(args mock.Arguments) {
			updatedScene = args.Get(2).(models.ScenePartial)
		}).
		Return(&models.Scene{ID: 1}, nil)

	initClusterInstance(db)

	j := &AISceneTagJob{}
	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles(nil)

	result, err := j.tagSingleScene(context.Background(), ai.NewClient(server.URL, "test-model"), db.Repository(), s, true, true, false, false, 99, "")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Jane Doe blowjob", result.Title)

	require.NotNil(t, updatedScene.Title)
	assert.Equal(t, "Jane Doe blowjob", updatedScene.Title.Value)

	require.NotNil(t, updatedScene.PerformerIDs)
	assert.Equal(t, []int{5}, updatedScene.PerformerIDs.IDs)

	require.NotNil(t, updatedScene.TagIDs)
	assert.Contains(t, updatedScene.TagIDs.IDs, 7)
	assert.Contains(t, updatedScene.TagIDs.IDs, 99, "the AI tag must always be added")

	db.Performer.AssertExpectations(t)
	db.Tag.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}

func TestTagSingleScene_GenericPerformerSkipped(t *testing.T) {
	server := newAIChatServer(t, `{
		"title": "anal scene",
		"performers": [{"name": "unknown woman", "gender": "Female"}],
		"tags": [],
		"details": "an anal scene"
	}`)
	defer server.Close()

	db := mocks.NewDatabase()
	db.Scene.On("GetCover", mock.Anything, 1).Return([]byte("cover-data"), nil)

	var updatedScene models.ScenePartial
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).
		Run(func(args mock.Arguments) {
			updatedScene = args.Get(2).(models.ScenePartial)
		}).
		Return(&models.Scene{ID: 1}, nil)

	initClusterInstance(db)

	j := &AISceneTagJob{}
	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles(nil)

	_, err := j.tagSingleScene(context.Background(), ai.NewClient(server.URL, "test-model"), db.Repository(), s, true, true, false, false, 99, "")

	require.NoError(t, err)
	assert.Nil(t, updatedScene.PerformerIDs, "generic performers must not be added")
	require.NotNil(t, updatedScene.TagIDs)
	assert.Equal(t, []int{99}, updatedScene.TagIDs.IDs)
}

func TestTagSingleScene_NoScreenshotsOrCover(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("GetCover", mock.Anything, 1).Return([]byte{}, nil)

	initClusterInstance(db)

	j := &AISceneTagJob{}
	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles(nil)

	_, err := j.tagSingleScene(context.Background(), ai.NewClient("http://example.com", "test"), db.Repository(), s, true, true, false, false, 99, "")

	require.Error(t, err)
}

func TestEffectiveAIShotCount(t *testing.T) {
	initClusterInstance(mocks.NewDatabase())
	auto := func(d float64) int { return 4 }

	tests := []struct {
		name      string
		requested *int
		config    int
		want      int
	}{
		{"per-run override wins", intPtr(10), 0, 10},
		{"override wins over config", intPtr(6), 8, 6},
		{"config default used when no override", nil, 8, 8},
		{"automatic fallback", nil, 0, 4},
		{"zero override means auto", intPtr(0), 0, 4},
		{"zero override falls back to config", intPtr(0), 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance.Config.SetAIFramesToSample(tt.config)
			got := effectiveAIShotCount(tt.requested, 120, auto)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTagSingleScene_PerformersOnly_AttachesPerformersOnly(t *testing.T) {
	server := newAIChatServer(t, `{
		"title": "Jane Doe blowjob",
		"performers": [{"name": "Jane Doe", "gender": "Female", "ethnicity": "Caucasian", "hair_color": "Blonde", "eye_color": "Blue", "details": "blonde woman"}],
		"tags": ["Anal", "Blonde"],
		"details": "Jane Doe performs a blowjob."
	}`)
	defer server.Close()

	db := mocks.NewDatabase()
	db.Scene.On("GetCover", mock.Anything, 1).Return([]byte("cover-data"), nil)
	db.Performer.On("FindByNames", mock.Anything, []string{"Jane Doe"}, true).Return([]*models.Performer{
		{ID: 5, Name: "Jane Doe"},
	}, nil)

	var updatedScene models.ScenePartial
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).
		Run(func(args mock.Arguments) {
			updatedScene = args.Get(2).(models.ScenePartial)
		}).
		Return(&models.Scene{ID: 1}, nil)

	initClusterInstance(db)

	j := &AISceneTagJob{}
	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles(nil)

	result, err := j.tagSingleScene(context.Background(), ai.NewClient(server.URL, "test-model"), db.Repository(), s, true, true, true, false, 99, "")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Jane Doe blowjob", result.Title)

	require.NotNil(t, updatedScene.PerformerIDs)
	assert.Equal(t, []int{5}, updatedScene.PerformerIDs.IDs)
	assert.False(t, updatedScene.Title.Set, "performers-only mode must not change the title")
	assert.False(t, updatedScene.Details.Set, "performers-only mode must not change details")
	assert.Nil(t, updatedScene.TagIDs, "performers-only mode must not change tags")

	db.Performer.AssertExpectations(t)
	db.Tag.AssertNotCalled(t, "FindByName", mock.Anything, mock.Anything, mock.Anything)
	db.Scene.AssertExpectations(t)
}

func TestTagSingleScene_FillMissingOnly(t *testing.T) {
	server := newAIChatServer(t, `{
		"title": "Jane Doe blowjob",
		"performers": [{"name": "Jane Doe", "gender": "Female"}],
		"tags": ["Anal"],
		"details": "Jane Doe performs a blowjob."
	}`)
	defer server.Close()

	db := mocks.NewDatabase()
	db.Scene.On("GetCover", mock.Anything, 1).Return([]byte("cover-data"), nil)
	db.Performer.On("FindByNames", mock.Anything, []string{"Jane Doe"}, true).Return([]*models.Performer{
		{ID: 5, Name: "Jane Doe"},
	}, nil)

	var updatedScene models.ScenePartial
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).
		Run(func(args mock.Arguments) {
			updatedScene = args.Get(2).(models.ScenePartial)
		}).
		Return(&models.Scene{ID: 1}, nil)

	initClusterInstance(db)

	j := &AISceneTagJob{}
	s := &models.Scene{ID: 1, Title: "Existing Title", Details: ""}
	s.Files = models.NewRelatedVideoFiles(nil)

	_, err := j.tagSingleScene(context.Background(), ai.NewClient(server.URL, "test-model"), db.Repository(), s, true, true, false, true, 99, "")

	require.NoError(t, err)
	require.NotNil(t, updatedScene.PerformerIDs)
	assert.Equal(t, []int{5}, updatedScene.PerformerIDs.IDs)
	assert.False(t, updatedScene.Title.Set, "an existing title must not be overwritten")
	require.NotNil(t, updatedScene.Details)
	assert.Equal(t, "Jane Doe performs a blowjob.", updatedScene.Details.Value)
	assert.Nil(t, updatedScene.TagIDs, "fill-missing-only mode must not add tags")
}
