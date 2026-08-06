package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientChatCompletionStream(t *testing.T) {
	chunks := []string{"Hello", ", ", "world", "!"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		if !req.Stream {
			t.Errorf("expected stream to be true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		for i, c := range chunks {
			payload, _ := json.Marshal(map[string]any{
				"id":      "1",
				"object":  "chat.completion.chunk",
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": c}, "finish_reason": nil}},
			})
			fmt.Fprintf(w, "data: %s\n\n", payload)
			if i == 0 {
				w.(http.Flusher).Flush()
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model")

	var got []string
	text, err := client.ChatCompletionStream(context.Background(), ChatCompletionRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	}, func(chunk string) {
		got = append(got, chunk)
	})

	assert.NoError(t, err)
	assert.Equal(t, chunks, got)
	assert.Equal(t, "Hello, world!", text)
}

func TestClientChatCompletionStreamAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"bad"}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model")

	_, err := client.ChatCompletionStream(context.Background(), ChatCompletionRequest{}, func(string) {})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
}

func TestClientChatCompletionStreamServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"error":{"message":"stream not supported","type":"invalid_request_error"}}`+"\n\n")
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model")

	_, err := client.ChatCompletionStream(context.Background(), ChatCompletionRequest{}, func(string) {})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream not supported")
}

func TestClientChatCompletionStreamCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model")

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := client.ChatCompletionStream(ctx, ChatCompletionRequest{}, func(string) {})
		errCh <- err
	}()
	cancel()

	select {
	case err := <-errCh:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("expected stream to be aborted")
	}
}

func TestClientEmbeddingImageOllama(t *testing.T) {
	var gotReq ollamaEmbedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embed", r.URL.Path)
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"clip","embeddings":[[0.1,0.2,0.3]]}`)
	}))
	defer server.Close()

	client := NewEmbeddingClient(server.URL, "clip", "ollama")

	vec, err := client.EmbeddingImage(context.Background(), "QUJD", "image/jpeg")
	assert.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, vec)
	assert.Equal(t, "clip", gotReq.Model)
	assert.Len(t, gotReq.Input, 1)
	assert.Equal(t, "image", gotReq.Input[0].Type)
	assert.Equal(t, "QUJD", gotReq.Input[0].Data)
}

func TestClientEmbeddingImageOpenAICompat(t *testing.T) {
	var gotReq EmbeddingRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","data":[{"object":"embedding","embedding":[0.5,0.6,0.7],"index":0}]}`)
	}))
	defer server.Close()

	client := NewEmbeddingClient(server.URL, "clip-model", "lmstudio")

	vec, err := client.EmbeddingImage(context.Background(), "QUJD", "image/png")
	assert.NoError(t, err)
	assert.Equal(t, []float32{0.5, 0.6, 0.7}, vec)
	assert.Equal(t, "clip-model", gotReq.Model)
	assert.Equal(t, "data:image/png;base64,QUJD", gotReq.Input)
}

func TestClientEmbeddingOllama(t *testing.T) {
	var gotReq ollamaEmbedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embed", r.URL.Path)
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"mxbai","embeddings":[[0.1,0.2,0.3]]}`)
	}))
	defer server.Close()

	client := NewEmbeddingClient(server.URL, "mxbai", "ollama")

	vec, err := client.Embedding(context.Background(), "some text")
	assert.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, vec)
	assert.Equal(t, "mxbai", gotReq.Model)
	assert.Len(t, gotReq.Input, 1)
	assert.Equal(t, "text", gotReq.Input[0].Type)
	assert.Equal(t, "some text", gotReq.Input[0].Text)
}

func TestClientEmbeddingOpenAICompat(t *testing.T) {
	var gotReq EmbeddingRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","data":[{"object":"embedding","embedding":[0.5,0.6,0.7],"index":0}]}`)
	}))
	defer server.Close()

	client := NewEmbeddingClient(server.URL, "mxbai", "lmstudio")

	vec, err := client.Embedding(context.Background(), "some text")
	assert.NoError(t, err)
	assert.Equal(t, []float32{0.5, 0.6, 0.7}, vec)
	assert.Equal(t, "mxbai", gotReq.Model)
	assert.Equal(t, "some text", gotReq.Input)
}

