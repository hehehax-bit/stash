package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newChatServer returns an httptest server that responds to chat completion
// requests with a fixed assistant message. All request bodies are captured so
// tests can assert on the exact request shape (the resolver may make additional
// internal calls, e.g. title generation).
func newChatServer(t *testing.T, responseContent string) (*httptest.Server, *[]ai.ChatCompletionRequest) {
	t.Helper()

	var requests []ai.ChatCompletionRequest
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ai.ChatCompletionRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		resp := ai.ChatCompletionResponse{
			ID:     "1",
			Object: "chat.completion",
			Model:  "test-model",
			Choices: []ai.ChatCompletionChoice{{
				Index: 0,
				Message: ai.ChatMessage{
					Role:    "assistant",
					Content: responseContent,
				},
				FinishReason: "stop",
			}},
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))

	return server, &requests
}

// initTestManager installs a fresh Manager as the package singleton so that
// code calling manager.GetInstance() works in tests. It returns the Manager.
func initTestManager() *manager.Manager {
	mgr := &manager.Manager{}
	manager.SetInstanceForTest(mgr)
	mgr.Config = config.InitializeEmpty()
	return mgr
}

// newTestMutationResolver wires a manager with a mocked repository and an AI
// chat service pointed at the provided server.
func newTestMutationResolver(t *testing.T, server *httptest.Server) *mutationResolver {
	t.Helper()
	return newTestMutationResolverWithDB(t, server, mocks.NewDatabase())
}

func newTestMutationResolverWithDB(t *testing.T, server *httptest.Server, db *mocks.Database) *mutationResolver {
	t.Helper()

	mgr := initTestManager()

	mgr.Repository = db.Repository()

	mgr.AIService = ai.NewChatService(ai.NewClient(server.URL, "test-model"), mgr.Repository, "test system prompt")
	mgr.AIService.SetEnabled(true)

	db.Memory.On("FindAll", mock.Anything).Return([]*models.AIMemory{}, nil)

	db.AI.On("CreateSession", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		sess := args.Get(1).(*models.AIChatSession)
		sess.ID = "test-session-1"
	})
	db.AI.On("CreateMessage", mock.Anything, mock.Anything).Return(nil)
	db.AI.On("Touch", mock.Anything, mock.Anything).Return(nil)
	db.AI.On("FindBySessionID", mock.Anything, mock.Anything).Return([]*models.AIChatMessage{}, nil)
	db.AI.On("SetTitle", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	return &mutationResolver{Resolver: &Resolver{repository: db.Repository()}}
}

func TestAiChatSend_NewSession(t *testing.T) {
	server, requests := newChatServer(t, "Hi there!")
	defer server.Close()

	ctx := context.Background()
	r := newTestMutationResolver(t, server)
	input := AIChatSendInput{Message: "Hello"}

	result, err := r.AiChatSend(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "assistant", result.Role)
	assert.Equal(t, "Hi there!", result.Content)
	assert.Equal(t, "test-session-1", result.SessionID)

	// the first request is the main conversation; assert its shape
	require.GreaterOrEqual(t, len(*requests), 1)
	mainReq := (*requests)[0]
	assert.Equal(t, "test-model", mainReq.Model)
	assert.Len(t, mainReq.Messages, 2)
	assert.Equal(t, "system", mainReq.Messages[0].Role)
	assert.Equal(t, "user", mainReq.Messages[1].Role)
	assert.Equal(t, "Hello", mainReq.Messages[1].Content)
}

