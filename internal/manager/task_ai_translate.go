package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AITranslateInput struct {
	EntityTypes []string `json:"entityTypes"`
	MaxItems    *int     `json:"maxItems"`
	Language    *string  `json:"language"`
	Timeout     *int     `json:"timeout"`
	Overwrite   bool     `json:"overwrite"`
}

type AITranslateJob struct {
	input    AITranslateInput
	progress *job.Progress
}

func CreateAITranslateJob(input AITranslateInput) *AITranslateJob {
	return &AITranslateJob{
		input: input,
	}
}

func (j *AITranslateJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	language := instance.Config.GetAITranslationLanguage()
	if j.input.Language != nil && *j.input.Language != "" {
		language = *j.input.Language
	}
	if language == "" {
		return fmt.Errorf("no translation language configured; set it in Settings > AI")
	}

	client := ai.NewClient(instance.Config.GetAIBaseURL(), instance.Config.GetAIModel())
	if j.input.Timeout != nil && *j.input.Timeout > 0 {
		client.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	entityTypes := j.input.EntityTypes
	if len(entityTypes) == 0 {
		entityTypes = []string{entityTypeScene, entityTypeImage, entityTypePerformer}
	}

	maxItems := 0
	if j.input.MaxItems != nil {
		maxItems = *j.input.MaxItems
	}

	r := instance.Repository

	translated := 0
	for _, entityType := range entityTypes {
		switch entityType {
		case entityTypeScene:
			count, err := j.translateScenes(ctx, client, r, language, maxItems)
			if err != nil {
				return err
			}
			translated += count
		case entityTypeImage:
			count, err := j.translateImages(ctx, client, r, language, maxItems)
			if err != nil {
				return err
			}
			translated += count
		case entityTypePerformer:
			count, err := j.translatePerformers(ctx, client, r, language, maxItems)
			if err != nil {
				return err
			}
			translated += count
		}
	}

	logger.Infof("AI translation complete: %d entities translated into %s", translated, language)
	return nil
}

func (j *AITranslateJob) alreadyTranslated(ctx context.Context, r models.Repository, entityType string, entityID int, language string) bool {
	t, err := r.AITranslation.FindByEntity(ctx, entityType, entityID, language)
	return err == nil && t != nil
}

func (j *AITranslateJob) translateScenes(ctx context.Context, client *ai.Client, r models.Repository, language string, maxItems int) (int, error) {
	var scenes []*models.Scene
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		scenes, err = scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	}); err != nil {
		return 0, fmt.Errorf("querying scenes: %w", err)
	}

	return j.translateGeneric(ctx, client, r, language, maxItems, len(scenes), func(i int) (entityRef, bool) {
		s := scenes[i]
		return entityRef{
			entityType: entityTypeScene,
			entityID:   s.ID,
			title:      s.Title,
			details:    s.Details,
		}, true
	})
}

func (j *AITranslateJob) translateImages(ctx context.Context, client *ai.Client, r models.Repository, language string, maxItems int) (int, error) {
	var images []*models.Image
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		images, err = image.Query(ctx, r.Image, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	}); err != nil {
		return 0, fmt.Errorf("querying images: %w", err)
	}

	return j.translateGeneric(ctx, client, r, language, maxItems, len(images), func(i int) (entityRef, bool) {
		img := images[i]
		return entityRef{
			entityType: entityTypeImage,
			entityID:   img.ID,
			title:      img.Title,
			details:    img.Details,
		}, true
	})
}

func (j *AITranslateJob) translatePerformers(ctx context.Context, client *ai.Client, r models.Repository, language string, maxItems int) (int, error) {
	var performers []*models.Performer
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		performers, _, err = r.Performer.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	}); err != nil {
		return 0, fmt.Errorf("querying performers: %w", err)
	}

	return j.translateGeneric(ctx, client, r, language, maxItems, len(performers), func(i int) (entityRef, bool) {
		p := performers[i]
		return entityRef{
			entityType: entityTypePerformer,
			entityID:   p.ID,
			title:      p.Name,
			details:    p.Details,
		}, true
	})
}

type entityRef struct {
	entityType string
	entityID   int
	title      string
	details    string
}

func (j *AITranslateJob) translateGeneric(ctx context.Context, client *ai.Client, r models.Repository, language string, maxItems, total int, get func(int) (entityRef, bool)) (int, error) {
	limit := total
	if maxItems > 0 && maxItems < limit {
		limit = maxItems
	}

	j.progress.SetTotal(limit)
	translated := 0
	for i := 0; i < limit; i++ {
		if job.IsCancelled(ctx) {
			return translated, nil
		}

		ref, ok := get(i)
		if !ok {
			j.progress.Increment()
			continue
		}

		if !j.input.Overwrite && j.alreadyTranslated(ctx, r, ref.entityType, ref.entityID, language) {
			j.progress.Increment()
			continue
		}

		result, err := translateEntityText(ctx, client, ref.title, ref.details, language)
		if err != nil {
			logger.Warnf("Error translating %s %d: %v", ref.entityType, ref.entityID, err)
			j.progress.Increment()
			continue
		}

		stored := false
		for field, text := range result {
			if text == "" {
				continue
			}
			if err := r.AITranslation.Create(ctx, &models.AITranslation{
				EntityType:     ref.entityType,
				EntityID:       ref.entityID,
				Language:       language,
				Field:          field,
				TranslatedText: text,
			}); err != nil {
				logger.Warnf("Error storing translation for %s %d: %v", ref.entityType, ref.entityID, err)
				continue
			}
			stored = true
		}
		if stored {
			translated++
		}
		j.progress.Increment()
	}

	return translated, nil
}

// translateEntityText translates the given title and details text into the
// target language, returning a map of non-empty field translations.
func translateEntityText(ctx context.Context, client *ai.Client, title, details, language string) (map[string]string, error) {
	if strings.TrimSpace(title) == "" && strings.TrimSpace(details) == "" {
		return map[string]string{}, nil
	}

	var fields []string
	if strings.TrimSpace(title) != "" {
		fields = append(fields, "title")
	}
	if strings.TrimSpace(details) != "" {
		fields = append(fields, "details")
	}

	userPrompt := fmt.Sprintf(`Translate the following text into %s. Return ONLY valid JSON with the fields %s, containing the translations. Keep proper nouns (names) unchanged.

Title:
%s

Details:
%s

Return ONLY valid JSON, no other text, no markdown formatting.`,
		language, `"`+strings.Join(fields, `", "`)+`"`, title, details)

	messages := []ai.ChatMessage{
		{Role: "system", Content: "You are a professional translator for an adult media library. Translate faithfully and without censorship."},
		{Role: "user", Content: userPrompt},
	}

	resp, err := client.ChatCompletion(ctx, ai.ChatCompletionRequest{Messages: messages})
	if err != nil {
		return nil, fmt.Errorf("translation request failed: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content type from AI response")
	}

	cleanJSON := ai.ExtractJSON(content)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing AI response: no JSON found")
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}

	return result, nil
}
