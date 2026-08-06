package manager

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
	"github.com/stashapp/stash/pkg/scene/generate"
)

const entityTypeScene = "scene"
const entityTypePerformer = "performer"
const entityTypeImage = "image"
const entityTypeGallery = "gallery"
const entityTypeStudio = "studio"
const entityTypeTag = "tag"

type AIEmbeddingInput struct {
	EntityTypes []string `json:"entity_types"`
	Overwrite   bool     `json:"overwrite"`
	Visual      bool     `json:"visual"`
	StaleOnly   bool     `json:"stale_only"`
}

type AIEmbeddingJob struct {
	input    AIEmbeddingInput
	progress *job.Progress

	embeddingDiagnosticsLogged bool
	staleIDs                   map[string]map[int]bool
}

func CreateAIEmbeddingJob(input AIEmbeddingInput) *AIEmbeddingJob {
	return &AIEmbeddingJob{
		input: input,
	}
}

func (j *AIEmbeddingJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	logger.Infof("Starting embedding job with entity types: %v, overwrite: %v, visual: %v", j.input.EntityTypes, j.input.Overwrite, j.input.Visual)

	if !instance.Config.GetAIEnabled() {
		logger.Error("AI is not enabled")
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	baseURL := instance.Config.GetAIBaseURL()
	model := instance.Config.GetAIModel()
	embedModel := instance.Config.GetAIEmbeddingModel()
	if embedModel == "" {
		embedModel = model
	}

	imageEmbedModel := instance.Config.GetAIImageEmbeddingModel()
	if imageEmbedModel == "" {
		imageEmbedModel = embedModel
	}

	endpoint := instance.Config.GetAIEndpoint()
	imageEmbedBaseURL := instance.Config.GetAIImageEmbeddingBaseURL()
	if imageEmbedBaseURL == "" {
		imageEmbedBaseURL = baseURL
	}

	logger.Infof("Embedding job config: baseURL=%s, model=%s, embedModel=%s, imageEmbedModel=%s, imageEmbedBaseURL=%s, endpoint=%s", baseURL, model, embedModel, imageEmbedModel, imageEmbedBaseURL, endpoint)

	if baseURL == "" {
		logger.Error("AI base URL is not configured")
		return fmt.Errorf("AI base URL is not configured. Configure it in Settings > AI")
	}

	entityTypes := j.input.EntityTypes
	if len(entityTypes) == 0 {
		entityTypes = []string{entityTypeScene, entityTypePerformer, entityTypeImage, entityTypeGallery, entityTypeStudio, entityTypeTag}
	}

	// Only entities with image content can be embedded visually.
	if j.input.Visual {
		visualEntityTypes := []string{entityTypeScene, entityTypePerformer, entityTypeImage, entityTypeGallery}
		if len(j.input.EntityTypes) == 0 {
			entityTypes = visualEntityTypes
		} else {
			filtered := make([]string, 0, len(entityTypes))
			for _, et := range entityTypes {
				if slices.Contains(visualEntityTypes, et) {
					filtered = append(filtered, et)
				} else {
					logger.Warnf("entity type %q has no image content, skipping in visual mode", et)
				}
			}
			entityTypes = filtered
		}
	}

	if len(entityTypes) == 0 {
		logger.Warn("No entity types to process")
		return nil
	}

	var client *ai.Client
	modelForEmbedding := embedModel
	if j.input.Visual {
		client = ai.NewEmbeddingClient(imageEmbedBaseURL, imageEmbedModel, endpoint)
		modelForEmbedding = imageEmbedModel
	} else {
		client = ai.NewEmbeddingClient(baseURL, embedModel, endpoint)
	}

	r := instance.Repository

	if j.input.StaleOnly {
		stale := make(map[string]map[int]bool)
		for _, et := range entityTypes {
			ids, err := r.Embedding.FindStale(ctx, et, modelForEmbedding)
			if err != nil {
				logger.Warnf("error finding stale embeddings for %s: %v", et, err)
				continue
			}
			set := make(map[int]bool, len(ids))
			for _, id := range ids {
				set[id] = true
			}
			stale[et] = set
			logger.Infof("Embedding job: %d stale %s embeddings to refresh", len(ids), et)
		}
		j.staleIDs = stale
	}

	// The embedding helpers access the database both inside explicit
	// transactions (for writes) and directly (e.g. loading primary image
	// files, performer images, scene files). Wrap the whole job in
	// WithDatabase so the direct database access has a connection available;
	// otherwise those calls fail with "not in transaction".
	return r.WithDB(ctx, func(ctx context.Context) error {
		if j.input.Overwrite {
			if err := r.WithTxn(ctx, func(ctx context.Context) error {
				return r.Embedding.DeleteByModel(ctx, modelForEmbedding)
			}); err != nil {
				return fmt.Errorf("clearing embeddings: %w", err)
			}
		}

		total := 0
		for _, et := range entityTypes {
			if job.IsCancelled(ctx) {
				logger.Infof("Embedding job cancelled after %d embeddings generated", total)
				return nil
			}
			count, err := j.processEntityType(ctx, client, r, et, modelForEmbedding)
			if err != nil {
				return fmt.Errorf("processing %s: %w", et, err)
			}
			total += count
		}

		logger.Infof("Embedding job complete: %d embeddings generated using model %s", total, modelForEmbedding)
		return nil
	})
}

// logEmbeddingError logs an embedding failure and, once per job run, queries
// the AI server for the available models so identifier mismatches (e.g. LM
// Studio's "No models loaded" error when the requested model id does not match
// a cached model) are easy to diagnose.
func (j *AIEmbeddingJob) logEmbeddingError(ctx context.Context, client *ai.Client, model string, format string, args ...interface{}) {
	logger.Warnf(format, args...)
	if j.embeddingDiagnosticsLogged {
		return
	}
	j.embeddingDiagnosticsLogged = true

	available, err := client.ListModels(ctx)
	if err != nil {
		logger.Warnf("embedding model diagnostics: could not list models from the AI server: %v", err)
		return
	}

	logger.Warnf("embedding model diagnostics: configured embedding model %q; models available on the server: %v", model, available)
	for _, m := range available {
		if m == model {
			logger.Warnf("embedding model diagnostics: model %q is available on the server; load it there (e.g. the LM Studio Developer page) and retry", model)
			return
		}
	}
	logger.Warnf("embedding model diagnostics: model %q was not found among the server's models; set the Embedding Model setting to one of: %v", model, available)
}

// shouldEmbed reports whether the given entity should be embedded in this
// run, skipping non-stale entities when staleOnly is set.
func (j *AIEmbeddingJob) shouldEmbed(entityType string, entityID int) bool {
	if !j.input.StaleOnly {
		return true
	}
	return j.staleIDs[entityType][entityID]
}

func (j *AIEmbeddingJob) processEntityType(ctx context.Context, client *ai.Client, r models.Repository, entityType string, model string) (int, error) {
	switch entityType {
	case entityTypeScene:
		if j.input.Visual {
			return j.embedScenesVisual(ctx, client, r, model)
		}
		return j.embedScenes(ctx, client, r, model)
	case entityTypePerformer:
		if j.input.Visual {
			return j.embedPerformersVisual(ctx, client, r, model)
		}
		return j.embedPerformers(ctx, client, r, model)
	case entityTypeImage:
		if j.input.Visual {
			return j.embedImagesVisual(ctx, client, r, model)
		}
		return j.embedImages(ctx, client, r, model)
	case entityTypeGallery:
		if j.input.Visual {
			return j.embedGalleriesVisual(ctx, client, r, model)
		}
		return j.embedGalleries(ctx, client, r, model)
	case entityTypeStudio:
		return j.embedStudios(ctx, client, r, model)
	case entityTypeTag:
		return j.embedTags(ctx, client, r, model)
	default:
		return 0, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

func buildSceneText(ctx context.Context, r models.Repository, sceneID int) (string, error) {
	s, err := r.Scene.Find(ctx, sceneID)
	if err != nil || s == nil {
		return "", err
	}
	var parts []string
	if s.Title != "" {
		parts = append(parts, "Title: "+s.Title)
	}
	if s.Details != "" {
		parts = append(parts, "Details: "+s.Details)
	}
	if s.Director != "" {
		parts = append(parts, "Director: "+s.Director)
	}
	if s.Code != "" {
		parts = append(parts, "Code: "+s.Code)
	}
	if s.Date != nil {
		parts = append(parts, "Date: "+s.Date.String())
	}
	if s.StudioID != nil {
		studio, _ := r.Studio.Find(ctx, *s.StudioID)
		if studio != nil {
			parts = append(parts, "Studio: "+studio.Name)
		}
	}
	_ = s.LoadTagIDs(ctx, r.Scene)
	if len(s.TagIDs.List()) > 0 {
		tags, _ := r.Tag.FindMany(ctx, s.TagIDs.List())
		var names []string
		for _, t := range tags {
			names = append(names, t.Name)
		}
		parts = append(parts, "Tags: "+strings.Join(names, ", "))
	}
	_ = s.LoadPerformerIDs(ctx, r.Scene)
	if len(s.PerformerIDs.List()) > 0 {
		performers, _ := r.Performer.FindMany(ctx, s.PerformerIDs.List())
		var names []string
		for _, p := range performers {
			names = append(names, p.Name)
		}
		parts = append(parts, "Performers: "+strings.Join(names, ", "))
	}
	return strings.Join(parts, "\n"), nil
}

func buildPerformerText(ctx context.Context, r models.Repository, performerID int) (string, error) {
	p, err := r.Performer.Find(ctx, performerID)
	if err != nil || p == nil {
		return "", err
	}
	_ = p.LoadAliases(ctx, r.Performer)
	var parts []string
	parts = append(parts, "Name: "+p.Name)
	if len(p.Aliases.List()) > 0 {
		parts = append(parts, "Aliases: "+strings.Join(p.Aliases.List(), ", "))
	}
	if p.Gender != nil {
		parts = append(parts, "Gender: "+p.Gender.String())
	}
	if p.Ethnicity != "" {
		parts = append(parts, "Ethnicity: "+p.Ethnicity)
	}
	if p.Country != "" {
		parts = append(parts, "Country: "+p.Country)
	}
	if p.Details != "" {
		parts = append(parts, "Details: "+p.Details)
	}
	if p.Disambiguation != "" {
		parts = append(parts, "Also known as: "+p.Disambiguation)
	}
	return strings.Join(parts, "\n"), nil
}

func buildImageText(ctx context.Context, r models.Repository, imageID int) (string, error) {
	img, err := r.Image.Find(ctx, imageID)
	if err != nil || img == nil {
		return "", err
	}
	var parts []string
	if img.Title != "" {
		parts = append(parts, "Title: "+img.Title)
	}
	if img.Details != "" {
		parts = append(parts, "Details: "+img.Details)
	}
	if img.Code != "" {
		parts = append(parts, "Code: "+img.Code)
	}
	if img.Photographer != "" {
		parts = append(parts, "Photographer: "+img.Photographer)
	}
	if img.Date != nil {
		parts = append(parts, "Date: "+img.Date.String())
	}
	if img.StudioID != nil {
		studio, _ := r.Studio.Find(ctx, *img.StudioID)
		if studio != nil {
			parts = append(parts, "Studio: "+studio.Name)
		}
	}
	_ = img.LoadTagIDs(ctx, r.Image)
	if len(img.TagIDs.List()) > 0 {
		tags, _ := r.Tag.FindMany(ctx, img.TagIDs.List())
		var names []string
		for _, t := range tags {
			names = append(names, t.Name)
		}
		parts = append(parts, "Tags: "+strings.Join(names, ", "))
	}
	_ = img.LoadPerformerIDs(ctx, r.Image)
	if len(img.PerformerIDs.List()) > 0 {
		performers, _ := r.Performer.FindMany(ctx, img.PerformerIDs.List())
		var names []string
		for _, p := range performers {
			names = append(names, p.Name)
		}
		parts = append(parts, "Performers: "+strings.Join(names, ", "))
	}
	return strings.Join(parts, "\n"), nil
}

func buildGalleryText(ctx context.Context, r models.Repository, galleryID int) (string, error) {
	g, err := r.Gallery.Find(ctx, galleryID)
	if err != nil || g == nil {
		return "", err
	}
	var parts []string
	if g.Title != "" {
		parts = append(parts, "Title: "+g.Title)
	}
	if g.Details != "" {
		parts = append(parts, "Details: "+g.Details)
	}
	if g.Photographer != "" {
		parts = append(parts, "Photographer: "+g.Photographer)
	}
	if g.Code != "" {
		parts = append(parts, "Code: "+g.Code)
	}
	if g.Date != nil {
		parts = append(parts, "Date: "+g.Date.String())
	}
	_ = g.LoadTagIDs(ctx, r.Gallery)
	if len(g.TagIDs.List()) > 0 {
		tags, _ := r.Tag.FindMany(ctx, g.TagIDs.List())
		var names []string
		for _, t := range tags {
			names = append(names, t.Name)
		}
		parts = append(parts, "Tags: "+strings.Join(names, ", "))
	}
	_ = g.LoadPerformerIDs(ctx, r.Gallery)
	if len(g.PerformerIDs.List()) > 0 {
		performers, _ := r.Performer.FindMany(ctx, g.PerformerIDs.List())
		var names []string
		for _, p := range performers {
			names = append(names, p.Name)
		}
		parts = append(parts, "Performers: "+strings.Join(names, ", "))
	}
	return strings.Join(parts, "\n"), nil
}

func buildStudioText(ctx context.Context, r models.Repository, studioID int) (string, error) {
	st, err := r.Studio.Find(ctx, studioID)
	if err != nil || st == nil {
		return "", err
	}
	_ = st.LoadAliases(ctx, r.Studio)
	_ = st.LoadURLs(ctx, r.Studio)
	var parts []string
	parts = append(parts, "Name: "+st.Name)
	if len(st.Aliases.List()) > 0 {
		parts = append(parts, "Aliases: "+strings.Join(st.Aliases.List(), ", "))
	}
	if st.Details != "" {
		parts = append(parts, "Details: "+st.Details)
	}
	if len(st.URLs.List()) > 0 {
		parts = append(parts, "URLs: "+strings.Join(st.URLs.List(), ", "))
	}
	if st.ParentID != nil {
		parent, _ := r.Studio.Find(ctx, *st.ParentID)
		if parent != nil {
			parts = append(parts, "Parent Studio: "+parent.Name)
		}
	}
	return strings.Join(parts, "\n"), nil
}

func buildTagText(ctx context.Context, r models.Repository, tagID int) (string, error) {
	t, err := r.Tag.Find(ctx, tagID)
	if err != nil || t == nil {
		return "", err
	}
	_ = t.LoadAliases(ctx, r.Tag)
	var parts []string
	parts = append(parts, "Name: "+t.Name)
	if t.Description != "" {
		parts = append(parts, "Description: "+t.Description)
	}
	if len(t.Aliases.List()) > 0 {
		parts = append(parts, "Aliases: "+strings.Join(t.Aliases.List(), ", "))
	}
	return strings.Join(parts, "\n"), nil
}

func (j *AIEmbeddingJob) embedScenes(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var scenes []*models.Scene
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		scenes, err = scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding scenes: %w", err)
	}
	logger.Infof("embedScenes: found %d scenes", len(scenes))
	j.progress.SetTotal(len(scenes))
	count := 0
	for i, s := range scenes {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypeScene, s.ID) {
			j.progress.Increment()
			continue
		}
		err := r.WithTxn(ctx, func(ctx context.Context) error {
			text, err := buildSceneText(ctx, r, s.ID)
			if err != nil || text == "" {
				logger.Infof("embedScenes: scene %d has no text (err=%v, text_len=%d), skipping", s.ID, err, len(text))
				j.progress.Increment()
				return nil
			}
			logger.Infof("embedScenes: scene %d text length: %d", s.ID, len(text))
			vec, err := client.Embedding(ctx, text)
			if err != nil {
				j.logEmbeddingError(ctx, client, model, "error embedding scene %d: %v", s.ID, err)
				j.progress.Increment()
				return nil
			}
			logger.Infof("embedScenes: successfully embedded scene %d, vector length: %d", s.ID, len(vec))
			if err := r.Embedding.Set(ctx, entityTypeScene, s.ID, model, vec); err != nil {
				logger.Warnf("error storing embedding for scene %d: %v", s.ID, err)
			}
			count++
			j.progress.SetProcessed(i + 1)
			return nil
		})
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func (j *AIEmbeddingJob) embedPerformers(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var performers []*models.Performer
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		performers, _, err = r.Performer.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding performers: %w", err)
	}
	j.progress.SetTotal(len(performers))
	count := 0
	for i, p := range performers {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypePerformer, p.ID) {
			j.progress.Increment()
			continue
		}
		err := r.WithTxn(ctx, func(ctx context.Context) error {
			text, err := buildPerformerText(ctx, r, p.ID)
			if err != nil || text == "" {
				j.progress.Increment()
				return nil
			}
			vec, err := client.Embedding(ctx, text)
			if err != nil {
				j.logEmbeddingError(ctx, client, model, "error embedding performer %d: %v", p.ID, err)
				j.progress.Increment()
				return nil
			}
			if err := r.Embedding.Set(ctx, entityTypePerformer, p.ID, model, vec); err != nil {
				logger.Warnf("error storing embedding for performer %d: %v", p.ID, err)
			}
			count++
			j.progress.SetProcessed(i + 1)
			return nil
		})
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func (j *AIEmbeddingJob) embedImages(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var images []*models.Image
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		images, err = image.Query(ctx, r.Image, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding images: %w", err)
	}
	j.progress.SetTotal(len(images))
	count := 0
	for i, img := range images {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypeImage, img.ID) {
			j.progress.Increment()
			continue
		}
		err := r.WithTxn(ctx, func(ctx context.Context) error {
			text, err := buildImageText(ctx, r, img.ID)
			if err != nil || text == "" {
				j.progress.Increment()
				return nil
			}
			vec, err := client.Embedding(ctx, text)
			if err != nil {
				j.logEmbeddingError(ctx, client, model, "error embedding image %d: %v", img.ID, err)
				j.progress.Increment()
				return nil
			}
			if err := r.Embedding.Set(ctx, entityTypeImage, img.ID, model, vec); err != nil {
				logger.Warnf("error storing embedding for image %d: %v", img.ID, err)
			}
			count++
			j.progress.SetProcessed(i + 1)
			return nil
		})
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func (j *AIEmbeddingJob) embedGalleries(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var galleries []*models.Gallery
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		galleries, _, err = r.Gallery.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding galleries: %w", err)
	}
	j.progress.SetTotal(len(galleries))
	count := 0
	for i, g := range galleries {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypeGallery, g.ID) {
			j.progress.Increment()
			continue
		}
		err := r.WithTxn(ctx, func(ctx context.Context) error {
			text, err := buildGalleryText(ctx, r, g.ID)
			if err != nil || text == "" {
				j.progress.Increment()
				return nil
			}
			vec, err := client.Embedding(ctx, text)
			if err != nil {
				j.logEmbeddingError(ctx, client, model, "error embedding gallery %d: %v", g.ID, err)
				j.progress.Increment()
				return nil
			}
			if err := r.Embedding.Set(ctx, entityTypeGallery, g.ID, model, vec); err != nil {
				logger.Warnf("error storing embedding for gallery %d: %v", g.ID, err)
			}
			count++
			j.progress.SetProcessed(i + 1)
			return nil
		})
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func (j *AIEmbeddingJob) embedStudios(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var studios []*models.Studio
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		studios, _, err = r.Studio.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding studios: %w", err)
	}
	j.progress.SetTotal(len(studios))
	count := 0
	for i, st := range studios {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypeStudio, st.ID) {
			j.progress.Increment()
			continue
		}
		err := r.WithTxn(ctx, func(ctx context.Context) error {
			text, err := buildStudioText(ctx, r, st.ID)
			if err != nil || text == "" {
				j.progress.Increment()
				return nil
			}
			vec, err := client.Embedding(ctx, text)
			if err != nil {
				j.logEmbeddingError(ctx, client, model, "error embedding studio %d: %v", st.ID, err)
				j.progress.Increment()
				return nil
			}
			if err := r.Embedding.Set(ctx, entityTypeStudio, st.ID, model, vec); err != nil {
				logger.Warnf("error storing embedding for studio %d: %v", st.ID, err)
			}
			count++
			j.progress.SetProcessed(i + 1)
			return nil
		})
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func (j *AIEmbeddingJob) embedTags(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var tags []*models.Tag
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		tags, _, err = r.Tag.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding tags: %w", err)
	}
	j.progress.SetTotal(len(tags))
	count := 0
	for i, t := range tags {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypeTag, t.ID) {
			j.progress.Increment()
			continue
		}
		err := r.WithTxn(ctx, func(ctx context.Context) error {
			text, err := buildTagText(ctx, r, t.ID)
			if err != nil || text == "" {
				j.progress.Increment()
				return nil
			}
			vec, err := client.Embedding(ctx, text)
			if err != nil {
				j.logEmbeddingError(ctx, client, model, "error embedding tag %d: %v", t.ID, err)
				j.progress.Increment()
				return nil
			}
			if err := r.Embedding.Set(ctx, entityTypeTag, t.ID, model, vec); err != nil {
				logger.Warnf("error storing embedding for tag %d: %v", t.ID, err)
			}
			count++
			j.progress.SetProcessed(i + 1)
			return nil
		})
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func loadImageFile(ctx context.Context, r models.Repository, img *models.Image) (string, string, error) {
	if err := img.LoadPrimaryFile(ctx, r.File); err != nil {
		return "", "", fmt.Errorf("loading primary file: %w", err)
	}

	f := img.Files.Primary()
	if f == nil {
		return "", "", fmt.Errorf("image has no file")
	}

	data, err := os.ReadFile(f.Base().Path)
	if err != nil {
		return "", "", fmt.Errorf("reading image file: %w", err)
	}

	mediaType := http.DetectContentType(data)
	return base64.StdEncoding.EncodeToString(data), mediaType, nil
}

func (j *AIEmbeddingJob) embedImagesVisual(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var images []*models.Image
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		images, err = image.Query(ctx, r.Image, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding images: %w", err)
	}
	j.progress.SetTotal(len(images))
	count := 0
	for i, img := range images {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypeImage, img.ID) {
			j.progress.Increment()
			continue
		}
		base64Str, mediaType, err := loadImageFile(ctx, r, img)
		if err != nil {
			logger.Warnf("error loading image %d: %v", img.ID, err)
			j.progress.Increment()
			continue
		}
		vec, err := client.EmbeddingImage(ctx, base64Str, mediaType)
		if err != nil {
			j.logEmbeddingError(ctx, client, model, "error embedding image %d: %v", img.ID, err)
			j.progress.Increment()
			continue
		}
		err = r.WithTxn(ctx, func(ctx context.Context) error {
			return r.Embedding.Set(ctx, entityTypeImage, img.ID, model, vec)
		})
		if err != nil {
			logger.Warnf("error storing embedding for image %d: %v", img.ID, err)
		}
		count++
		j.progress.SetProcessed(i + 1)
	}
	return count, nil
}

func (j *AIEmbeddingJob) embedGalleriesVisual(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var galleries []*models.Gallery
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		galleries, _, err = r.Gallery.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding galleries: %w", err)
	}
	j.progress.SetTotal(len(galleries))
	count := 0
	for i, g := range galleries {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypeGallery, g.ID) {
			j.progress.Increment()
			continue
		}
		var imageIDs []int
		var err error
		if imageIDs, err = r.Gallery.GetImageIDs(ctx, g.ID); err != nil {
			logger.Warnf("error loading gallery %d images: %v", g.ID, err)
			j.progress.Increment()
			continue
		}
		if len(imageIDs) == 0 {
			logger.Infof("embedGalleriesVisual: gallery %d has no images, skipping", g.ID)
			j.progress.Increment()
			continue
		}
		img, err := r.Image.Find(ctx, imageIDs[0])
		if err != nil || img == nil {
			logger.Warnf("error loading gallery %d image %d: %v", g.ID, imageIDs[0], err)
			j.progress.Increment()
			continue
		}
		base64Str, mediaType, err := loadImageFile(ctx, r, img)
		if err != nil {
			logger.Warnf("error loading gallery %d image: %v", g.ID, err)
			j.progress.Increment()
			continue
		}
		vec, err := client.EmbeddingImage(ctx, base64Str, mediaType)
		if err != nil {
			j.logEmbeddingError(ctx, client, model, "error embedding gallery %d: %v", g.ID, err)
			j.progress.Increment()
			continue
		}
		err = r.WithTxn(ctx, func(ctx context.Context) error {
			return r.Embedding.Set(ctx, entityTypeGallery, g.ID, model, vec)
		})
		if err != nil {
			logger.Warnf("error storing embedding for gallery %d: %v", g.ID, err)
		}
		count++
		j.progress.SetProcessed(i + 1)
	}
	return count, nil
}

func (j *AIEmbeddingJob) embedPerformersVisual(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var performers []*models.Performer
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		performers, _, err = r.Performer.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding performers: %w", err)
	}
	j.progress.SetTotal(len(performers))
	count := 0
	for i, p := range performers {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypePerformer, p.ID) {
			j.progress.Increment()
			continue
		}
		data, err := r.Performer.GetImage(ctx, p.ID)
		if err != nil || len(data) == 0 {
			logger.Infof("embedPerformersVisual: performer %d has no image, skipping", p.ID)
			j.progress.Increment()
			continue
		}
		mediaType := http.DetectContentType(data)
		base64Str := base64.StdEncoding.EncodeToString(data)
		vec, err := client.EmbeddingImage(ctx, base64Str, mediaType)
		if err != nil {
			j.logEmbeddingError(ctx, client, model, "error embedding performer %d: %v", p.ID, err)
			j.progress.Increment()
			continue
		}
		err = r.WithTxn(ctx, func(ctx context.Context) error {
			return r.Embedding.Set(ctx, entityTypePerformer, p.ID, model, vec)
		})
		if err != nil {
			logger.Warnf("error storing embedding for performer %d: %v", p.ID, err)
		}
		count++
		j.progress.SetProcessed(i + 1)
	}
	return count, nil
}

