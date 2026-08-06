package manager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeMediaQuality_Clamping(t *testing.T) {
	server := newAIChatServer(t, `{"quality_score": 150, "visual_clarity": -10, "lighting": 80, "composition": 90, "camera_work": 75, "notes": "Great lighting"}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	quality, err := analyzeMediaQuality(context.Background(), client, images)

	require.NoError(t, err)
	assert.Equal(t, 100, quality.QualityScore)
	assert.Equal(t, 0, quality.VisualClarity)
	assert.Equal(t, 80, quality.Lighting)
	assert.Equal(t, 90, quality.Composition)
	assert.Equal(t, 75, quality.CameraWork)
	assert.Equal(t, "Great lighting", quality.Notes)
}

func TestAnalyzeMediaQuality_ValidScores(t *testing.T) {
	server := newAIChatServer(t, `{"quality_score": 85, "visual_clarity": 90, "lighting": 75, "composition": 80, "camera_work": 70, "notes": "Good quality"}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	quality, err := analyzeMediaQuality(context.Background(), client, images)

	require.NoError(t, err)
	assert.Equal(t, 85, quality.QualityScore)
	assert.Equal(t, 90, quality.VisualClarity)
	assert.Equal(t, 75, quality.Lighting)
	assert.Equal(t, 80, quality.Composition)
	assert.Equal(t, 70, quality.CameraWork)
}

func TestAnalyzeMediaQuality_InvalidJSON(t *testing.T) {
	server := newAIChatServer(t, "not json")
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	_, err := analyzeMediaQuality(context.Background(), client, images)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing AI response")
}

func TestAnalyzeMediaQuality_MissingFields(t *testing.T) {
	server := newAIChatServer(t, `{}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	quality, err := analyzeMediaQuality(context.Background(), client, images)

	require.NoError(t, err)
	assert.Equal(t, 0, quality.QualityScore)
	assert.Equal(t, 0, quality.VisualClarity)
	assert.Equal(t, 0, quality.Lighting)
	assert.Equal(t, 0, quality.Composition)
	assert.Equal(t, 0, quality.CameraWork)
}

func TestAssessedEntities_SkipLogic(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIMediaQuality.On("FindAssessedEntities", mock.Anything, "scene").Return([]int{1, 2}, nil)

	j := &AIMediaQualityJob{input: AIMediaQualityInput{EntityTypes: []string{"scene"}, Overwrite: false}}
	skip := j.assessedEntities(context.Background(), db.Repository(), "scene")

	assert.True(t, skip[1])
	assert.True(t, skip[2])
	assert.False(t, skip[3])
}

func TestAssessedEntities_NoExisting(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIMediaQuality.On("FindAssessedEntities", mock.Anything, "scene").Return([]int{}, nil)

	j := &AIMediaQualityJob{input: AIMediaQualityInput{EntityTypes: []string{"scene"}, Overwrite: false}}
	skip := j.assessedEntities(context.Background(), db.Repository(), "scene")

	assert.Empty(t, skip)
}

func TestAssessScenes_SkipAssessed(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIMediaQuality.On("FindAssessedEntities", mock.Anything, "scene").Return([]int{1}, nil)
	db.Scene.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(2, nil)
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(mocks.SceneQueryResult([]*models.Scene{
		{ID: 1},
		{ID: 2},
	}, 2), nil)
	initClusterInstance(db)

	client := ai.NewClient("http://localhost:1", "test-model")
	j := &AIMediaQualityJob{
		input:    AIMediaQualityInput{EntityTypes: []string{"scene"}, Overwrite: false},
		progress: &job.Progress{},
	}

	err := j.assessScenes(context.Background(), client, db.Repository(), 10)

	require.NoError(t, err)
	// scene 1 is skipped; scene 2 has no file so it is skipped internally
	db.AIMediaQuality.AssertNotCalled(t, "Upsert")
}

func TestAssessImages_SkipAssessed(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIMediaQuality.On("FindAssessedEntities", mock.Anything, "image").Return([]int{1}, nil)
	db.Image.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(2, nil)
	db.Image.On("Query", mock.Anything, mock.Anything).Return(mocks.ImageQueryResult([]*models.Image{
		{ID: 1},
		{ID: 2},
	}, 2), nil)
	initClusterInstance(db)

	client := ai.NewClient("http://localhost:1", "test-model")
	j := &AIMediaQualityJob{
		input:    AIMediaQualityInput{EntityTypes: []string{"image"}, Overwrite: false},
		progress: &job.Progress{},
	}

	err := j.assessImages(context.Background(), client, db.Repository(), 10)

	require.NoError(t, err)
	db.AIMediaQuality.AssertNotCalled(t, "Upsert")
}

func TestAssessScene_NoScreenshots(t *testing.T) {
	db := mocks.NewDatabase()
	initClusterInstance(db)

	client := ai.NewClient("http://localhost:1", "test-model")
	j := &AIMediaQualityJob{progress: &job.Progress{}}

	j.assessScene(context.Background(), client, db.Repository(), &models.Scene{ID: 1})

	db.AIMediaQuality.AssertNotCalled(t, "Upsert")
}

func TestAssessImage_SavesQuality(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.jpg")
	require.NoError(t, os.WriteFile(imgPath, []byte("fake-image-bytes"), 0o644))

	img := &models.Image{ID: 7, Path: imgPath}
	img.Files = models.NewRelatedFiles([]models.File{&models.ImageFile{
		BaseFile: &models.BaseFile{Path: imgPath},
	}})

	server := newAIChatServer(t, `{"quality_score": 85, "visual_clarity": 90, "lighting": 75, "composition": 80, "camera_work": 70, "notes": "Good"}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")

	db := mocks.NewDatabase()
	var saved *models.AIMediaQuality
	db.AIMediaQuality.On("Upsert", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.AIMediaQuality)
	})

	j := &AIMediaQualityJob{progress: &job.Progress{}}
	j.assessImage(context.Background(), client, db.Repository(), img)

	require.NotNil(t, saved)
	assert.Equal(t, entityTypeImage, saved.EntityType)
	assert.Equal(t, 7, saved.EntityID)
	assert.Equal(t, 85, saved.QualityScore)
	assert.Equal(t, 90, saved.VisualClarity)
	assert.Equal(t, "Good", saved.Notes)
	db.AIMediaQuality.AssertExpectations(t)
}

func TestAssessImage_SkipsNoFile(t *testing.T) {
	db := mocks.NewDatabase()

	client := ai.NewClient("http://localhost:1", "test-model")
	j := &AIMediaQualityJob{progress: &job.Progress{}}

	j.assessImage(context.Background(), client, db.Repository(), &models.Image{ID: 1})

	db.AIMediaQuality.AssertNotCalled(t, "Upsert")
}

func TestClampScore(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{-10, 0},
		{0, 0},
		{50, 50},
		{100, 100},
		{150, 100},
		{1000, 100},
	}

	for _, tc := range tests {
		result := clampScore(tc.input)
		assert.Equal(t, tc.expected, result, "input: %d", tc.input)
	}
}
