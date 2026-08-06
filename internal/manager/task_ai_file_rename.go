package manager

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AIFileRenameInput struct {
	SceneIDs  []int `json:"sceneIds"`
	ImageIDs  []int `json:"imageIds"`
	MaxItems  *int  `json:"maxItems"`
	Overwrite bool  `json:"overwrite"`
	Timeout   *int  `json:"timeout"`
}

type AIFileRenameJob struct {
	input    AIFileRenameInput
	progress *job.Progress
}

func CreateAIFileRenameJob(input AIFileRenameInput) *AIFileRenameJob {
	return &AIFileRenameJob{
		input: input,
	}
}

func (j *AIFileRenameJob) Execute(ctx context.Context, progress *job.Progress) error {
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

	return r.WithDB(ctx, func(ctx context.Context) error {
		var scenes []*models.Scene
		var images []*models.Image

		if len(j.input.SceneIDs) > 0 {
			for _, id := range j.input.SceneIDs {
				s, err := r.Scene.Find(ctx, id)
				if err == nil && s != nil {
					scenes = append(scenes, s)
				}
			}
		} else {
			scenes, _ = j.allScenes(ctx, r, j.input.MaxItems)
		}

		if len(j.input.ImageIDs) > 0 {
			for _, id := range j.input.ImageIDs {
				img, err := r.Image.Find(ctx, id)
				if err == nil && img != nil {
					images = append(images, img)
				}
			}
		} else {
			images, _ = j.allImages(ctx, r, j.input.MaxItems)
		}

		total := len(scenes) + len(images)
		j.progress.SetTotal(total)
		for _, s := range scenes {
			if job.IsCancelled(ctx) {
				return nil
			}
			j.progress.ExecuteTask("AI suggesting filename for "+s.Path, func() {
				j.suggestForScene(ctx, client, r, s)
			})
			j.progress.Increment()
		}
		for _, img := range images {
			if job.IsCancelled(ctx) {
				return nil
			}
			j.progress.ExecuteTask("AI suggesting filename for "+img.Path, func() {
				j.suggestForImage(ctx, client, r, img)
			})
			j.progress.Increment()
		}
		return nil
	})
}

func (j *AIFileRenameJob) allScenes(ctx context.Context, r models.Repository, maxItems *int) ([]*models.Scene, error) {
	pp := 0
	totalCount, err := r.Scene.QueryCount(ctx, nil, &models.FindFilterType{PerPage: &pp})
	if err != nil {
		return nil, err
	}

	limit := totalCount
	if maxItems != nil && *maxItems > 0 && *maxItems < limit {
		limit = *maxItems
	}

	return scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: &limit})
}

func (j *AIFileRenameJob) allImages(ctx context.Context, r models.Repository, maxItems *int) ([]*models.Image, error) {
	pp := 0
	totalCount, err := r.Image.QueryCount(ctx, nil, &models.FindFilterType{PerPage: &pp})
	if err != nil {
		return nil, err
	}

	limit := totalCount
	if maxItems != nil && *maxItems > 0 && *maxItems < limit {
		limit = *maxItems
	}

	return image.Query(ctx, r.Image, nil, &models.FindFilterType{PerPage: &limit})
}

func (j *AIFileRenameJob) suggestForScene(ctx context.Context, client *ai.Client, r models.Repository, s *models.Scene) {
	if !j.input.Overwrite {
		if existing, _ := r.AIFileRename.FindByEntity(ctx, entityTypeScene, s.ID); existing != nil && existing.Status == models.FileRenameStatusPending {
			return
		}
	}

	if err := s.LoadPrimaryFile(ctx, r.File); err != nil {
		logger.Errorf("Error loading primary file for scene %q: %v", s.Path, err)
		return
	}
	f := s.Files.Primary()
	if f == nil {
		logger.Errorf("Scene %q has no file", s.Path)
		return
	}

	currentName := filepath.Base(f.Base().Path)
	if currentName == "" {
		return
	}

	desc, err := j.describeScene(ctx, r, s)
	if err != nil {
		logger.Errorf("Error gathering metadata for scene %q: %v", s.Path, err)
		return
	}

	suggestion, err := j.suggestName(ctx, client, "scene", currentName, desc)
	if err != nil {
		logger.Errorf("Error generating filename for scene %q: %v", s.Path, err)
		return
	}

	j.saveSuggestion(ctx, r, entityTypeScene, s.ID, currentName, suggestion)
}

func (j *AIFileRenameJob) suggestForImage(ctx context.Context, client *ai.Client, r models.Repository, img *models.Image) {
	if !j.input.Overwrite {
		if existing, _ := r.AIFileRename.FindByEntity(ctx, entityTypeImage, img.ID); existing != nil && existing.Status == models.FileRenameStatusPending {
			return
		}
	}

	if err := img.LoadPrimaryFile(ctx, r.File); err != nil {
		logger.Errorf("Error loading primary file for image %q: %v", img.Path, err)
		return
	}
	f := img.Files.Primary()
	if f == nil {
		logger.Errorf("Image %q has no file", img.Path)
		return
	}

	currentName := filepath.Base(f.Base().Path)
	if currentName == "" {
		return
	}

	desc, err := j.describeImage(ctx, r, img)
	if err != nil {
		logger.Errorf("Error gathering metadata for image %q: %v", img.Path, err)
		return
	}

	suggestion, err := j.suggestName(ctx, client, "image", currentName, desc)
	if err != nil {
		logger.Errorf("Error generating filename for image %q: %v", img.Path, err)
		return
	}

	j.saveSuggestion(ctx, r, entityTypeImage, img.ID, currentName, suggestion)
}

