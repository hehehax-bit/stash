package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL           string
	httpClient        *http.Client
	model             string
	endpoint          string
	transcriptionPath string
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ChatCompletionRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  interface{}      `json:"tool_choice,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   ChatCompletionUsage    `json:"usage"`
}

func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		model: model,
	}
}

// NewEmbeddingClient creates a client for embedding requests. endpoint selects
// the server API used for image embeddings: "ollama" uses Ollama's native
// /api/embed endpoint, anything else uses the OpenAI-compatible /embeddings
// endpoint.
func NewEmbeddingClient(baseURL, model, endpoint string) *Client {
	c := NewClient(baseURL, model)
	c.endpoint = endpoint
	return c
}

// SetTimeout sets the HTTP client timeout for requests to the AI server.
// Pass a duration <= 0 to restore the default 300s timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	defaultTimeout := 300 * time.Second
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if c.httpClient != nil {
		c.httpClient.Timeout = timeout
	}
}

// ListModels returns the model identifiers available on the server, as
// reported by the OpenAI-compatible /models endpoint. It is used for
// diagnostics when embedding requests fail with model-not-found style errors.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending models request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading models response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling models response: %w", err)
	}

	var ids []string
	for _, m := range result.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// SetTranscriptionPath overrides the URL path used for transcription requests.
// The default is /audio/transcriptions (OpenAI-compatible). Some whisper.cpp
// servers expose the endpoint at /inference or other paths.
func (c *Client) SetTranscriptionPath(path string) {
	if path == "" {
		path = "/audio/transcriptions"
	}
	c.transcriptionPath = path
}

func (c *Client) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if req.Model == "" {
		req.Model = c.model
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &result, nil
}

// TranscribeRequest holds the parameters for an audio transcription request.
type TranscribeRequest struct {
	Model          string
	Filename       string
	Audio          []byte
	Language       string
	Prompt         string
	ResponseFormat string
}

// Transcribe sends an audio file to the OpenAI-compatible /audio/transcriptions
// endpoint and returns the transcribed text. model may be empty to use the
// client's default model.
func (c *Client) Transcribe(ctx context.Context, model, filename string, audio []byte) (string, error) {
	result, err := c.TranscribeResult(ctx, TranscribeRequest{
		Model:    model,
		Filename: filename,
		Audio:    audio,
	})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// TranscriptionSegment is a timed segment of a transcript, as returned by
// whisper-style verbose_json responses.
type TranscriptionSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscriptionResult is the parsed output of a transcription request.
type TranscriptionResult struct {
	Text     string
	Segments []TranscriptionSegment
}

// TranscribeResult transcribes audio with additional options. OpenAI-compatible
// servers respond with JSON {text}, while whisper.cpp servers may return plain
// text or verbose_json (with timed segments); all formats are supported.
func (c *Client) TranscribeResult(ctx context.Context, req TranscribeRequest) (*TranscriptionResult, error) {
	if req.Model == "" {
		req.Model = c.model
	}

	path := c.transcriptionPath
	if path == "" {
		path = "/audio/transcriptions"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("model", req.Model); err != nil {
		return nil, fmt.Errorf("writing model field: %w", err)
	}
	if req.Language != "" {
		if err := writer.WriteField("language", req.Language); err != nil {
			return nil, fmt.Errorf("writing language field: %w", err)
		}
	}
	if req.Prompt != "" {
		if err := writer.WriteField("prompt", req.Prompt); err != nil {
			return nil, fmt.Errorf("writing prompt field: %w", err)
		}
	}
	if req.ResponseFormat != "" {
		if err := writer.WriteField("response_format", req.ResponseFormat); err != nil {
			return nil, fmt.Errorf("writing response_format field: %w", err)
		}
	}

	filename := req.Filename
	if filename == "" {
		filename = "audio.wav"
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(req.Audio); err != nil {
		return nil, fmt.Errorf("writing audio data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending transcription request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading transcription response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("transcription API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	text, segments := parseTranscriptionResponse(respBody)
	if text == "" {
		return nil, fmt.Errorf("transcription response contained no text")
	}

	return &TranscriptionResult{
		Text:     strings.TrimSpace(text),
		Segments: segments,
	}, nil
}

// parseTranscriptionResponse extracts the transcript text and any timed
// segments from a transcription response body. OpenAI-compatible servers
// return JSON {text} or verbose_json (with a segments array), while whisper.cpp
// servers may return the raw text body.
func parseTranscriptionResponse(respBody []byte) (string, []TranscriptionSegment) {
	trimmed := bytes.TrimSpace(respBody)
	if len(trimmed) == 0 {
		return "", nil
	}

	if trimmed[0] == '{' {
		var result struct {
			Text     json.RawMessage        `json:"text"`
			Segments []TranscriptionSegment `json:"segments"`
		}
		if err := json.Unmarshal(trimmed, &result); err != nil {
			return "", nil
		}

		text := ""
		if len(result.Text) > 0 {
			// text may be a plain string or a nested object {"text": "..."}
			var s string
			if err := json.Unmarshal(result.Text, &s); err == nil {
				text = s
			} else {
				var nested struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(result.Text, &nested); err == nil {
					text = nested.Text
				}
			}
		}

		// some servers (e.g. LocalAI) report segment times in nanoseconds;
		// normalize everything to seconds
		for i := range result.Segments {
			result.Segments[i].Start = normalizeSegmentTime(result.Segments[i].Start)
			result.Segments[i].End = normalizeSegmentTime(result.Segments[i].End)
		}

		return text, result.Segments
	}

	return string(trimmed), nil
}

// chatCompletionStreamChunk is a single SSE payload emitted by the
// /chat/completions endpoint when stream is enabled.
type chatCompletionStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Index        int         `json:"index"`
		Delta        ChatMessage `json:"delta"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ChatCompletionStream streams a chat completion, invoking onChunk for each
