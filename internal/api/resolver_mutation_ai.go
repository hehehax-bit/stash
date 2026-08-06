package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/models"
)

func (r *mutationResolver) ConfigureAi(ctx context.Context, input config.AIConfigInput) (*config.AIConfig, error) {
	cfg := manager.GetInstance().Config

	if input.Enabled != nil {
		cfg.SetAIEnabled(*input.Enabled)
	}
	if input.BaseURL != nil {
		cfg.SetAIBaseURL(*input.BaseURL)
	}
	if input.Endpoint != nil {
		cfg.SetAIEndpoint(*input.Endpoint)
	}
	if input.Model != nil {
		cfg.SetAIModel(*input.Model)
	}
	if input.EmbeddingModel != nil {
		cfg.SetAIEmbeddingModel(*input.EmbeddingModel)
	}
	if input.ImageEmbeddingModel != nil {
		cfg.SetAIImageEmbeddingModel(*input.ImageEmbeddingModel)
	}
	if input.SystemPrompt != nil {
		cfg.SetAISystemPrompt(*input.SystemPrompt)
	}
	if input.Automatic1111Enabled != nil {
		cfg.SetAIAutomatic1111Enabled(*input.Automatic1111Enabled)
	}
	if input.Automatic1111BaseURL != nil {
		cfg.SetAIAutomatic1111BaseURL(*input.Automatic1111BaseURL)
	}
	if input.Tag != nil {
		cfg.SetAITag(*input.Tag)
	}
	if input.TranscriptionBaseURL != nil {
		cfg.SetAITranscriptionBaseURL(*input.TranscriptionBaseURL)
	}
	if input.TranscriptionModel != nil {
		cfg.SetAITranscriptionModel(*input.TranscriptionModel)
	}
	if input.TranscriptionEndpoint != nil {
		cfg.SetAITranscriptionEndpoint(*input.TranscriptionEndpoint)
	}
	if input.PerformerClusterMinConfidence != nil {
		cfg.SetAIPerformerClusterMinConfidence(*input.PerformerClusterMinConfidence)
	}
	if input.MaxTokens != nil {
		cfg.SetAIMaxTokens(*input.MaxTokens)
	}
	if input.SilenceNoiseThreshold != nil {
		cfg.SetAISilenceNoiseThreshold(*input.SilenceNoiseThreshold)
	}
	if input.SilenceDurationMin != nil {
		cfg.SetAISilenceDurationMin(*input.SilenceDurationMin)
	}
	if input.FramesToSample != nil {
		cfg.SetAIFramesToSample(*input.FramesToSample)
	}
	if input.TranslationLanguage != nil {
		cfg.SetAITranslationLanguage(*input.TranslationLanguage)
	}
	if input.ScheduledTasks != nil {
		cfg.SetAIScheduledTasks(*input.ScheduledTasks)
	}

	if err := cfg.Write(); err != nil {
		return nil, fmt.Errorf("writing config: %w", err)
	}

	mgr := manager.GetInstance()
	mgr.RefreshAIService()

	return &config.AIConfig{
		Enabled:                       cfg.GetAIEnabled(),
		BaseURL:                       cfg.GetAIBaseURL(),
		Endpoint:                      cfg.GetAIEndpoint(),
		Model:                         cfg.GetAIModel(),
		EmbeddingModel:                cfg.GetAIEmbeddingModel(),
		ImageEmbeddingModel:           cfg.GetAIImageEmbeddingModel(),
		SystemPrompt:                  cfg.GetAISystemPrompt(),
		Automatic1111Enabled:          cfg.GetAIAutomatic1111Enabled(),
		Automatic1111BaseURL:          cfg.GetAIAutomatic1111BaseURL(),
		Tag:                           cfg.GetAITag(),
		TranscriptionBaseURL:          cfg.GetAITranscriptionBaseURL(),
		TranscriptionModel:            cfg.GetAITranscriptionModel(),
		TranscriptionEndpoint:         cfg.GetAITranscriptionEndpoint(),
		PerformerClusterMinConfidence: cfg.GetAIPerformerClusterMinConfidence(),
		MaxTokens:                     cfg.GetAIMaxTokens(),
		SilenceNoiseThreshold:         cfg.GetAISilenceNoiseThreshold(),
		SilenceDurationMin:            cfg.GetAISilenceDurationMin(),
		FramesToSample:                cfg.GetAIFramesToSample(),
		TranslationLanguage:           cfg.GetAITranslationLanguage(),
		ScheduledTasks:                cfg.GetAIScheduledTasks(),
	}, nil
}