func TestClientEmbeddingImageOllamaError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"no image input"}`)
	}))
	defer server.Close()

	client := NewEmbeddingClient(server.URL, "clip", "ollama")

	_, err := client.EmbeddingImage(context.Background(), "QUJD", "image/jpeg")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
}

func newTranscribeServer(t *testing.T, status int, contentType, body string) (*httptest.Server, *http.Request) {
	t.Helper()

	var lastReq http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastReq = *r.Clone(r.Context())
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)

	return server, &lastReq
}

func TestClientTranscribeOpenAIJSON(t *testing.T) {
	server, lastReq := newTranscribeServer(t, http.StatusOK, "application/json", `{"text":"Hello there"}`)

	client := NewClient(server.URL, "whisper-1")
	text, err := client.Transcribe(context.Background(), "whisper-1", "audio.wav", []byte("RIFF...."))

	assert.NoError(t, err)
	assert.Equal(t, "Hello there", text)
	assert.Equal(t, "/audio/transcriptions", lastReq.URL.Path)
	assert.True(t, strings.HasPrefix(lastReq.Header.Get("Content-Type"), "multipart/form-data"))
}

func TestClientTranscribePlainText(t *testing.T) {
	server, _ := newTranscribeServer(t, http.StatusOK, "text/plain", "plain transcript body")

	client := NewClient(server.URL, "whisper-1")
	text, err := client.Transcribe(context.Background(), "whisper-1", "audio.wav", []byte("RIFF...."))

	assert.NoError(t, err)
	assert.Equal(t, "plain transcript body", text)
}

func TestClientTranscribeVerboseJSON(t *testing.T) {
	server, _ := newTranscribeServer(t, http.StatusOK, "application/json",
		`{"task":"transcribe","language":"en","duration":2.0,"text":{"text":"verbose transcript","language":"en"}}`)

	client := NewClient(server.URL, "whisper-1")
	text, err := client.Transcribe(context.Background(), "whisper-1", "audio.wav", []byte("RIFF...."))

	assert.NoError(t, err)
	assert.Equal(t, "verbose transcript", text)
}

func TestClientTranscribeEmptyText(t *testing.T) {
	server, _ := newTranscribeServer(t, http.StatusOK, "application/json", `{"text":""}`)

	client := NewClient(server.URL, "whisper-1")
	_, err := client.Transcribe(context.Background(), "whisper-1", "audio.wav", []byte("RIFF...."))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no text")
}

func TestClientTranscribeAPIError(t *testing.T) {
	server, _ := newTranscribeServer(t, http.StatusBadRequest, "text/plain", "bad request body")

	client := NewClient(server.URL, "whisper-1")
	_, err := client.Transcribe(context.Background(), "whisper-1", "audio.wav", []byte("RIFF...."))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "bad request body")
}

func TestClientTranscribeCustomPath(t *testing.T) {
	server, lastReq := newTranscribeServer(t, http.StatusOK, "text/plain", "custom path transcript")

	client := NewClient(server.URL, "whisper-1")
	client.SetTranscriptionPath("/inference")
	text, err := client.Transcribe(context.Background(), "whisper-1", "audio.wav", []byte("RIFF...."))

	assert.NoError(t, err)
	assert.Equal(t, "custom path transcript", text)
	assert.Equal(t, "/inference", lastReq.URL.Path)
}

func TestClientTranscribeRequestOptions(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parsing multipart form: %v", err)
		}
		gotBody = r.FormValue("body") + "|" + r.FormValue("language") + "|" + r.FormValue("prompt") + "|" + r.FormValue("response_format")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"text":"with options"}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	result, err := client.TranscribeResult(context.Background(), TranscribeRequest{
		Filename:       "custom.wav",
		Audio:          []byte("RIFF...."),
		Language:       "de",
		Prompt:         "context words",
		ResponseFormat: "text",
	})

	assert.NoError(t, err)
	assert.Equal(t, "with options", result.Text)
	assert.Equal(t, "|de|context words|text", gotBody)
}

func TestClientListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","data":[{"id":"model-a"},{"id":"model-b"}]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "model-a")
	models, err := client.ListModels(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, []string{"model-a", "model-b"}, models)
}

func TestClientListModelsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","data":[]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "model-a")
	models, err := client.ListModels(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, models)
}

func TestClientListModelsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "boom")
	}))
	defer server.Close()

	client := NewClient(server.URL, "model-a")
	_, err := client.ListModels(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestClientListModelsMalformed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not json`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "model-a")
	_, err := client.ListModels(context.Background())

	assert.Error(t, err)
}

func TestClientTranscribeSegments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"task":"transcribe","language":"en","duration":2.0,"text":"hello world","segments":[{"id":0,"start":0.0,"end":1.0,"text":"hello"},{"id":1,"start":1.0,"end":2.0,"text":"world"}]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "whisper-1")
	result, err := client.TranscribeResult(context.Background(), TranscribeRequest{
		Filename:       "audio.wav",
		Audio:          []byte("RIFF...."),
		ResponseFormat: "verbose_json",
	})

	assert.NoError(t, err)
	assert.Equal(t, "hello world", result.Text)
	require.Len(t, result.Segments, 2)
	assert.Equal(t, "hello", result.Segments[0].Text)
	assert.InDelta(t, 0.0, result.Segments[0].Start, 0.0001)
	assert.InDelta(t, 2.0, result.Segments[1].End, 0.0001)
}

