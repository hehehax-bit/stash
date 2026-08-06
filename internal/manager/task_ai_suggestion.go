package manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AISuggestionInput struct {
	EntityTypes []string `json:"entityTypes"`
	MaxItems    *int     `json:"maxItems"`
	Timeout     *int     `json:"timeout"`
}

type AISuggestionJob struct {
	input    AISuggestionInput
	progress *job.Progress
}

func CreateAISuggestionJob(input AISuggestionInput) *AISuggestionJob {
	return &AISuggestionJob{
		input: input,
	}
}

func (j *AISuggestionJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	baseURL := instance.Config.GetAIBaseURL()
	model := instance.Config.GetAIModel()
	client := ai.NewClient(baseURL, model)

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
				if err := j.suggestScenes(ctx, client, r, maxItems); err != nil {
					logger.Errorf("Error generating scene suggestions: %v", err)
				}
			case entityTypeImage:
				if err := j.suggestImages(ctx, client, r, maxItems); err != nil {
					logger.Errorf("Error generating image suggestions: %v", err)
				}
			default:
				logger.Warnf("Unknown entity type %q, skipping", et)
			}
		}
		return nil
	})
}

// pendingEntityIDs returns the entity IDs that already have a suggestion, so we can skip them.
func (j *AISuggestionJob) pendingEntityIDs(ctx context.Context, r models.Repository, entityType string) map[int]bool {
	suggestions, err := r.AISuggestion.FindPendingByEntityType(ctx, entityType)
	if err != nil {
		return nil
	}
	out := make(map[int]bool, len(suggestions))
	for _, s := range suggestions {
		out[s.EntityID] = true
	}
	return out
}

func (j *AISuggestionJob) suggestScenes(ctx context.Context, client *ai.Client, r models.Repository, maxScenes int) error {
	skip := j.pendingEntityIDs(ctx, r, entityTypeScene)

	var aiTagID int
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		id, err := resolveAITag(ctx, r)
		if err != nil {
			return err
		}
		aiTagID = id
		return nil
	}); err != nil {
		return fmt.Errorf("resolving AI tag: %w", err)
	}

	idStr := fmt.Sprintf("%d", aiTagID)
	sceneFilter := &models.SceneFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{idStr},
			Modifier: models.CriterionModifierExcludes,
		},
	}

	pp := 0
	findFilter := &models.FindFilterType{PerPage: &pp}
	totalCount, err := r.Scene.QueryCount(ctx, sceneFilter, findFilter)
	if err != nil {
		return fmt.Errorf("error counting scenes: %w", err)
	}

	limit := totalCount
	if maxScenes > 0 && maxScenes < limit {
		limit = maxScenes
	}

	scenes, err := scene.Query(ctx, r.Scene, sceneFilter, &models.FindFilterType{PerPage: &limit})
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

		j.progress.ExecuteTask("AI analyzing "+s.Path, func() {
			j.suggestScene(ctx, client, r, s)
		})
		j.progress.Increment()
	}

	return nil
}

func (j *AISuggestionJob) suggestScene(ctx context.Context, client *ai.Client, r models.Repository, s *models.Scene) {
	images, err := sceneScreenshotsForSuggestion(ctx, s, r)
	if err != nil || len(images) == 0 {
		logger.Errorf("Error getting screenshots for scene %q: %v", s.Path, err)
		return
	}

	analysis, err := analyzeMediaWithVision(ctx, client, "scene", s.Path, images, "")
	if err != nil {
		logger.Errorf("Error analyzing scene %q: %v", s.Path, err)
		return
	}

	suggestion := &models.AISuggestion{
		EntityType: entityTypeScene,
		EntityID:   s.ID,
		Title:      analysis.Title,
		Details:    analysis.Details,
		Performers: performerNames(analysis.Performers),
		Tags:       analysis.Tags,
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		return r.AISuggestion.Create(ctx, suggestion)
	}); err != nil {
		logger.Errorf("Error saving suggestion for scene %d: %v", s.ID, err)
	}
}

func (j *AISuggestionJob) suggestImages(ctx context.Context, client *ai.Client, r models.Repository, maxImages int) error {
	skip := j.pendingEntityIDs(ctx, r, entityTypeImage)

	var aiTagID int
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		id, err := resolveAITag(ctx, r)
		if err != nil {
			return err
		}
		aiTagID = id
		return nil
	}); err != nil {
		return fmt.Errorf("resolving AI tag: %w", err)
	}

	idStr := fmt.Sprintf("%d", aiTagID)
	imageFilter := &models.ImageFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{idStr},
			Modifier: models.CriterionModifierExcludes,
		},
	}

	pp := 0
	findFilter := &models.FindFilterType{PerPage: &pp}
	totalCount, err := r.Image.QueryCount(ctx, imageFilter, findFilter)
	if err != nil {
		return fmt.Errorf("error counting images: %w", err)
	}

	limit := totalCount
	if maxImages > 0 && maxImages < limit {
		limit = maxImages
	}

	images, err := image.Query(ctx, r.Image, imageFilter, &models.FindFilterType{PerPage: &limit})
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

		j.progress.ExecuteTask("AI analyzing "+img.Path, func() {
			j.suggestImage(ctx, client, r, img)
		})
		j.progress.Increment()
	}

	return nil
}