func (r *mutationResolver) AiChatSend(ctx context.Context, input AIChatSendInput) (*models.AIChatMessage, error) {
	mgr := manager.GetInstance()

	if !mgr.AIService.IsEnabled() {
		return nil, fmt.Errorf("AI is not enabled; configure AI settings first")
	}

	var sessionID string
	var history []ai.ChatMessage

	if err := r.withTxn(ctx, func(ctx context.Context) error {
		if input.SessionID != nil && *input.SessionID != "" {
			sessionID = *input.SessionID
			dbMessages, err := r.repository.AI.FindBySessionID(ctx, sessionID)
			if err != nil {
				return fmt.Errorf("loading history: %w", err)
			}
			for _, m := range dbMessages {
				history = append(history, ai.ChatMessage{
					Role:    m.Role,
					Content: m.Content,
				})
			}
			if err := r.repository.AI.Touch(ctx, sessionID); err != nil {
				return fmt.Errorf("touching session: %w", err)
			}
		} else {
			sess := &models.AIChatSession{}
			if err := r.repository.AI.CreateSession(ctx, sess); err != nil {
				return fmt.Errorf("creating session: %w", err)
			}
			sessionID = sess.ID
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// augment the conversation with similar scenes from the library when the
	// message is plain text and library context is enabled
	useLibraryContext := true
	if input.UseLibraryContext != nil {
		useLibraryContext = *input.UseLibraryContext
	}
	hasImage := input.Image != nil && *input.Image != ""
	if useLibraryContext && !hasImage && strings.TrimSpace(input.Message) != "" {
		if contextMsg, err := r.libraryContext(ctx, input.Message); err == nil && contextMsg != nil {
			history = append([]ai.ChatMessage{*contextMsg}, history...)
		}
	}

	var resp ai.ChatMessage
	if err := mgr.Repository.WithDB(ctx, func(ctx context.Context) error {
		var err error
		if hasImage {
			resp, err = mgr.AIService.SendMessageWithImage(ctx, history, input.Message, *input.Image)
		} else {
			resp, err = mgr.AIService.SendMessage(ctx, history, input.Message)
		}
		return err
	}); err != nil {
		return nil, fmt.Errorf("AI chat error: %w", err)
	}

	var result *models.AIChatMessage
	var isNew bool
	if input.SessionID == nil || *input.SessionID == "" {
		isNew = true
	}
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		userContent := input.Message
		if strings.TrimSpace(userContent) == "" && input.Image != nil && *input.Image != "" {
			userContent = "Sent an image"
		}
		if err := r.repository.AI.CreateMessage(ctx, &models.AIChatMessage{
			SessionID: sessionID,
			Role:      "user",
			Content:   userContent,
		}); err != nil {
			return fmt.Errorf("saving user message: %w", err)
		}

		respContent, ok := resp.Content.(string)
		if !ok {
			return fmt.Errorf("unexpected content type from AI response")
		}
		msg := &models.AIChatMessage{
			SessionID: sessionID,
			Role:      resp.Role,
			Content:   respContent,
		}
		if err := r.repository.AI.CreateMessage(ctx, msg); err != nil {
			return fmt.Errorf("saving ai response: %w", err)
		}
		result = msg
		return nil
	}); err != nil {
		return nil, err
	}

	if isNew {
		title, err := mgr.AIService.GenerateTitle(ctx, input.Message)
		if err == nil {
			if err := r.withTxn(ctx, func(ctx context.Context) error {
				return r.repository.AI.SetTitle(ctx, sessionID, title)
			}); err != nil {
				return nil, fmt.Errorf("saving title: %w", err)
			}
		}
	}

	return result, nil
}

func (r *mutationResolver) AiChatClear(ctx context.Context, sessionID string) (bool, error) {
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.AI.DeleteBySessionID(ctx, sessionID)
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (r *mutationResolver) AiChatDeleteMessage(ctx context.Context, messageID string) (bool, error) {
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		return r.repository.AI.DeleteByID(ctx, messageID)
	}); err != nil {
		return false, err
	}
	return true, nil
}

// libraryContext builds a system message containing the scenes whose
// embeddings are most similar to the user's message, so the chat model can
// answer questions about the library. Returns nil when no context is
// available or when embedding/searching fails.
func (r *Resolver) libraryContext(ctx context.Context, message string) (*ai.ChatMessage, error) {
	cfg := manager.GetInstance().Config
	model := cfg.GetAIEmbeddingModel()
	if model == "" {
		model = cfg.GetAIModel()
	}
	baseURL := cfg.GetAIBaseURL()
	if model == "" || baseURL == "" {
		return nil, nil
	}

	client := ai.NewEmbeddingClient(baseURL, model, cfg.GetAIEndpoint())

	queryEmbedding, err := client.Embedding(ctx, message)
	if err != nil {
		return nil, err
	}

	var contextText strings.Builder
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		similar, err := r.repository.Embedding.SearchSimilar(ctx, "scene", model, queryEmbedding, 5)
		if err != nil {
			return err
		}

		for _, sim := range similar {
			s, err := r.repository.Scene.Find(ctx, sim.EntityID)
			if err != nil || s == nil {
				continue
			}

			title := s.Title
			if title == "" {
				title = s.Path
			}
			fmt.Fprintf(&contextText, "- %s", title)
			if s.Details != "" {
				fmt.Fprintf(&contextText, ": %s", truncateText(s.Details, 500))
			}
			if audio, err := r.repository.AISceneAudio.FindBySceneID(ctx, s.ID); err == nil && audio != nil {
				if audio.Summary != "" {
					fmt.Fprintf(&contextText, " [summary: %s]", truncateText(audio.Summary, 300))
				}
				if audio.Transcript != "" {
					fmt.Fprintf(&contextText, " [transcript: %s]", truncateText(audio.Transcript, 500))
				}
			}
			contextText.WriteString("\n")
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if contextText.Len() == 0 {
		return nil, nil
	}

	return &ai.ChatMessage{
		Role:    "system",
		Content: "Library context (use it when it helps answer the user's question):\n" + contextText.String(),
	}, nil
}

func truncateText(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