func newEmbeddingServer(t *testing.T, vec []float32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"object": "list",
			"data":   []map[string]any{{"object": "embedding", "embedding": vec, "index": 0}},
			"model":  "embed",
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
}

func TestClientTranscribeSegments_Nanoseconds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"task":"transcribe","language":"en","duration":10.0,"text":"hello world","segments":[{"id":0,"start":0,"end":9640000000,"text":"hello"},{"id":1,"start":9640000000,"end":10000000000,"text":"world"}]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "whisper-1")
	result, err := client.TranscribeResult(context.Background(), TranscribeRequest{
		Filename:       "audio.wav",
		Audio:          []byte("RIFF...."),
		ResponseFormat: "verbose_json",
	})

	require.NoError(t, err)
	assert.Equal(t, "hello world", result.Text)
	require.Len(t, result.Segments, 2)
	// nanosecond values must be normalized to seconds
	assert.InDelta(t, 0.0, result.Segments[0].Start, 0.0001)
	assert.InDelta(t, 9.64, result.Segments[0].End, 0.001)
	assert.InDelta(t, 9.64, result.Segments[1].Start, 0.001)
	assert.InDelta(t, 10.0, result.Segments[1].End, 0.001)
}

func TestClientTranscribeSegments_SecondsUntouched(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"task":"transcribe","language":"en","duration":2.0,"text":"hi","segments":[{"id":0,"start":0.5,"end":1.5,"text":"hi"}]}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "whisper-1")
	result, err := client.TranscribeResult(context.Background(), TranscribeRequest{
		Filename:       "audio.wav",
		Audio:          []byte("RIFF...."),
		ResponseFormat: "verbose_json",
	})

	require.NoError(t, err)
	require.Len(t, result.Segments, 1)
	assert.InDelta(t, 0.5, result.Segments[0].Start, 0.0001)
	assert.InDelta(t, 1.5, result.Segments[0].End, 0.0001)
}

func TestNormalizeVisionImage_Downscales(t *testing.T) {
	img := imaging.New(3000, 2000, color.White)
	var buf bytes.Buffer
	require.NoError(t, imaging.Encode(&buf, img, imaging.PNG))

	normalized, mediaType := normalizeVisionImage(buf.Bytes())

	require.Equal(t, "image/jpeg", mediaType)
	decoded, err := imaging.Decode(bytes.NewReader(normalized))
	require.NoError(t, err)
	bounds := decoded.Bounds()
	assert.LessOrEqual(t, bounds.Dx(), maxVisionImageDimension)
	assert.LessOrEqual(t, bounds.Dy(), maxVisionImageDimension)
	assert.Greater(t, len(normalized), 0)
}

func TestNormalizeVisionImage_SmallUntouched(t *testing.T) {
	img := imaging.New(200, 100, color.Black)
	var buf bytes.Buffer
	require.NoError(t, imaging.Encode(&buf, img, imaging.JPEG))

	normalized, mediaType := normalizeVisionImage(buf.Bytes())

	assert.Equal(t, "image/jpeg", mediaType)
	decoded, err := imaging.Decode(bytes.NewReader(normalized))
	require.NoError(t, err)
	assert.Equal(t, 200, decoded.Bounds().Dx())
	assert.Equal(t, 100, decoded.Bounds().Dy())
}

func TestNormalizeVisionImage_CorruptPassthrough(t *testing.T) {
	data := []byte("not an image at all")
	normalized, _ := normalizeVisionImage(data)

	assert.Nil(t, normalized)
}

func TestNormalizeVisionImagePart(t *testing.T) {
	img := imaging.New(2500, 2500, color.Gray{Y: 128})
	var buf bytes.Buffer
	require.NoError(t, imaging.Encode(&buf, img, imaging.PNG))

	part := MultiImage{
		Base64:    base64.StdEncoding.EncodeToString(buf.Bytes()),
		MediaType: "image/png",
	}

	normalized := normalizeVisionImagePart(part)

	require.NotEqual(t, part.Base64, normalized.Base64)
	require.Equal(t, "image/jpeg", normalized.MediaType)
	decoded, err := imaging.Decode(bytes.NewReader(mustBase64(t, normalized.Base64)))
	require.NoError(t, err)
	assert.LessOrEqual(t, decoded.Bounds().Dx(), maxVisionImageDimension)

	// empty and corrupt parts pass through unchanged
	empty := normalizeVisionImagePart(MultiImage{})
	assert.Equal(t, "", empty.Base64)

	corrupt := MultiImage{Base64: base64.StdEncoding.EncodeToString([]byte("junk")), MediaType: "image/png"}
	assert.Equal(t, corrupt, normalizeVisionImagePart(corrupt))
}

func mustBase64(t *testing.T, s string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(s)
	require.NoError(t, err)
	return data
}