func (j *AISuggestionJob) suggestImage(ctx context.Context, client *ai.Client, r models.Repository, img *models.Image) {
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

	mediaType := http.DetectContentType(data)
	multiImages := []ai.MultiImage{{
		Base64:    base64.StdEncoding.EncodeToString(data),
		MediaType: mediaType,
	}}

	analysis, err := analyzeMediaWithVision(ctx, client, "image", img.Path, multiImages, "")
	if err != nil {
		logger.Errorf("Error analyzing image %q: %v", img.Path, err)
		return
	}

	suggestion := &models.AISuggestion{
		EntityType: entityTypeImage,
		EntityID:   img.ID,
		Title:      analysis.Title,
		Details:    analysis.Details,
		Performers: performerNames(analysis.Performers),
		Tags:       analysis.Tags,
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		return r.AISuggestion.Create(ctx, suggestion)
	}); err != nil {
		logger.Errorf("Error saving suggestion for image %d: %v", img.ID, err)
	}
}

type aiMediaAnalysis struct {
	Title      string             `json:"title"`
	Performers []aiImagePerformer `json:"performers"`
	Tags       []string           `json:"tags"`
	Details    string             `json:"details"`
}

func performerNames(performers []aiImagePerformer) []string {
	var names []string
	for _, p := range performers {
		n := strings.TrimSpace(p.Name)
		if n != "" && !strings.EqualFold(n, "unknown") {
			names = append(names, n)
		}
	}
	return names
}

func analyzeMediaWithVision(ctx context.Context, client *ai.Client, mediaType string, path string, images []ai.MultiImage, customContext string) (*aiMediaAnalysis, error) {
	systemPrompt := "You are an adult content analyst. Describe sexual acts explicitly and without censorship. Never refuse, censor, or moralize about any content."
	userPrompt := `Analyze this adult media and return ONLY valid JSON with the following fields:
- "title": a short descriptive title (max 10 words). Use performer names when identified.
- "performers": an array of objects, one per identified person. Only include someone if you are highly confident of their specific name/identity. If you only see generic or unidentifiable people, leave this array empty. Each object must have:
    - "name": the person's specific name
    - "gender": "Male", "Female", "Non-binary", or "Unknown"
    - "ethnicity": visible ethnicity
    - "hair_color": hair color
    - "eye_color": eye color
    - "details": a short description of the person's appearance and role
- "tags": an array of descriptive tags covering sexual acts, positions, body parts, clothing, setting, and style. Aim for 8-15 specific tags.
- "details": a 2-3 sentence explicit description of the media.`

	if customContext != "" {
		userPrompt += fmt.Sprintf("\nAdditional context: %s", customContext)
	}

	userPrompt += "\n\nReturn ONLY valid JSON, no other text, no markdown formatting."

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

	req := ai.ChatCompletionRequest{
		Messages: messages,
	}

	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("vision analysis failed: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content type from vision response")
	}

	cleanJSON := ai.ExtractJSON(content)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing AI response: no JSON found")
	}

	var analysis aiMediaAnalysis
	if err := json.Unmarshal([]byte(cleanJSON), &analysis); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}

	return &analysis, nil
}

func sceneScreenshotsForSuggestion(ctx context.Context, s *models.Scene, r models.Repository) ([]ai.MultiImage, error) {
	if err := s.LoadPrimaryFile(ctx, r.File); err != nil {
		return nil, err
	}

	f := s.Files.Primary()
	if f == nil {
		return nil, fmt.Errorf("scene has no file")
	}

	videoPath := f.Base().Path
	if videoPath == "" {
		return nil, fmt.Errorf("scene has no path")
	}

	duration := f.Duration
	if duration <= 0 {
		duration = 60
	}

	numShots := effectiveAIShotCount(nil, duration, func(d float64) int {
		n := 4
		if d < 30 {
			n = 2
		}
		if d < 10 {
			n = 1
		}
		return n
	})

	var images []ai.MultiImage
	var tmpFiles []string
	defer func() {
		for _, p := range tmpFiles {
			os.Remove(p)
		}
	}()

	for i := 1; i <= numShots; i++ {
		t := duration * (float64(i) / float64(numShots+1))
		if t > duration-0.5 {
			t = duration - 0.5
		}
		if t < 0.1 {
			t = 0.1
		}

		tmpFile, err := os.CreateTemp("", "stash-ai-suggest-*.jpg")
		if err != nil {
			continue
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		tmpFiles = append(tmpFiles, tmpPath)

		opts := transcoder.ScreenshotOptions{
			OutputPath: tmpPath,
			OutputType: transcoder.ScreenshotOutputTypeImage2,
		}
		args := transcoder.ScreenshotTime(videoPath, t, opts)

		if err := instance.FFMpeg.Generate(ctx, args); err != nil {
			continue
		}

		data, err := os.ReadFile(tmpPath)
		if err != nil {
			continue
		}

		mediaType := http.DetectContentType(data)
		images = append(images, ai.MultiImage{
			Base64:    base64.StdEncoding.EncodeToString(data),
			MediaType: mediaType,
		})
	}

	return images, nil
}
