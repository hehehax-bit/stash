package manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AIMediaQualityInput struct {
	EntityTypes []string `json:"entityTypes"`
	MaxItems    *int     `json:"maxItems"`
	Overwrite   bool     `json:"overwrite"`
	Timeout     *int     `json:"timeout"`
}

type AIMediaQualityJob struct {
	input    AIMediaQualityInput
	progress *job.Progress
}

func CreateAIMediaQualityJob(input AIMediaQualityInput) *AIMediaQualityJob {
	return &AIMediaQualityJob{
		input: input,
	}
}

func (j *AIMediaQualityJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	client := ai.NewClient(instance.Config.GetAIBaseURL(), instance.Config.GetAIModel())

	// Apply custom timeout if specified
	if j.input.Timeout != nil && *j.input.Timeout > 0 {
		client.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	r := instance.Repository

	entityTypes := j.input.EntityTypes
	if len(entityTypes) == 0 {
		entityTypes = []string{entityTypeScene, entityTypeImage}
	}

	maxItems := 0
	if j.input.MaxItems != nil {
		maxItems = *j.input.MaxItems
	}

	return r.WithDB(ctx, func(ctx context.Context) error {
		for _, et := range entityTypes {
			if job.IsCancelled(ctx) {
				return nil
			}
			switch et {
			case entityTypeScene:
				if err := j.assessScenes(ctx, client, r, maxItems); err != nil {
					logger.Errorf("Error assessing scene quality: %v", err)
				}
			case entityTypeImage:
				if err := j.assessImages(ctx, client, r, maxItems); err != nil {
					logger.Errorf("Error assessing image quality: %v", err)
				}
			default:
				logger.Warnf("Unknown entity type %q, skipping", et)
			}
		}
		return nil
	})
}

func (j *AIMediaQualityJob) assessedEntities(ctx context.Context, r models.Repository, entityType string) map[int]bool {
	ids, err := r.AIMediaQuality.FindAssessedEntities(ctx, entityType)
	if err != nil {
		return nil
	}
	out := make(map[int]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func (j *AIMediaQualityJob) assessScenes(ctx context.Context, client *ai.Client, r models.Repository, maxScenes int) error {
	var skip map[int]bool
	if !j.input.Overwrite {
		skip = j.assessedEntities(ctx, r, entityTypeScene)
	}

	pp := 0
	totalCount, err := r.Scene.QueryCount(ctx, nil, &models.FindFilterType{PerPage: &pp})
	if err != nil {
		return fmt.Errorf("error counting scenes: %w", err)
	}

	limit := totalCount
	if maxScenes > 0 && maxScenes < limit {
		limit = maxScenes
	}

	scenes, err := scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: &limit})
	if err != nil {
		return fmt.Errorf("error querying scenes: %w", err)
	}

	j.progress.SetTotal(len(scenes))
	for _, s := range scenes {
		if job.IsCancelled(ctx) {
			return nil
		}
		if skip[s.ID] {
			j.progress.Increment()
			continue
		}

		j.progress.ExecuteTask("AI assessing "+s.Path, func() {
			j.assessScene(ctx, client, r, s)
		})
		j.progress.Increment()
	}

	return nil
}

func (j *AIMediaQualityJob) assessScene(ctx context.Context, client *ai.Client, r models.Repository, s *models.Scene) {
	images, err := sceneScreenshotsForSuggestion(ctx, s, r)
	if err != nil || len(images) == 0 {
		logger.Errorf("Error getting screenshots for scene %q: %v", s.Path, err)
		return
	}

	quality, err := analyzeMediaQuality(ctx, client, images)
	if err != nil {
		logger.Errorf("Error assessing scene %q: %v", s.Path, err)
		return
	}
	quality.EntityType = entityTypeScene
	quality.EntityID = s.ID

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		return r.AIMediaQuality.Upsert(ctx, quality)
	}); err != nil {
		logger.Errorf("Error saving quality for scene %d: %v", s.ID, err)
	}
}