// content delta as it arrives. It returns the accumulated full text. If the
// server does not support streaming, an error is returned.
func (c *Client) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest, onChunk func(chunk string)) (string, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("API error (status %d)", resp.StatusCode)
		}
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var full strings.Builder
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if payload == "[DONE]" {
					break
				}
				if payload == "" {
					continue
				}

				var chunk chatCompletionStreamChunk
				if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
					return "", fmt.Errorf("unmarshaling stream chunk: %w", err)
				}

				if chunk.Error != nil {
					return "", fmt.Errorf("API error: %s", chunk.Error.Message)
				}

				for _, choice := range chunk.Choices {
					if content, ok := choice.Delta.Content.(string); ok && content != "" {
						full.WriteString(content)
						onChunk(content)
					}
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("reading stream: %w", err)
		}
	}

	return full.String(), nil
}

type MultiImage struct {
	Base64    string
	MediaType string
}

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedInputItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

type ollamaEmbedRequest struct {
	Model string                 `json:"model"`
	Input []ollamaEmbedInputItem `json:"input"`
}

type ollamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

// Embedding generates an embedding vector for text input. Depending on the
// client endpoint, this uses either Ollama's native /api/embed endpoint or the
// OpenAI-compatible /embeddings endpoint.
func (c *Client) Embedding(ctx context.Context, input string) ([]float32, error) {
	if c.endpoint == "ollama" {
		req := ollamaEmbedRequest{
			Model: c.model,
			Input: []ollamaEmbedInputItem{{Type: "text", Text: input}},
		}

		return c.postEmbedding(ctx, c.baseURL+"/api/embed", req, func(respBody []byte) ([]float32, error) {
			var result ollamaEmbedResponse
			if err := json.Unmarshal(respBody, &result); err != nil {
				return nil, fmt.Errorf("unmarshaling ollama embedding response: %w", err)
			}

			if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
				return nil, fmt.Errorf("no embedding data in response")
			}

			return result.Embeddings[0], nil
		})
	}

	req := EmbeddingRequest{
		Model: c.model,
		Input: input,
	}

	return c.postEmbedding(ctx, c.baseURL+"/embeddings", req, func(respBody []byte) ([]float32, error) {
		var result EmbeddingResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("unmarshaling embedding response: %w", err)
		}

		if len(result.Data) == 0 {
			return nil, fmt.Errorf("no embedding data in response")
		}

		return result.Data[0].Embedding, nil
	})
}

// EmbeddingImage generates an embedding vector for image content. The image is
// passed as base64-encoded data with its media type (e.g. "image/jpeg").
// Depending on the client endpoint, this uses either Ollama's native /api/embed
// endpoint or the OpenAI-compatible /embeddings endpoint with a data URL input.
func (c *Client) EmbeddingImage(ctx context.Context, imageBase64 string, mediaType string) ([]float32, error) {
	if c.endpoint == "ollama" {
		req := ollamaEmbedRequest{
			Model: c.model,
			Input: []ollamaEmbedInputItem{{Type: "image", Data: imageBase64}},
		}

		return c.postEmbedding(ctx, c.baseURL+"/api/embed", req, func(respBody []byte) ([]float32, error) {
			var result ollamaEmbedResponse
			if err := json.Unmarshal(respBody, &result); err != nil {
				return nil, fmt.Errorf("unmarshaling ollama embedding response: %w", err)
			}

			if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
				return nil, fmt.Errorf("no embedding data in response")
			}

			return result.Embeddings[0], nil
		})
	}

	req := EmbeddingRequest{
		Model: c.model,
		Input: "data:" + mediaType + ";base64," + imageBase64,
	}

	return c.postEmbedding(ctx, c.baseURL+"/embeddings", req, func(respBody []byte) ([]float32, error) {
		var result EmbeddingResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("unmarshaling embedding response: %w", err)
		}

		if len(result.Data) == 0 {
			return nil, fmt.Errorf("no embedding data in response")
		}

		return result.Data[0].Embedding, nil
	})
}

func (c *Client) postEmbedding(ctx context.Context, url string, req interface{}, parse func([]byte) ([]float32, error)) ([]float32, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling embedding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating embedding request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending embedding request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading embedding response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return parse(respBody)
}

func (c *Client) VisionCompletion(ctx context.Context, systemPrompt string, userText string, imageBase64 string, imageMediaType string) (string, error) {
	return c.MultiVisionCompletion(ctx, systemPrompt, userText, []MultiImage{{Base64: imageBase64, MediaType: imageMediaType}})
}

func (c *Client) MultiVisionCompletion(ctx context.Context, systemPrompt string, userText string, images []MultiImage) (string, error) {
	contentParts := []ContentPart{
		{Type: "text", Text: userText},
	}
	for _, img := range images {
		contentParts = append(contentParts, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURL{URL: "data:" + img.MediaType + ";base64," + img.Base64},
		})
	}

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: contentParts,
		},
	}

	req := ChatCompletionRequest{
		Messages: messages,
	}

	resp, err := c.ChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("vision completion: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type from vision response")
	}

	return content, nil
}

// normalizeSegmentTime converts a transcription segment timestamp to seconds.
// Values in the nanosecond range (> 1e6) are divided by 1e9; second-valued
// timestamps pass through unchanged.
func normalizeSegmentTime(v float64) float64 {
	if v > 1e6 {
		return v / 1e9
	}
	return v
}
