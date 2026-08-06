package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMemoryStore struct {
	memories []*models.AIMemory
}

func (f *fakeMemoryStore) FindAll(ctx context.Context) ([]*models.AIMemory, error) {
	return f.memories, nil
}

func (f *fakeMemoryStore) FindByKey(ctx context.Context, key string) (*models.AIMemory, error) {
	for _, m := range f.memories {
		if m.Key == key {
			return m, nil
		}
	}
	return nil, models.ErrNotFound
}

func (f *fakeMemoryStore) Set(ctx context.Context, key string, value string) error {
	return nil
}

func (f *fakeMemoryStore) Delete(ctx context.Context, key string) error {
	return nil
}

func (f *fakeMemoryStore) DeleteAll(ctx context.Context) error {
	return nil
}

func newTestChatService(t *testing.T, server *httptest.Server) *ChatService {
	t.Helper()

	repo := models.Repository{
		Memory: &fakeMemoryStore{},
	}

	s := newChatServiceWithConfig(NewClient(server.URL, "test-model"), repo, "system prompt", ToolConfig{})
	s.SetEnabled(true)
	return s
}

// Regression test: some OpenAI-compatible servers (LM Studio, llama.cpp,
// vLLM) report finish_reason "stop" alongside tool_calls. The chat loop must
// still execute the tool calls and continue, rather than returning early.
func TestSendMessageToolCallsWithStopFinishReason(t *testing.T) {
	var requests int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		requests++

		w.Header().Set("Content-Type", "application/json")

		switch requests {
		case 1:
			assert.NotEmpty(t, req.Tools)
			assert.Equal(t, defaultMaxTokens, req.MaxTokens)

			resp := ChatCompletionResponse{
				ID:     "1",
				Object: "chat.completion",
				Model:  "test-model",
				Choices: []ChatCompletionChoice{{
					Index: 0,
					Message: ChatMessage{
						Role: "assistant",
						ToolCalls: []ToolCall{{
							ID:   "call_1",
							Type: "function",
							Function: ToolCallFunction{
								Name:      "list_memories",
								Arguments: "{}",
							},
						}},
					},
					FinishReason: "stop",
				}},
			}
			assert.NoError(t, json.NewEncoder(w).Encode(resp))
		default:
			var gotAssistant, gotTool bool
			for _, m := range req.Messages {
				if m.Role == "assistant" && len(m.ToolCalls) > 0 {
					gotAssistant = true
				}
				if m.Role == "tool" && m.ToolCallID == "call_1" && m.Content == "No memories saved yet. Use `remember` to save preferences." {
					gotTool = true
				}
			}
			assert.True(t, gotAssistant, "expected assistant tool_calls message in follow-up request")
			assert.True(t, gotTool, "expected tool result message in follow-up request")

			resp := ChatCompletionResponse{
				ID:     "2",
				Object: "chat.completion",
				Model:  "test-model",
				Choices: []ChatCompletionChoice{{
					Index:        0,
					Message:      ChatMessage{Role: "assistant", Content: "No memories saved yet."},
					FinishReason: "stop",
				}},
			}
			assert.NoError(t, json.NewEncoder(w).Encode(resp))
		}
	}))
	defer server.Close()

	s := newTestChatService(t, server)

	msg, err := s.SendMessage(context.Background(), nil, "hi")
	require.NoError(t, err)
	assert.Equal(t, "No memories saved yet.", msg.Content)
	assert.Equal(t, 2, requests)
}

func TestSendMessageEmptyResponseErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := ChatCompletionResponse{
			ID:     "1",
			Object: "chat.completion",
			Model:  "test-model",
			Choices: []ChatCompletionChoice{{
				Index:        0,
				Message:      ChatMessage{Role: "assistant"},
				FinishReason: "stop",
			}},
		}
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	s := newTestChatService(t, server)

	_, err := s.SendMessage(context.Background(), nil, "hi")
	assert.ErrorContains(t, err, "empty response")
}

func TestSendMessageTruncatedResponseErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := ChatCompletionResponse{
			ID:     "1",
			Object: "chat.completion",
			Model:  "test-model",
			Choices: []ChatCompletionChoice{{
				Index:        0,
				Message:      ChatMessage{Role: "assistant", Content: "partial answer"},
				FinishReason: "length",
			}},
		}
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	s := newTestChatService(t, server)

	_, err := s.SendMessage(context.Background(), nil, "hi")
	assert.ErrorContains(t, err, "truncated")
}
