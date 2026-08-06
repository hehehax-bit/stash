package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newEmbeddingServer returns an httptest server that answers embedding
// requests with a fixed embedding vector.
func newEmbeddingServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ai.EmbeddingRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		w.Header().Set("Content-Type", "application/json")
		resp := ai.EmbeddingResponse{
			Object: "list",
			Data: []ai.EmbeddingData{{
				Object:    "embedding",
				Embedding: []float32{0.1, 0.2, 0.3},
				Index:     0,
			}},
			Model: "nomic-embed-text",
			Usage: ai.EmbeddingUsage{PromptTokens: 5, TotalTokens: 5},
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
}

func newEmptyQueryResolver() *queryResolver {
	return &queryResolver{Resolver: &Resolver{repository: mocks.NewDatabase().Repository()}}
}

type txnMarkerKey struct{}

type markerTxnManager struct{}

func (markerTxnManager) Begin(ctx context.Context, writable bool) (context.Context, error) {
	return context.WithValue(ctx, txnMarkerKey{}, true), nil
}

func (markerTxnManager) WithDatabase(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (markerTxnManager) Commit(ctx context.Context) error {
	return nil
}

func (markerTxnManager) Rollback(ctx context.Context) error {
	return nil
}

func (markerTxnManager) IsLocked(err error) bool {
	return false
}

// TestSemanticSearch_WithReadTxn verifies that SemanticSearch runs the
// SearchSimilar query inside a read transaction. Previously the query was
// executed against the raw request context, which has no database connection,
// so SearchSimilar errored for every entity type and the resolver silently
// swallowed the errors, yielding "No results found" for all queries.
func TestSemanticSearch_WithReadTxn(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(ai.NewClient("", "test-model"), models.Repository{}, "")
	mgr.AIService.SetEnabled(true)
	mgr.Config.SetAIEnabled(true)
	mgr.Config.SetAIBaseURL(server.URL)
	mgr.Config.SetAIEmbeddingModel("nomic-embed-text")

	db := mocks.NewDatabase()
	withinTxn := false
	db.Embedding.On("SearchSimilar", mock.Anything, "scene", "nomic-embed-text", mock.Anything, 20).
		Return([]models.SimilarityResult{{EntityID: 1, Score: 0.9}}, nil).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			withinTxn = ctx.Value(txnMarkerKey{}) == true
		})
	db.Scene.On("Find", mock.Anything, 1).Return((*models.Scene)(nil), nil)

	repo := db.Repository()
	repo.TxnManager = markerTxnManager{}

	ctx := context.Background()
	r := &queryResolver{Resolver: &Resolver{repository: repo}}
	input := SemanticSearchInput{
		Query:       "test query",
		EntityTypes: []string{"scene"},
	}

	results, err := r.SemanticSearch(ctx, input)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, withinTxn, "SearchSimilar must be executed within a read transaction")
}