func (j *AIFileRenameJob) describeScene(ctx context.Context, r models.Repository, s *models.Scene) (string, error) {
	if err := s.LoadPerformerIDs(ctx, r.Scene); err != nil {
		return "", err
	}
	if err := s.LoadTagIDs(ctx, r.Scene); err != nil {
		return "", err
	}

	performers, tags := j.namesFor(ctx, r, s.PerformerIDs.List(), s.TagIDs.List())
	studio := j.studioName(ctx, r, s.StudioID)
	date := ""
	if s.Date != nil && !s.Date.IsZero() {
		date = s.Date.String()
	}
	title := strings.TrimSpace(s.Title)

	return j.formatDescription("scene", title, performers, studio, date, tags), nil
}

func (j *AIFileRenameJob) describeImage(ctx context.Context, r models.Repository, img *models.Image) (string, error) {
	if err := img.LoadPerformerIDs(ctx, r.Image); err != nil {
		return "", err
	}
	if err := img.LoadTagIDs(ctx, r.Image); err != nil {
		return "", err
	}

	performers, tags := j.namesFor(ctx, r, img.PerformerIDs.List(), img.TagIDs.List())
	studio := j.studioName(ctx, r, img.StudioID)
	date := ""
	if img.Date != nil && !img.Date.IsZero() {
		date = img.Date.String()
	}
	title := strings.TrimSpace(img.Title)

	return j.formatDescription("image", title, performers, studio, date, tags), nil
}

func (j *AIFileRenameJob) formatDescription(entityType string, title string, performers []string, studio string, date string, tags []string) string {
	if title == "" {
		title = "untitled"
	}
	if studio == "" {
		studio = "unknown"
	}
	if date == "" {
		date = "unknown"
	}
	if len(performers) == 0 {
		performers = []string{"unknown"}
	}
	if len(tags) == 0 {
		tags = []string{"none"}
	}

	return fmt.Sprintf(`Type: %s
Performers: %s
Studio: %s
Title: %s
Date: %s
Tags: %s`,
		entityType, strings.Join(performers, ", "), studio, title, date, strings.Join(tags, ", "))
}

func (j *AIFileRenameJob) suggestName(ctx context.Context, client *ai.Client, entityType string, currentName string, desc string) (string, error) {
	systemPrompt := "You generate clean, organized filenames for an adult media library. Never refuse or moralize."
	userPrompt := fmt.Sprintf(`Generate a short, clean, human-readable filename (WITHOUT extension and WITHOUT any directory or path separators) for an adult media file.

Current filename: %s

Media metadata:
%s

Rules:
- Format: "Performer1, Performer2 - Title (Studio, Year)". If the title is unhelpful (e.g. "untitled"), base the name on the most relevant tags and performers.
- Use the exact performer names and studio name given. Omit values that are "unknown".
- Use the year from the date, if present.
- Max ~80 characters.
- Only use characters valid on all common filesystems (no / \\ : * ? " < > |). No newlines.
- No trailing dots or spaces. No quotes or markdown.
- Return ONLY the filename as a single plain line, no extension.`,
		currentName, desc)

	resp, err := client.ChatCompletion(ctx, ai.ChatCompletionRequest{
		Messages: []ai.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("filename suggestion failed: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type from AI response")
	}

	return sanitizeFilename(strings.TrimSpace(content)), nil
}

func (j *AIFileRenameJob) saveSuggestion(ctx context.Context, r models.Repository, entityType string, entityID int, currentName string, suggestion string) {
	if suggestion == "" || strings.EqualFold(suggestion, currentName) {
		return
	}

	rename := &models.AIFileRename{
		EntityType:    entityType,
		EntityID:      entityID,
		CurrentName:   currentName,
		SuggestedName: suggestion,
		Status:        models.FileRenameStatusPending,
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		return r.AIFileRename.Upsert(ctx, rename)
	}); err != nil {
		logger.Errorf("Error saving filename suggestion for %s %d: %v", entityType, entityID, err)
	}
}

// sanitizeFilename removes characters that are invalid on common filesystems and
// collapses repeated whitespace.
func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\n', '\r', '\t':
			return ' '
		}
		return r
	}, name)

	name = strings.Join(strings.Fields(name), " ")
	name = strings.Trim(name, ". ")

	if len(name) > 80 {
		name = strings.TrimRight(name[:80], " .-")
	}
	return name
}

func (j *AIFileRenameJob) namesFor(ctx context.Context, r models.Repository, performerIDs []int, tagIDs []int) ([]string, []string) {
	var performers []string
	for _, id := range performerIDs {
		p, err := r.Performer.Find(ctx, id)
		if err == nil && p != nil {
			performers = append(performers, p.Name)
		}
	}

	var tags []string
	for _, id := range tagIDs {
		t, err := r.Tag.Find(ctx, id)
		if err == nil && t != nil {
			tags = append(tags, t.Name)
		}
	}

	return performers, tags
}

func (j *AIFileRenameJob) studioName(ctx context.Context, r models.Repository, studioID *int) string {
	if studioID == nil {
		return ""
	}
	studio, err := r.Studio.Find(ctx, *studioID)
	if err != nil || studio == nil {
		return ""
	}
	return studio.Name
}