func (j *AIEmbeddingJob) embedScenesVisual(ctx context.Context, client *ai.Client, r models.Repository, model string) (int, error) {
	var scenes []*models.Scene
	err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		scenes, err = scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("finding scenes: %w", err)
	}
	j.progress.SetTotal(len(scenes))
	count := 0
	for i, s := range scenes {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		default:
		}
		if !j.shouldEmbed(entityTypeScene, s.ID) {
			j.progress.Increment()
			continue
		}
		frame, err := j.sceneVisualFrame(ctx, r, s)
		if err != nil {
			logger.Warnf("error generating frame for scene %d: %v", s.ID, err)
			j.progress.Increment()
			continue
		}
		base64Str := base64.StdEncoding.EncodeToString(frame)
		vec, err := client.EmbeddingImage(ctx, base64Str, "image/jpeg")
		if err != nil {
			j.logEmbeddingError(ctx, client, model, "error embedding scene %d: %v", s.ID, err)
			j.progress.Increment()
			continue
		}
		err = r.WithTxn(ctx, func(ctx context.Context) error {
			return r.Embedding.Set(ctx, entityTypeScene, s.ID, model, vec)
		})
		if err != nil {
			logger.Warnf("error storing embedding for scene %d: %v", s.ID, err)
		}
		count++
		j.progress.SetProcessed(i + 1)
	}
	return count, nil
}

func (j *AIEmbeddingJob) sceneVisualFrame(ctx context.Context, r models.Repository, s *models.Scene) ([]byte, error) {
	if err := s.LoadFiles(ctx, r.Scene); err != nil {
		return nil, fmt.Errorf("loading scene files: %w", err)
	}

	videoFile := s.Files.Primary()
	if videoFile == nil {
		return nil, fmt.Errorf("scene has no video file")
	}

	g := &generate.Generator{
		Encoder:      instance.FFMpeg,
		FFMpegConfig: instance.Config,
		LockManager:  instance.ReadLockManager,
		MarkerPaths:  instance.Paths.SceneMarkers,
		ScenePaths:   instance.Paths.Scene,
		Overwrite:    false,
	}

	frame, err := g.Screenshot(ctx, videoFile.Path, videoFile.Width, videoFile.Duration, generate.ScreenshotOptions{})
	if err != nil {
		return nil, fmt.Errorf("generating screenshot: %w", err)
	}

	return frame, nil
}

func intPtr(i int) *int {
	return &i
}