func TestSemanticSearch_Defaults(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(ai.NewClient("", "test-model"), models.Repository{}, "")
	mgr.AIService.SetEnabled(true)
	mgr.Config.SetAIEnabled(true)
	mgr.Config.SetAIBaseURL(server.URL)
	mgr.Config.SetAIEmbeddingModel("nomic-embed-text")

	db := mocks.NewDatabase()
	db.Embedding.On("SearchSimilar", mock.Anything, mock.Anything, "nomic-embed-text", mock.Anything, 20).Return([]models.SimilarityResult{}, nil)

	ctx := context.Background()
	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	input := SemanticSearchInput{Query: "test query"}

	results, err := r.SemanticSearch(ctx, input)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSemanticSearch_CustomModel(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(ai.NewClient("", "test-model"), models.Repository{}, "")
	mgr.AIService.SetEnabled(true)
	mgr.Config.SetAIEnabled(true)
	mgr.Config.SetAIBaseURL(server.URL)
	mgr.Config.SetAIEmbeddingModel("nomic-embed-text")

	db := mocks.NewDatabase()
	db.Embedding.On("SearchSimilar", mock.Anything, mock.Anything, "custom-model", mock.Anything, 20).Return([]models.SimilarityResult{}, nil)

	ctx := context.Background()
	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	input := SemanticSearchInput{
		Query: "test query",
		Model: "custom-model",
	}

	_, err := r.SemanticSearch(ctx, input)

	require.NoError(t, err)
}

func TestSemanticSearch_CustomLimit(t *testing.T) {
	server := newEmbeddingServer(t)
	defer server.Close()

	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(ai.NewClient("", "test-model"), models.Repository{}, "")
	mgr.AIService.SetEnabled(true)
	mgr.Config.SetAIEnabled(true)
	mgr.Config.SetAIBaseURL(server.URL)
	mgr.Config.SetAIEmbeddingModel("nomic-embed-text")

	db := mocks.NewDatabase()
	db.Embedding.On("SearchSimilar", mock.Anything, "scene", "nomic-embed-text", mock.Anything, 5).Return([]models.SimilarityResult{}, nil).Once()
	db.Embedding.On("SearchSimilar", mock.Anything, "performer", "nomic-embed-text", mock.Anything, 5).Return([]models.SimilarityResult{}, nil).Once()

	ctx := context.Background()
	r := &queryResolver{Resolver: &Resolver{repository: db.Repository()}}
	limit := 5
	input := SemanticSearchInput{
		Query:       "test query",
		EntityTypes: []string{"scene", "performer"},
		Limit:       &limit,
	}

	_, err := r.SemanticSearch(ctx, input)

	require.NoError(t, err)
}

func TestSemanticSearch_MissingConfig(t *testing.T) {
	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(ai.NewClient("", "test-model"), models.Repository{}, "")
	mgr.AIService.SetEnabled(true)
	mgr.Config.SetAIEnabled(true)
	mgr.Config.SetAIEmbeddingModel("nomic-embed-text")
	mgr.Config.SetAIBaseURL("")

	ctx := context.Background()
	r := newEmptyQueryResolver()
	input := SemanticSearchInput{Query: "test"}

	_, err := r.SemanticSearch(ctx, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI base URL is not configured")
}

func TestSemanticSearch_NoEmbeddingModel(t *testing.T) {
	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(ai.NewClient("", "test-model"), models.Repository{}, "")
	mgr.AIService.SetEnabled(true)
	mgr.Config.SetAIEnabled(true)
	mgr.Config.SetAIBaseURL("http://localhost:11434")
	mgr.Config.SetAIModel("")
	mgr.Config.SetAIEmbeddingModel("")

	ctx := context.Background()
	r := newEmptyQueryResolver()
	input := SemanticSearchInput{Query: "test"}

	_, err := r.SemanticSearch(ctx, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no embedding model configured")
}

func TestSemanticSearch_AIDisabled(t *testing.T) {
	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(ai.NewClient("", "test-model"), models.Repository{}, "")
	mgr.AIService.SetEnabled(false)
	mgr.Config.SetAIEnabled(false)

	ctx := context.Background()
	r := newEmptyQueryResolver()
	input := SemanticSearchInput{Query: "test"}

	_, err := r.SemanticSearch(ctx, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI is not enabled")
}

func TestSemanticSearch_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	mgr := initTestManager()
	mgr.AIService = ai.NewChatService(ai.NewClient("", "test-model"), models.Repository{}, "")
	mgr.AIService.SetEnabled(true)
	mgr.Config.SetAIEnabled(true)
	mgr.Config.SetAIBaseURL(server.URL)
	mgr.Config.SetAIEmbeddingModel("nomic-embed-text")

	ctx := context.Background()
	r := newEmptyQueryResolver()
	input := SemanticSearchInput{Query: "test"}

	_, err := r.SemanticSearch(ctx, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding API error")
}