func TestAiChatSend_WithExistingSession(t *testing.T) {
	server, _ := newChatServer(t, "Follow up response")
	defer server.Close()

	ctx := context.Background()
	r := newTestMutationResolver(t, server)
	input := AIChatSendInput{
		SessionID: stringPtr("test-session-1"),
		Message:   "Follow up",
	}

	result, err := r.AiChatSend(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Follow up response", result.Content)
}

func TestAiChatSend_WithImage(t *testing.T) {
	server, requests := newChatServer(t, "I see the image")
	defer server.Close()

	ctx := context.Background()
	r := newTestMutationResolver(t, server)
	imageData := "data:image/png;base64,fakebase64"
	input := AIChatSendInput{
		Message: "What's in this image?",
		Image:   &imageData,
	}

	result, err := r.AiChatSend(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "I see the image", result.Content)

	require.GreaterOrEqual(t, len(*requests), 1)
	mainReq := (*requests)[0]
	require.Len(t, mainReq.Messages, 2)
	content, ok := mainReq.Messages[1].Content.([]interface{})
	require.True(t, ok)
	assert.Len(t, content, 2)
	part0 := content[0].(map[string]interface{})
	part1 := content[1].(map[string]interface{})
	assert.Equal(t, "text", part0["type"])
	assert.Equal(t, "image_url", part1["type"])
}

func TestAiChatSend_DisabledAI(t *testing.T) {
	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(nil, models.Repository{}, "test system prompt")
	mgr.AIService.SetEnabled(false)

	ctx := context.Background()
	r := &mutationResolver{}
	input := AIChatSendInput{Message: "Hello"}

	_, err := r.AiChatSend(ctx, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI is not enabled")
}

func TestAiChatSend_EmptyMessageWithImage(t *testing.T) {
	server, requests := newChatServer(t, "Image received")
	defer server.Close()

	ctx := context.Background()

	mgr := initTestManager()
	db := mocks.NewDatabase()
	mgr.Repository = db.Repository()

	mgr.AIService = ai.NewChatService(ai.NewClient(server.URL, "test-model"), mgr.Repository, "test system prompt")
	mgr.AIService.SetEnabled(true)

	var created []*models.AIChatMessage
	db.Memory.On("FindAll", mock.Anything).Return([]*models.AIMemory{}, nil)
	db.AI.On("CreateSession", mock.Anything, mock.Anything).Return(nil)
	db.AI.On("CreateMessage", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		created = append(created, args.Get(1).(*models.AIChatMessage))
	})
	db.AI.On("Touch", mock.Anything, mock.Anything).Return(nil)
	db.AI.On("FindBySessionID", mock.Anything, mock.Anything).Return([]*models.AIChatMessage{}, nil)
	db.AI.On("SetTitle", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	r := &mutationResolver{Resolver: &Resolver{repository: db.Repository()}}
	imageData := "data:image/png;base64,fakebase64"
	input := AIChatSendInput{
		Message: "",
		Image:   &imageData,
	}

	result, err := r.AiChatSend(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Image received", result.Content)

	// the fallback text is stored in the DB, not sent to the AI
	require.Len(t, created, 2)
	assert.Equal(t, "user", created[0].Role)
	assert.Equal(t, "Sent an image", created[0].Content)
	assert.Equal(t, "assistant", created[1].Role)

	require.GreaterOrEqual(t, len(*requests), 1)
	mainReq := (*requests)[0]
	require.Len(t, mainReq.Messages, 2)
	content, ok := mainReq.Messages[1].Content.([]interface{})
	require.True(t, ok)
	require.Len(t, content, 1)
	part0 := content[0].(map[string]interface{})
	assert.Equal(t, "image_url", part0["type"])
}

func TestAiChatClear(t *testing.T) {
	db := mocks.NewDatabase()
	db.AI.On("DeleteBySessionID", mock.Anything, "test-session").Return(nil)

	r := &mutationResolver{Resolver: &Resolver{repository: db.Repository()}}
	result, err := r.AiChatClear(context.Background(), "test-session")

	require.NoError(t, err)
	assert.True(t, result)
}

func TestAiChatDeleteMessage(t *testing.T) {
	db := mocks.NewDatabase()
	db.AI.On("DeleteByID", mock.Anything, "msg-1").Return(nil)

	r := &mutationResolver{Resolver: &Resolver{repository: db.Repository()}}
	result, err := r.AiChatDeleteMessage(context.Background(), "msg-1")

	require.NoError(t, err)
	assert.True(t, result)
}

func TestAiChatSessions(t *testing.T) {
	db := mocks.NewDatabase()
	db.AI.On("FindAll", mock.Anything).Return([]*models.AIChatSession{{ID: "sess-1"}}, nil)

	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	sessions, err := r.AiChatSessions(context.Background())

	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "sess-1", sessions[0].ID)
}

func TestAiChatHistory(t *testing.T) {
	db := mocks.NewDatabase()
	db.AI.On("FindBySessionID", mock.Anything, "sess-1").Return([]*models.AIChatMessage{
		{ID: "msg-1", SessionID: "sess-1", Role: "user", Content: "Hello"},
		{ID: "msg-2", SessionID: "sess-1", Role: "assistant", Content: "Hi"},
	}, nil)

	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	messages, err := r.AiChatHistory(context.Background(), "sess-1")

	require.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)
}

func stringPtr(s string) *string {
	return &s
}

func TestAiChatSend_LibraryContext(t *testing.T) {
	// embedding server
	embedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2,0.3],"index":0}],"model":"embed"}`)
	}))
	defer embedServer.Close()

	chatServer, requests := newChatServer(t, "Here is the answer.")
	defer chatServer.Close()

	ctx := context.Background()
	db := mocks.NewDatabase()
	r := newTestMutationResolverWithDB(t, chatServer, db)

	// configure embedding model/base URL on the test manager
	mgr := manager.GetInstance()
	mgr.Config.SetAIBaseURL(embedServer.URL)
	mgr.Config.SetAIEmbeddingModel("embed")

	// mock the repository lookups the library context performs
	db.Embedding.On("SearchSimilar", mock.Anything, "scene", "embed", mock.Anything, 5).
		Return([]models.SimilarityResult{{EntityID: 1, Score: 0.9}}, nil)
	db.Scene.On("Find", mock.Anything, 1).
		Return(&models.Scene{ID: 1, Title: "Jane Doe blowjob", Details: "Jane Doe performs."}, nil)
	db.AISceneAudio.On("FindBySceneID", mock.Anything, 1).
		Return(&models.AISceneAudio{SceneID: 1, Summary: "A summary", Transcript: "The transcript"}, nil)

	input := AIChatSendInput{Message: "What happens in the blowjob scene?", UseLibraryContext: boolPtr(true)}
	result, err := r.AiChatSend(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Here is the answer.", result.Content)

	// the main chat request must carry the library context message
	require.GreaterOrEqual(t, len(*requests), 1)
	mainReq := (*requests)[0]
	require.GreaterOrEqual(t, len(mainReq.Messages), 3)
	// ChatService prepends its own system prompt, the library context follows
	assert.Equal(t, "system", mainReq.Messages[0].Role)
	assert.Equal(t, "system", mainReq.Messages[1].Role)
	assert.Contains(t, mainReq.Messages[1].Content, "Library context")
	assert.Contains(t, mainReq.Messages[1].Content, "Jane Doe blowjob")
	assert.Contains(t, mainReq.Messages[1].Content, "A summary")
	assert.Equal(t, "user", mainReq.Messages[2].Role)
}

func TestAiChatSend_LibraryContextDisabled(t *testing.T) {
	chatServer, requests := newChatServer(t, "Hi!")
	defer chatServer.Close()

	ctx := context.Background()
	r := newTestMutationResolver(t, chatServer)
	mgr := manager.GetInstance()
	mgr.Config.SetAIBaseURL("http://example.com")
	mgr.Config.SetAIEmbeddingModel("embed")

	input := AIChatSendInput{Message: "Hello", UseLibraryContext: boolPtr(false)}
	result, err := r.AiChatSend(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.GreaterOrEqual(t, len(*requests), 1)
	mainReq := (*requests)[0]
	assert.Len(t, mainReq.Messages, 2, "no library context message when disabled")
}

func boolPtr(b bool) *bool {
	return &b
}
