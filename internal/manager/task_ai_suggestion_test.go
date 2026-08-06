package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestAnalyzeMediaWithVision_JSONExtraction_Valid(t *testing.T) {
	server := newAIChatServer(t, `{"title": "Jane Doe blowjob", "performers": [{"name": "Jane Doe", "gender": "Female", "ethnicity": "Caucasian", "hair_color": "Blonde", "eye_color": "Blue", "details": "Performer in scene"}], "tags": ["blowjob", "oral", "pov"], "details": "Jane Doe performs oral sex in POV style."}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	analysis, err := analyzeMediaWithVision(context.Background(), client, "scene", "test.mp4", images, "")

	require.NoError(t, err)
	assert.Equal(t, "Jane Doe blowjob", analysis.Title)
	assert.Len(t, analysis.Performers, 1)
	assert.Equal(t, "Jane Doe", analysis.Performers[0].Name)
	assert.Equal(t, "Female", analysis.Performers[0].Gender)
	assert.Contains(t, analysis.Tags, "blowjob")
	assert.Contains(t, analysis.Tags, "oral")
	assert.Equal(t, "Jane Doe performs oral sex in POV style.", analysis.Details)
}

func TestAnalyzeMediaWithVision_UnknownPerformers(t *testing.T) {
	server := newAIChatServer(t, `{"title": "Blowjob scene", "performers": [{"name": "Unknown", "gender": "Female", "ethnicity": "Unknown", "hair_color": "Unknown", "eye_color": "Unknown", "details": "Unidentified"}], "tags": ["blowjob"], "details": "A blowjob scene."}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	analysis, err := analyzeMediaWithVision(context.Background(), client, "scene", "test.mp4", images, "")

	require.NoError(t, err)
	assert.Len(t, analysis.Performers, 1)
	// performerNames filters out "unknown"
	assert.Empty(t, performerNames(analysis.Performers))
}

func TestAnalyzeMediaWithVision_EmptyPerformers(t *testing.T) {
	server := newAIChatServer(t, `{"title": "Solo scene", "performers": [], "tags": ["solo", "masturbation"], "details": "Solo female masturbation."}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	analysis, err := analyzeMediaWithVision(context.Background(), client, "scene", "test.mp4", images, "")

	require.NoError(t, err)
	assert.Len(t, analysis.Performers, 0)
	assert.Contains(t, analysis.Tags, "solo")
}

func TestAnalyzeMediaWithVision_InvalidJSON(t *testing.T) {
	server := newAIChatServer(t, "not json")
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	_, err := analyzeMediaWithVision(context.Background(), client, "scene", "test.mp4", images, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing AI response")
}

func TestAnalyzeMediaWithVision_EmptyJSON(t *testing.T) {
	server := newAIChatServer(t, `{}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	analysis, err := analyzeMediaWithVision(context.Background(), client, "scene", "test.mp4", images, "")

	require.NoError(t, err)
	assert.Empty(t, analysis.Performers)
	assert.Empty(t, analysis.Tags)
}

func TestAnalyzeMediaWithVision_CustomContext(t *testing.T) {
	var requestBody ai.ChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(aiChatCompletionResponse(`{"title": "Scene with context", "performers": [], "tags": ["context"], "details": "Context used"}`)))
	}))
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	images := []ai.MultiImage{{Base64: "base64", MediaType: "image/jpeg"}}

	analysis, err := analyzeMediaWithVision(context.Background(), client, "scene", "test.mp4", images, "User provided context about performers")

	require.NoError(t, err)
	assert.Equal(t, "Context used", analysis.Details)

	require.Len(t, requestBody.Messages, 2)
	content, ok := requestBody.Messages[1].Content.([]interface{})
	require.True(t, ok)
	require.Len(t, content, 2)
	textPart := content[0].(map[string]interface{})
	assert.Contains(t, textPart["text"], "User provided context about performers")
}

func TestPendingEntityIDs(t *testing.T) {
	db := mocks.NewDatabase()
	db.AISuggestion.On("FindPendingByEntityType", mock.Anything, "scene").Return([]*models.AISuggestion{
		{EntityID: 1},
		{EntityID: 3},
	}, nil)

	j := &AISuggestionJob{}
	pending := j.pendingEntityIDs(context.Background(), db.Repository(), "scene")

	require.NotNil(t, pending)
	assert.True(t, pending[1])
	assert.True(t, pending[3])
	assert.False(t, pending[2])
}

func TestSuggestScenes_SkipsPending(t *testing.T) {
	db := mocks.NewDatabase()
	db.AISuggestion.On("FindPendingByEntityType", mock.Anything, "scene").Return([]*models.AISuggestion{
		{EntityID: 1},
	}, nil)
	db.Tag.On("FindByName", mock.Anything, mock.Anything, true).Return(&models.Tag{ID: 999}, nil)
	db.Scene.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(mocks.SceneQueryResult([]*models.Scene{
		{ID: 1},
	}, 1), nil)
	initClusterInstance(db)

	client := ai.NewClient("http://localhost:1", "test-model")
	j := &AISuggestionJob{progress: &job.Progress{}}

	err := j.suggestScenes(context.Background(), client, db.Repository(), 10)

	require.NoError(t, err)
	db.AISuggestion.AssertNotCalled(t, "Create")
}

func TestSuggestScene_NoScreenshots(t *testing.T) {
	db := mocks.NewDatabase()
	initClusterInstance(db)

	client := ai.NewClient("http://localhost:1", "test-model")
	j := &AISuggestionJob{progress: &job.Progress{}}

	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles([]*models.VideoFile{{
		BaseFile: &models.BaseFile{Path: "/nonexistent/video.mp4"},
		Duration: 120,
	}})

	j.suggestScene(context.Background(), client, db.Repository(), s)

	db.AISuggestion.AssertNotCalled(t, "Create")
}

func TestSuggestImage_SavesSuggestion(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.jpg")
	require.NoError(t, os.WriteFile(imgPath, []byte("fake-image-bytes"), 0o644))

	img := &models.Image{ID: 7, Path: imgPath}
	img.Files = models.NewRelatedFiles([]models.File{&models.ImageFile{
		BaseFile: &models.BaseFile{Path: imgPath},
	}})

	server := newAIChatServer(t, `{"title": "Test Image", "performers": [], "tags": ["test"], "details": "Test details"}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")

	db := mocks.NewDatabase()
	var saved *models.AISuggestion
	db.AISuggestion.On("Create", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.AISuggestion)
	})

	j := &AISuggestionJob{progress: &job.Progress{}}
	j.suggestImage(context.Background(), client, db.Repository(), img)

	require.NotNil(t, saved)
	assert.Equal(t, entityTypeImage, saved.EntityType)
	assert.Equal(t, 7, saved.EntityID)
	assert.Equal(t, "Test Image", saved.Title)
	assert.Equal(t, "Test details", saved.Details)
	assert.Equal(t, []string{"test"}, saved.Tags)
	db.AISuggestion.AssertExpectations(t)
}

func TestSuggestImage_SkipsNoFile(t *testing.T) {
	db := mocks.NewDatabase()

	client := ai.NewClient("http://localhost:1", "test-model")
	j := &AISuggestionJob{progress: &job.Progress{}}

	img := &models.Image{ID: 1}

	j.suggestImage(context.Background(), client, db.Repository(), img)

	db.AISuggestion.AssertNotCalled(t, "Create")
}
