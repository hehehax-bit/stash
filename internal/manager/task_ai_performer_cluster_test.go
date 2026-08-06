package manager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// aiChatCompletionResponse builds a valid chat completion response body with
// the given assistant message content.
func aiChatCompletionResponse(content string) string {
	body, err := json.Marshal(ai.ChatCompletionResponse{
		ID:     "1",
		Object: "chat.completion",
		Model:  "test-model",
		Choices: []ai.ChatCompletionChoice{{
			Index: 0,
			Message: ai.ChatMessage{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: "stop",
		}},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func newAIChatServer(t *testing.T, content string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(aiChatCompletionResponse(content)))
	}))
}

// initClusterInstance installs a Manager with a configured repository and a
// zero-value FFMpeg (whose Generate always fails, so screenshot generation is
// exercised but produces no output).
func initClusterInstance(db *mocks.Database) {
	instance = &Manager{}
	instance.Config = config.InitializeEmpty()
	instance.Repository = db.Repository()
	instance.FFMpeg = &ffmpeg.FFMpeg{}
}

func TestMatchPerformer_JSONParsing_Valid(t *testing.T) {
	server := newAIChatServer(t, `{"present": true, "confidence": 0.85}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{}
	performer := &models.Performer{ID: 1, Name: "Jane Doe"}
	refImage := []byte("fake-image-data")
	candidates := []ai.MultiImage{{Base64: "base64data", MediaType: "image/jpeg"}}

	present, confidence := j.matchPerformer(context.Background(), client, performer, refImage, candidates)

	assert.True(t, present)
	assert.InDelta(t, 0.85, confidence, 0.01)
}

func TestMatchPerformer_JSONParsing_False(t *testing.T) {
	server := newAIChatServer(t, `{"present": false, "confidence": 0.15}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{}

	present, confidence := j.matchPerformer(context.Background(), client, &models.Performer{Name: "Test"}, []byte{}, []ai.MultiImage{})

	assert.False(t, present)
	assert.InDelta(t, 0.15, confidence, 0.01)
}

func TestMatchPerformer_InvalidJSON(t *testing.T) {
	server := newAIChatServer(t, "not json")
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{}

	present, confidence := j.matchPerformer(context.Background(), client, &models.Performer{Name: "Test"}, []byte{}, []ai.MultiImage{})

	assert.False(t, present)
	assert.Equal(t, 0.0, confidence)
}

func TestMatchPerformer_MissingFields(t *testing.T) {
	server := newAIChatServer(t, `{}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{}

	present, confidence := j.matchPerformer(context.Background(), client, &models.Performer{Name: "Test"}, []byte{}, []ai.MultiImage{})

	assert.False(t, present)
	assert.Equal(t, 0.0, confidence)
}

func TestGetPerformers_WithIDs(t *testing.T) {
	db := mocks.NewDatabase()
	db.Performer.On("FindMany", mock.Anything, []int{1, 2}).Return([]*models.Performer{
		{ID: 1, Name: "Performer 1"},
		{ID: 2, Name: "Performer 2"},
	}, nil)

	j := &AIPerformerClusterJob{
		input: AIPerformerClusterInput{PerformerIDs: []int{1, 2}},
	}
	performers, err := j.getPerformers(context.Background(), db.Repository())

	require.NoError(t, err)
	assert.Len(t, performers, 2)
	assert.Equal(t, "Performer 1", performers[0].Name)
	db.Performer.AssertExpectations(t)
}

func TestGetPerformers_AllPerformers(t *testing.T) {
	db := mocks.NewDatabase()
	db.Performer.On("Query", mock.Anything, mock.Anything, mock.Anything).Return([]*models.Performer{
		{ID: 1, Name: "Performer 1"},
		{ID: 2, Name: "Performer 2"},
	}, 2, nil)

	j := &AIPerformerClusterJob{input: AIPerformerClusterInput{}}
	performers, err := j.getPerformers(context.Background(), db.Repository())

	require.NoError(t, err)
	assert.Len(t, performers, 2)
	db.Performer.AssertExpectations(t)
}

func TestScenesWithoutPerformer_OverwriteFalse(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(mocks.SceneQueryResult([]*models.Scene{
		{ID: 1},
	}, 1), nil)

	j := &AIPerformerClusterJob{}
	scenes, err := j.scenesWithoutPerformer(context.Background(), db.Repository(), 1, 0, false)

	require.NoError(t, err)
	assert.Len(t, scenes, 1)
	db.Scene.AssertExpectations(t)
}

func TestScenesWithoutPerformer_OverwriteTrue(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(mocks.SceneQueryResult([]*models.Scene{
		{ID: 1},
	}, 1), nil)

	j := &AIPerformerClusterJob{}
	scenes, err := j.scenesWithoutPerformer(context.Background(), db.Repository(), 1, 0, true)

	require.NoError(t, err)
	assert.Len(t, scenes, 1)
	db.Scene.AssertExpectations(t)
}

func TestScenesWithoutPerformer_MaxScenesZero_NoLimit(t *testing.T) {
	var captured models.SceneQueryOptions
	db := mocks.NewDatabase()
	db.Scene.On("Query", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(models.SceneQueryOptions)
		}).
		Return(mocks.SceneQueryResult(nil, 0), nil)

	j := &AIPerformerClusterJob{}
	_, err := j.scenesWithoutPerformer(context.Background(), db.Repository(), 1, 0, false)

	require.NoError(t, err)
	require.NotNil(t, captured.FindFilter, "find filter should be set")
	assert.True(t, captured.FindFilter.IsGetAll(),
		"maxScenes=0 should mean no limit (PerPageAll), got PerPage=%v", captured.FindFilter.PerPage)
}

func TestScenesWithoutPerformer_MaxScenesPositive_Limited(t *testing.T) {
	var captured models.SceneQueryOptions
	db := mocks.NewDatabase()
	db.Scene.On("Query", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(models.SceneQueryOptions)
		}).
		Return(mocks.SceneQueryResult(nil, 0), nil)

	j := &AIPerformerClusterJob{}
	_, err := j.scenesWithoutPerformer(context.Background(), db.Repository(), 1, 25, false)

	require.NoError(t, err)
	require.NotNil(t, captured.FindFilter)
	assert.False(t, captured.FindFilter.IsGetAll())
	assert.Equal(t, 25, captured.FindFilter.GetPageSize())
}

func TestClusterScenes_SkipsScenesWithoutFile(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(mocks.SceneQueryResult([]*models.Scene{
		{ID: 1},
	}, 1), nil)
	initClusterInstance(db)

	server := newAIChatServer(t, `{"present": true, "confidence": 0.9}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{
		input:    AIPerformerClusterInput{Overwrite: true},
		progress: &job.Progress{},
	}

	count, err := j.clusterScenes(context.Background(), client, db.Repository(), &models.Performer{ID: 1, Name: "Test"}, []byte("ref-image"), 10, 0.7, true)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestClusterScenes_SkipsWhenScreenshotsFail(t *testing.T) {
	db := mocks.NewDatabase()
	scene := &models.Scene{ID: 1}
	scene.Files = models.NewRelatedVideoFiles([]*models.VideoFile{{
		BaseFile: &models.BaseFile{Path: "/nonexistent/video.mp4"},
	}})
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(mocks.SceneQueryResult([]*models.Scene{scene}, 1), nil)
	initClusterInstance(db)

	server := newAIChatServer(t, `{"present": true, "confidence": 0.9}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{
		input:    AIPerformerClusterInput{Overwrite: true},
		progress: &job.Progress{},
	}

	count, err := j.clusterScenes(context.Background(), client, db.Repository(), &models.Performer{ID: 1, Name: "Test"}, []byte("ref-image"), 10, 0.7, true)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestClusterImages_MatchesPerformer(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.jpg")
	require.NoError(t, os.WriteFile(imgPath, []byte("fake-image-bytes"), 0o644))

	img := &models.Image{ID: 1}
	img.Files = models.NewRelatedFiles([]models.File{&models.ImageFile{
		BaseFile: &models.BaseFile{Path: imgPath},
	}})

	db := mocks.NewDatabase()
	db.Image.On("Query", mock.Anything, mock.Anything).Return(mocks.ImageQueryResult([]*models.Image{img}, 1), nil)
	db.Image.On("UpdatePartial", mock.Anything, 1, mock.Anything).Return(nil, nil)
	initClusterInstance(db)

	server := newAIChatServer(t, `{"present": true, "confidence": 0.9}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{
		input:    AIPerformerClusterInput{Overwrite: true},
		progress: &job.Progress{},
	}

	count, err := j.clusterImages(context.Background(), client, db.Repository(), &models.Performer{ID: 1, Name: "Test"}, []byte("ref-image"), 10, 0.7, true)

	require.NoError(t, err)
	assert.Equal(t, 1, count)
	db.Image.AssertExpectations(t)
}

func TestClusterImages_SkipsWhenBelowConfidence(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.jpg")
	require.NoError(t, os.WriteFile(imgPath, []byte("fake-image-bytes"), 0o644))

	img := &models.Image{ID: 1}
	img.Files = models.NewRelatedFiles([]models.File{&models.ImageFile{
		BaseFile: &models.BaseFile{Path: imgPath},
	}})

	db := mocks.NewDatabase()
	db.Image.On("Query", mock.Anything, mock.Anything).Return(mocks.ImageQueryResult([]*models.Image{img}, 1), nil)
	initClusterInstance(db)

	server := newAIChatServer(t, `{"present": true, "confidence": 0.1}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{
		input:    AIPerformerClusterInput{Overwrite: true},
		progress: &job.Progress{},
	}

	count, err := j.clusterImages(context.Background(), client, db.Repository(), &models.Performer{ID: 1, Name: "Test"}, []byte("ref-image"), 10, 0.7, true)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestClusterImages_SkipsWhenNoPrimaryFile(t *testing.T) {
	db := mocks.NewDatabase()
	db.Image.On("Query", mock.Anything, mock.Anything).Return(mocks.ImageQueryResult([]*models.Image{
		{ID: 2},
	}, 1), nil)
	initClusterInstance(db)

	server := newAIChatServer(t, `{"present": true, "confidence": 0.9}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	j := &AIPerformerClusterJob{
		input:    AIPerformerClusterInput{Overwrite: true},
		progress: &job.Progress{},
	}

	count, err := j.clusterImages(context.Background(), client, db.Repository(), &models.Performer{ID: 1, Name: "Test"}, []byte("ref-image"), 10, 0.7, true)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestClusterImages_MaxImagesZero_NoLimit(t *testing.T) {
	var captured models.ImageQueryOptions
	db := mocks.NewDatabase()
	db.Image.On("Query", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(models.ImageQueryOptions)
		}).
		Return(mocks.ImageQueryResult(nil, 0), nil)

	j := &AIPerformerClusterJob{}
	_, err := j.clusterImages(context.Background(), ai.NewClient("http://example.com", "test"), db.Repository(), &models.Performer{ID: 1, Name: "Test"}, []byte("ref-image"), 0, 0.7, true)

	require.NoError(t, err)
	require.NotNil(t, captured.FindFilter, "find filter should be set")
	assert.True(t, captured.FindFilter.IsGetAll(),
		"maxImages=0 should mean no limit (PerPageAll), got PerPage=%v", captured.FindFilter.PerPage)
}

func TestClusterImages_MaxImagesPositive_Limited(t *testing.T) {
	var captured models.ImageQueryOptions
	db := mocks.NewDatabase()
	db.Image.On("Query", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(models.ImageQueryOptions)
		}).
		Return(mocks.ImageQueryResult(nil, 0), nil)

	j := &AIPerformerClusterJob{}
	_, err := j.clusterImages(context.Background(), ai.NewClient("http://example.com", "test"), db.Repository(), &models.Performer{ID: 1, Name: "Test"}, []byte("ref-image"), 25, 0.7, true)

	require.NoError(t, err)
	require.NotNil(t, captured.FindFilter)
	assert.False(t, captured.FindFilter.IsGetAll())
	assert.Equal(t, 25, captured.FindFilter.GetPageSize())
}

func TestExecute_Cancelled(t *testing.T) {
	db := mocks.NewDatabase()
	db.Performer.On("FindMany", mock.Anything, []int{1}).Return([]*models.Performer{
		{ID: 1, Name: "Performer 1"},
	}, nil)
	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	j := &AIPerformerClusterJob{
		input: AIPerformerClusterInput{PerformerIDs: []int{1}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := j.Execute(ctx, nil)
	assert.NoError(t, err)
}
