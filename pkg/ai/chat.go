package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

const defaultMaxTokens = 2048

type ToolConfig struct {
	A1111BaseURL         string
	A1111Enabled         bool
	GeneratedPath        string
	LLMBaseURL           string
	LLMModel             string
	EmbeddingModel       string
	FFMpegPath           string
	MaxTokens            int
	StartEmbeddingJob    func(ctx context.Context, entityTypes []string, overwrite bool) (int, error)
	StartSceneSegmentJob func(ctx context.Context, sceneIDs []int, maxScenes *int, overwrite bool) (int, error)
}

type ChatService struct {
	client       *Client
	tools        []Tool
	toolMap      map[string]Tool
	repo         models.Repository
	systemPrompt string
	enabled      bool
	toolCfg      ToolConfig
}

func NewChatService(client *Client, repo models.Repository, systemPrompt string) *ChatService {
	return newChatServiceWithConfig(client, repo, systemPrompt, ToolConfig{})
}

func newChatServiceWithConfig(client *Client, repo models.Repository, systemPrompt string, toolCfg ToolConfig) *ChatService {
	tools := GetTools(toolCfg)
	toolMap := make(map[string]Tool, len(tools))
	for _, t := range tools {
		toolMap[t.Name] = t
	}

	return &ChatService{
		client:       client,
		tools:        tools,
		toolMap:      toolMap,
		repo:         repo,
		systemPrompt: systemPrompt,
		enabled:      false,
		toolCfg:      toolCfg,
	}
}

func (s *ChatService) SetToolConfig(cfg ToolConfig) {
	s.toolCfg = cfg
	s.tools = GetTools(cfg)
	s.toolMap = make(map[string]Tool, len(s.tools))
	for _, t := range s.tools {
		s.toolMap[t.Name] = t
	}
}

func (s *ChatService) SetEnabled(enabled bool) {
	s.enabled = enabled
}

func (s *ChatService) IsEnabled() bool {
	return s.enabled
}

func (s *ChatService) SetSystemPrompt(prompt string) {
	s.systemPrompt = prompt
}

func (s *ChatService) SetClient(client *Client) {
	s.client = client
}

type ToolCallResult struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

func (s *ChatService) SendMessage(ctx context.Context, history []ChatMessage, userMessage string) (ChatMessage, error) {
	return s.sendMessageInternal(ctx, history, userMessage, "")
}

func (s *ChatService) SendMessageWithImage(ctx context.Context, history []ChatMessage, userMessage, imageDataURI string) (ChatMessage, error) {
	return s.sendMessageInternal(ctx, history, userMessage, imageDataURI)
}

func (s *ChatService) sendMessageInternal(ctx context.Context, history []ChatMessage, userMessage, imageDataURI string) (ChatMessage, error) {
	if !s.enabled || s.client == nil {
		return ChatMessage{}, fmt.Errorf("AI is not configured or disabled")
	}

	systemContent := s.systemPrompt

	memories, err := s.repo.Memory.FindAll(ctx)
	if err == nil && len(memories) > 0 {
		var memParts []string
		for _, m := range memories {
			memParts = append(memParts, fmt.Sprintf("- %s: %s", m.Key, m.Value))
		}
		systemContent += "\n\nUser preferences/memories:\n" + strings.Join(memParts, "\n")
	}

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemContent,
		},
	}

	messages = append(messages, history...)

	if imageDataURI != "" {
		parts := make([]ContentPart, 0, 2)
		if userMessage != "" {
			parts = append(parts, ContentPart{Type: "text", Text: userMessage})
		}
		parts = append(parts, ContentPart{Type: "image_url", ImageURL: &ImageURL{URL: imageDataURI}})
		messages = append(messages, ChatMessage{
			Role:    "user",
			Content: parts,
		})
	} else {
		messages = append(messages, ChatMessage{
			Role:    "user",
			Content: userMessage,
		})
	}

	toolDefs := make([]ToolDefinition, len(s.tools))
	for i, t := range s.tools {
		toolDefs[i] = toolDefFromTool(t)
	}

	maxTokens := s.toolCfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	for i := 0; i < 10; i++ {
		req := ChatCompletionRequest{
			Messages:   messages,
			Tools:      toolDefs,
			ToolChoice: "auto",
			MaxTokens:  maxTokens,
		}

		resp, err := s.client.ChatCompletion(ctx, req)
		if err != nil {
			return ChatMessage{}, fmt.Errorf("chat completion: %w", err)
		}

		choice := resp.Choices[0]

		// Process tool calls whenever present, regardless of finish_reason.
		// Some OpenAI-compatible servers (LM Studio, llama.cpp, vLLM) report
		// finish_reason "stop" rather than "tool_calls" when the model requests
		// tools. Returning early in that case would drop the pending tool calls
		// and surface an empty response to the user.
		if len(choice.Message.ToolCalls) > 0 {
			messages = append(messages, choice.Message)

			for _, tc := range choice.Message.ToolCalls {
				tool, ok := s.toolMap[tc.Function.Name]
				if !ok {
					result := fmt.Sprintf("Error: unknown tool '%s'", tc.Function.Name)
					messages = append(messages, ChatMessage{
						Role:       "tool",
						Content:    result,
						ToolCallID: tc.ID,
					})
					continue
				}

				result, err := tool.Execute(ctx, s.repo, json.RawMessage(tc.Function.Arguments))
				if err != nil {
					result = fmt.Sprintf("Error executing %s: %v", tc.Function.Name, err)
				}

				messages = append(messages, ChatMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
			}

			continue
		}

		// No tool calls: this is the final answer. Refuse to surface empty
		// responses, which providers can produce when the response is truncated
		// or when content is missing entirely.
		if choice.FinishReason == "length" {
			return ChatMessage{}, fmt.Errorf("AI response was truncated because it hit the token limit")
		}
		if strings.TrimSpace(contentToString(choice.Message)) == "" {
			return ChatMessage{}, fmt.Errorf("AI returned an empty response (finish reason %q)", choice.FinishReason)
		}

		return choice.Message, nil
	}

	return ChatMessage{
		Role:    "assistant",
		Content: "I've reached the maximum number of tool calls. Please refine your question.",
	}, nil
}

func contentToString(m ChatMessage) string {
	switch c := m.Content.(type) {
	case string:
		return c
	case []ContentPart:
		var b strings.Builder
		for _, p := range c {
			b.WriteString(p.Text)
		}
		return b.String()
	default:
		return ""
	}
}

func (s *ChatService) GenerateTitle(ctx context.Context, userMessage string) (string, error) {
	if !s.enabled || s.client == nil {
		return "", fmt.Errorf("AI is not configured or disabled")
	}

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: "Generate a concise title (maximum 6 words) for a chat conversation that starts with this message. Respond with ONLY the title, no quotes, punctuation, or explanation.",
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	req := ChatCompletionRequest{
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   defaultMaxTokens,
	}

	resp, err := s.client.ChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("generating title: %w", err)
	}

	title, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type from title response")
	}

	return title, nil
}