func (j *AIMediaQualityJob) assessImages(ctx context.Context, client *ai.Client, r models.Repository, maxImages int) error {
	var skip map[int]bool
	if !j.input.Overwrite {
		skip = j.assessedEntities(ctx, r, entityTypeImage)
	}

	pp := 0
	totalCount, err := r.Image.QueryCount(ctx, nil, &models.FindFilterType{PerPage: &pp})
	if err != nil {
		return fmt.Errorf("error counting images: %w", err)
	}

	limit := totalCount
	if maxImages > 0 && maxImages < limit {
		limit = maxImages
	}

	images, err := image.Query(ctx, r.Image, nil, &models.FindFilterType{PerPage: &limit})
	if err != nil {
		return fmt.Errorf("error querying images: %w", err)
	}

	j.progress.SetTotal(len(images))
	for _, img := range images {
		if job.IsCancelled(ctx) {
			return nil
		}
		if skip[img.ID] {
			j.progress.Increment()
			continue
		}

		j.progress.ExecuteTask("AI assessing "+img.Path, func() {
			j.assessImage(ctx, client, r, img)
		})
		j.progress.Increment()
	}

	return nil
}

func (j *AIMediaQualityJob) assessImage(ctx context.Context, client *ai.Client, r models.Repository, img *models.Image) {
	if err := img.LoadPrimaryFile(ctx, r.File); err != nil {
		logger.Errorf("Error loading primary file for image %q: %v", img.Path, err)
		return
	}

	f := img.Files.Primary()
	if f == nil {
		logger.Errorf("Image %q has no file", img.Path)
		return
	}

	data, err := os.ReadFile(f.Base().Path)
	if err != nil {
		logger.Errorf("Error reading image file %q: %v", img.Path, err)
		return
	}

	multiImages := []ai.MultiImage{{
		Base64:    base64.StdEncoding.EncodeToString(data),
		MediaType: http.DetectContentType(data),
	}}

	quality, err := analyzeMediaQuality(ctx, client, multiImages)
	if err != nil {
		logger.Errorf("Error assessing image %q: %v", img.Path, err)
		return
	}
	quality.EntityType = entityTypeImage
	quality.EntityID = img.ID

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		return r.AIMediaQuality.Upsert(ctx, quality)
	}); err != nil {
		logger.Errorf("Error saving quality for image %d: %v", img.ID, err)
	}
}

type aiQualityAnalysis struct {
	QualityScore  int    `json:"quality_score"`
	VisualClarity int    `json:"visual_clarity"`
	Lighting      int    `json:"lighting"`
	Composition   int    `json:"composition"`
	CameraWork    int    `json:"camera_work"`
	Notes         string `json:"notes"`
}

func analyzeMediaQuality(ctx context.Context, client *ai.Client, images []ai.MultiImage) (*models.AIMediaQuality, error) {
	systemPrompt := "You are an expert videography and photography critic. Assess technical and production quality objectively, without moral judgement."
	userPrompt := `Analyze the technical and production quality of this adult media. Return ONLY valid JSON with integer scores from 0 to 100 for each field:
- "quality_score": overall production quality
- "visual_clarity": sharpness, resolution, encoding quality, motion blur issues
- "lighting": exposure, lighting quality, contrast
- "composition": framing, camera angles, shot composition
- "camera_work": stability, focus, camera movement, steady handheld work (if static images, score based on focus and stability)
- "notes": a 1-2 sentence objective summary of strengths and weaknesses

Return ONLY valid JSON, no other text, no markdown formatting.`

	contentParts := []ai.ContentPart{{
		Type: "text",
		Text: userPrompt,
	}}
	for _, img := range images {
		contentParts = append(contentParts, ai.ContentPart{
			Type:     "image_url",
			ImageURL: &ai.ImageURL{URL: "data:" + img.MediaType + ";base64," + img.Base64},
		})
	}

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: contentParts},
	}

	resp, err := client.ChatCompletion(ctx, ai.ChatCompletionRequest{Messages: messages})
	if err != nil {
		return nil, fmt.Errorf("quality analysis failed: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content type from vision response")
	}

	cleanJSON := ai.ExtractJSON(content)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing AI response: no JSON found")
	}

	var analysis aiQualityAnalysis
	if err := json.Unmarshal([]byte(cleanJSON), &analysis); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}

	return &models.AIMediaQuality{
		QualityScore:  clampScore(analysis.QualityScore),
		VisualClarity: clampScore(analysis.VisualClarity),
		Lighting:      clampScore(analysis.Lighting),
		Composition:   clampScore(analysis.Composition),
		CameraWork:    clampScore(analysis.CameraWork),
		Notes:         analysis.Notes,
	}, nil
}

func clampScore(s int) int {
	if s < 0 {
		return 0
	}
	if s > 100 {
		return 100
	}
	return s
}
