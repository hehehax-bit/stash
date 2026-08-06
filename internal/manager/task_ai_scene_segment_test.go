package manager

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeSegments_JSONParsing_Valid(t *testing.T) {
	server := newAIChatServer(t, `{"segments": [{"title": "Intro", "start": 0, "end": 30, "description": "Opening scene", "tags": ["solo", "tease"]}]}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AISceneSegmentJob{}

	shots := []aiSceneShot{
		{Timestamp: 10, MultiImage: ai.MultiImage{Base64: "img1"}},
		{Timestamp: 20, MultiImage: ai.MultiImage{Base64: "img2"}},
	}

	result, err := j.analyzeSegments(context.Background(), client, &models.Scene{}, shots)

	require.NoError(t, err)
	require.Len(t, result.Segments, 1)
	assert.Equal(t, "Intro", result.Segments[0].Title)
	assert.Equal(t, 0.0, result.Segments[0].Start)
	assert.Equal(t, 30.0, *result.Segments[0].End)
	assert.Equal(t, "Opening scene", result.Segments[0].Description)
	assert.Equal(t, []string{"solo", "tease"}, result.Segments[0].Tags)
}

func TestAnalyzeSegments_JSONParsing_MultipleSegments(t *testing.T) {
	server := newAIChatServer(t, `{"segments": [
		{"title": "Intro", "start": 0, "end": 60, "description": "Tease", "tags": ["solo"]},
		{"title": "Main Act", "start": 60, "end": 300, "description": "Action", "tags": ["oral", "sex"]},
		{"title": "Finale", "start": 300, "end": null, "description": "Ending", "tags": ["cumshot"]}
	]}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AISceneSegmentJob{}

	shots := []aiSceneShot{
		{Timestamp: 30, MultiImage: ai.MultiImage{Base64: "img1"}},
		{Timestamp: 180, MultiImage: ai.MultiImage{Base64: "img2"}},
		{Timestamp: 400, MultiImage: ai.MultiImage{Base64: "img3"}},
	}

	result, err := j.analyzeSegments(context.Background(), client, &models.Scene{}, shots)

	require.NoError(t, err)
	require.Len(t, result.Segments, 3)
	assert.Nil(t, result.Segments[2].End)
}

func TestAnalyzeSegments_InvalidJSON(t *testing.T) {
	server := newAIChatServer(t, "not json")
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AISceneSegmentJob{}

	shots := []aiSceneShot{{Timestamp: 10, MultiImage: ai.MultiImage{Base64: "img1"}}}

	_, err := j.analyzeSegments(context.Background(), client, &models.Scene{}, shots)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing segment analysis response")
}

func TestAnalyzeSegments_EmptySegments(t *testing.T) {
	server := newAIChatServer(t, `{}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AISceneSegmentJob{}

	shots := []aiSceneShot{{Timestamp: 10, MultiImage: ai.MultiImage{Base64: "img1"}}}

	result, err := j.analyzeSegments(context.Background(), client, &models.Scene{}, shots)

	require.NoError(t, err)
	assert.Empty(t, result.Segments)
}

func TestApplySegments_Clamping_NegativeStart(t *testing.T) {
	db := mocks.NewDatabase()
	var created *models.SceneMarker
	db.SceneMarker.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		created = args.Get(1).(*models.SceneMarker)
		created.ID = 1
	})

	j := &AISceneSegmentJob{}
	s := &models.Scene{ID: 1}
	seg := aiSceneSegmentation{
		Segments: []aiSceneSegment{
			{Title: "Test", Start: -5, End: float64Ptr(50)},
		},
	}

	err := j.applySegments(context.Background(), db.Repository(), s, &seg, 999)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, 0.0, created.Seconds)
	assert.Equal(t, 50.0, *created.EndSeconds)
	assert.Equal(t, 999, created.PrimaryTagID)
	assert.Equal(t, 1, created.SceneID)
}

func TestApplySegments_Clamping_EndBeyondDuration(t *testing.T) {
	db := mocks.NewDatabase()
	var created *models.SceneMarker
	db.SceneMarker.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		created = args.Get(1).(*models.SceneMarker)
		created.ID = 1
	})

	j := &AISceneSegmentJob{}
	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles([]*models.VideoFile{{
		BaseFile: &models.BaseFile{},
		Duration: 100,
	}})
	seg := aiSceneSegmentation{
		Segments: []aiSceneSegment{
			{Title: "Test", Start: 80, End: float64Ptr(150)},
		},
	}

	err := j.applySegments(context.Background(), db.Repository(), s, &seg, 999)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, 80.0, created.Seconds)
	assert.Equal(t, 100.0, *created.EndSeconds)
}

func TestApplySegments_Clamping_StartAtDuration(t *testing.T) {
	db := mocks.NewDatabase()
	var created *models.SceneMarker
	db.SceneMarker.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		created = args.Get(1).(*models.SceneMarker)
		created.ID = 1
	})

	j := &AISceneSegmentJob{}
	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles([]*models.VideoFile{{
		BaseFile: &models.BaseFile{},
		Duration: 100,
	}})
	seg := aiSceneSegmentation{
		Segments: []aiSceneSegment{
			{Title: "Test", Start: 100, End: float64Ptr(120)},
		},
	}

	err := j.applySegments(context.Background(), db.Repository(), s, &seg, 999)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, 99.0, created.Seconds)
	require.NotNil(t, created.EndSeconds)
	assert.Equal(t, 100.0, *created.EndSeconds)
}

func TestApplySegments_EmptyTitleSkipped(t *testing.T) {
	db := mocks.NewDatabase()
	var created []*models.SceneMarker
	db.SceneMarker.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		created = append(created, args.Get(1).(*models.SceneMarker))
	})

	j := &AISceneSegmentJob{}
	s := &models.Scene{ID: 1}
	seg := aiSceneSegmentation{
		Segments: []aiSceneSegment{
			{Title: "", Start: 0, End: float64Ptr(10)},
			{Title: "Valid", Start: 10, End: float64Ptr(20)},
		},
	}

	err := j.applySegments(context.Background(), db.Repository(), s, &seg, 999)

	require.NoError(t, err)
	require.Len(t, created, 1)
	assert.Equal(t, "Valid", created[0].Title)
}

func TestApplySegments_TagCreation(t *testing.T) {
	db := mocks.NewDatabase()
	var created *models.SceneMarker
	db.SceneMarker.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		created = args.Get(1).(*models.SceneMarker)
		created.ID = 5
	})
	db.Tag.On("FindByName", mock.Anything, "new-tag", true).Return(nil, nil)
	db.Tag.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		input := args.Get(1).(*models.CreateTagInput)
		input.Tag.ID = 123
	}).Return(nil)
	db.SceneMarker.On("UpdateTags", mock.Anything, 5, []int{123}).Return(nil)

	j := &AISceneSegmentJob{}
	s := &models.Scene{ID: 1}
	seg := aiSceneSegmentation{
		Segments: []aiSceneSegment{
			{Title: "Test", Start: 0, End: float64Ptr(10), Tags: []string{"new-tag"}},
		},
	}

	err := j.applySegments(context.Background(), db.Repository(), s, &seg, 999)

	require.NoError(t, err)
	require.NotNil(t, created)
	db.Tag.AssertExpectations(t)
	db.SceneMarker.AssertExpectations(t)
}

func TestApplySegments_ExistingTagReused(t *testing.T) {
	db := mocks.NewDatabase()
	db.SceneMarker.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*models.SceneMarker).ID = 5
	})
	db.Tag.On("FindByName", mock.Anything, "existing", true).Return(&models.Tag{ID: 42}, nil)
	db.SceneMarker.On("UpdateTags", mock.Anything, 5, []int{42}).Return(nil)

	j := &AISceneSegmentJob{}
	s := &models.Scene{ID: 1}
	seg := aiSceneSegmentation{
		Segments: []aiSceneSegment{
			{Title: "Test", Start: 0, End: float64Ptr(10), Tags: []string{"existing"}},
		},
	}

	err := j.applySegments(context.Background(), db.Repository(), s, &seg, 999)

	require.NoError(t, err)
	db.Tag.AssertNotCalled(t, "Create")
	db.SceneMarker.AssertExpectations(t)
}

func TestSceneShots_NoFile(t *testing.T) {
	db := mocks.NewDatabase()
	j := &AISceneSegmentJob{progress: &job.Progress{}}

	s := &models.Scene{ID: 1}
	shots, err := j.sceneShots(context.Background(), s, db.Repository())

	assert.Error(t, err)
	assert.Nil(t, shots)
}

func TestSceneShots_NoPath(t *testing.T) {
	db := mocks.NewDatabase()
	j := &AISceneSegmentJob{progress: &job.Progress{}}

	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles([]*models.VideoFile{{
		BaseFile: &models.BaseFile{},
		Duration: 60,
	}})

	shots, err := j.sceneShots(context.Background(), s, db.Repository())

	assert.Error(t, err)
	assert.Nil(t, shots)
}

func TestSceneShots_GenerationFailuresProduceNoShots(t *testing.T) {
	db := mocks.NewDatabase()
	initClusterInstance(db)
	j := &AISceneSegmentJob{progress: &job.Progress{}}

	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles([]*models.VideoFile{{
		BaseFile: &models.BaseFile{Path: "/nonexistent/video.mp4"},
		Duration: 120,
	}})

	shots, err := j.sceneShots(context.Background(), s, db.Repository())

	require.NoError(t, err)
	assert.Empty(t, shots)
}

func TestSegmentScene_NoScreenshots(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, mock.Anything, true).Return(&models.Tag{ID: 999}, nil)
	initClusterInstance(db)

	client := ai.NewClient("http://localhost:1", "test-model")
	j := &AISceneSegmentJob{progress: &job.Progress{}}

	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles([]*models.VideoFile{{
		BaseFile: &models.BaseFile{Path: "/nonexistent/video.mp4"},
		Duration: 120,
	}})

	err := j.segmentScene(context.Background(), client, db.Repository(), s)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no screenshots could be generated")
}

func float64Ptr(f float64) *float64 {
	return &f
}
