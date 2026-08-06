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
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AIPerformerClusterInput struct {
	PerformerIDs         []int    `json:"performerIds"`
	EntityTypes          []string `json:"entityTypes"`
	MaxMediaPerPerformer *int     `json:"maxMediaPerPerformer"`
	MinConfidence        *float64 `json:"minConfidence"`
	Timeout              *int     `json:"timeout"`
	Overwrite            bool     `json:"overwrite"`
}

type AIPerformerClusterJob struct {
	input    AIPerformerClusterInput
	progress *job.Progress
}

func CreateAIPerformerClusterJob(input AIPerformerClusterInput) *AIPerformerClusterJob {
	return &AIPerformerClusterJob{
		input: input,
	}
}

type aiFaceMatchResult struct {
	Present    bool    `json:"present"`
	Confidence float64 `json:"confidence"`
}

func (j *AIPerformerClusterJob) Execute(ctx context.Context, progress *job.Progress) error {
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

	maxMedia := 0
	if j.input.MaxMediaPerPerformer != nil {
		maxMedia = *j.input.MaxMediaPerPerformer
	}

	minConfidence := instance.Config.GetAIPerformerClusterMinConfidence()
	if j.input.MinConfidence != nil {
		minConfidence = *j.input.MinConfidence
	}

	overwrite := j.input.Overwrite

	return r.WithDB(ctx, func(ctx context.Context) error {
		performers, err := j.getPerformers(ctx, r)
		if err != nil {
			return err
		}
		if len(performers) == 0 {
			logger.Info("No performers to cluster")
			return nil
		}

		for _, p := range performers {
			if job.IsCancelled(ctx) {
				return nil
			}

			imageData, err := r.Performer.GetImage(ctx, p.ID)
			if err != nil || len(imageData) == 0 {
				logger.Warnf("Performer %q has no reference image, skipping", p.Name)
				continue
			}

			if err := j.clusterPerformer(ctx, client, r, p, imageData, entityTypes, maxMedia, minConfidence, overwrite); err != nil {
				logger.Errorf("Error clustering performer %q: %v", p.Name, err)
			}
		}

		return nil
	})
}

func (j *AIPerformerClusterJob) getPerformers(ctx context.Context, r models.Repository) ([]*models.Performer, error) {
	if len(j.input.PerformerIDs) > 0 {
		// Filter out invalid IDs (0 is not a valid performer ID)
		validIDs := make([]int, 0, len(j.input.PerformerIDs))
		for _, id := range j.input.PerformerIDs {
			if id > 0 {
				validIDs = append(validIDs, id)
			}
		}
		if len(validIDs) == 0 {
			return nil, nil
		}
		performers, err := r.Performer.FindMany(ctx, validIDs)
		if err != nil {
			return nil, fmt.Errorf("finding performers: %w", err)
		}
		var out []*models.Performer
		for _, p := range performers {
			if p != nil {
				out = append(out, p)
			}
		}
		return out, nil
	}

	performers, _, err := r.Performer.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
	if err != nil {
		return nil, fmt.Errorf("querying performers: %w", err)
	}
	return performers, nil
}

func (j *AIPerformerClusterJob) clusterPerformer(
	ctx context.Context,
	client *ai.Client,
	r models.Repository,
	performer *models.Performer,
	refImage []byte,
	entityTypes []string,
	maxMedia int,
	minConfidence float64,
	overwrite bool,
) error {
	var mediaCount int

	for _, entityType := range entityTypes {
		if job.IsCancelled(ctx) {
			return nil
		}

		switch entityType {
		case entityTypeScene:
			count, err := j.clusterScenes(ctx, client, r, performer, refImage, maxMedia, minConfidence, overwrite)
			if err != nil {
				logger.Warnf("Error clustering scenes for performer %q: %v", performer.Name, err)
			}
			mediaCount += count
		case entityTypeImage:
			count, err := j.clusterImages(ctx, client, r, performer, refImage, maxMedia, minConfidence, overwrite)
			if err != nil {
				logger.Warnf("Error clustering images for performer %q: %v", performer.Name, err)
			}
			mediaCount += count
		}
	}

	logger.Infof("Performer clustering complete for %q: matched in %d new media items", performer.Name, mediaCount)
	return nil
}

// performerClusterFindFilter returns a find filter for querying candidate
// media for clustering. maxMedia <= 0 means no limit (all results), which
// requires PerPageAll (-1) rather than 0 to avoid a LIMIT 0 query.
func performerClusterFindFilter(maxMedia int) *models.FindFilterType {
	perPage := models.PerPageAll
	if maxMedia > 0 {
		perPage = maxMedia
	}
	return &models.FindFilterType{PerPage: &perPage}
}

// scenesWithoutPerformer returns scenes that do not currently list the given performer.
// If overwrite is true, returns all scenes instead.
func (j *AIPerformerClusterJob) scenesWithoutPerformer(ctx context.Context, r models.Repository, performerID int, maxScenes int, overwrite bool) ([]*models.Scene, error) {
	var sceneFilter *models.SceneFilterType
	if !overwrite {
		sceneFilter = &models.SceneFilterType{
			Performers: &models.MultiCriterionInput{
				Value:    []string{fmt.Sprintf("%d", performerID)},
				Modifier: models.CriterionModifierExcludes,
			},
		}
	}

	return scene.Query(ctx, r.Scene, sceneFilter, performerClusterFindFilter(maxScenes))
}

func (j *AIPerformerClusterJob) clusterScenes(
	ctx context.Context,
	client *ai.Client,
	r models.Repository,
	performer *models.Performer,
	refImage []byte,
	maxScenes int,
	minConfidence float64,
	overwrite bool,
) (int, error) {
	scenes, err := j.scenesWithoutPerformer(ctx, r, performer.ID, maxScenes, overwrite)
	if err != nil {
		return 0, fmt.Errorf("querying scenes: %w", err)
	}

	if len(scenes) == 0 {
		return 0, nil
	}

	logger.Infof("Clustering %d scenes for performer %q (overwrite=%v)", len(scenes), performer.Name, overwrite)
	j.progress.SetTotal(len(scenes))
	count := 0
	for _, s := range scenes {
		if job.IsCancelled(ctx) {
			return count, nil
		}

		screenshots, err := j.sceneScreenshotsForCluster(ctx, s, r)
		if err != nil || len(screenshots) == 0 {
			logger.Debugf("No screenshots for scene %d, skipping", s.ID)
			j.progress.Increment()
			continue
		}

		present, confidence := j.matchPerformer(ctx, client, performer, refImage, screenshots)
		if present && confidence >= minConfidence {
			if err := r.WithTxn(ctx, func(ctx context.Context) error {
				partial := models.NewScenePartial()
				partial.PerformerIDs = &models.UpdateIDs{
					IDs:  []int{performer.ID},
					Mode: models.RelationshipUpdateModeAdd,
				}
				_, err := r.Scene.UpdatePartial(ctx, s.ID, partial)
				return err
			}); err != nil {
				logger.Warnf("Error adding performer %q to scene %d: %v", performer.Name, s.ID, err)
			} else {
				count++
				logger.Infof("Performer %q matched scene %d (confidence %.2f)", performer.Name, s.ID, confidence)
			}
		} else {
			logger.Debugf("Performer %q not matched in scene %d (present=%v, confidence=%.2f)", performer.Name, s.ID, present, confidence)
		}

		j.progress.Increment()
	}

	return count, nil
}

func (j *AIPerformerClusterJob) clusterImages(
	ctx context.Context,
	client *ai.Client,
	r models.Repository,
	performer *models.Performer,
	refImage []byte,
	maxImages int,
	minConfidence float64,
	overwrite bool,
) (int, error) {
	var imageFilter *models.ImageFilterType
	if !overwrite {
		imageFilter = &models.ImageFilterType{
			Performers: &models.MultiCriterionInput{
				Value:    []string{fmt.Sprintf("%d", performer.ID)},
				Modifier: models.CriterionModifierExcludes,
			},
		}
	}

	images, err := image.Query(ctx, r.Image, imageFilter, performerClusterFindFilter(maxImages))
	if err != nil {
		return 0, fmt.Errorf("querying images: %w", err)
	}

	if len(images) == 0 {
		return 0, nil
	}

	logger.Infof("Clustering %d images for performer %q (overwrite=%v)", len(images), performer.Name, overwrite)
	j.progress.SetTotal(len(images))
	count := 0
	for _, img := range images {
		if job.IsCancelled(ctx) {
			return count, nil
		}

		if err := img.LoadPrimaryFile(ctx, r.File); err != nil {
			logger.Debugf("Error loading primary file for image %d, skipping", img.ID)
			j.progress.Increment()
			continue
		}

		f := img.Files.Primary()
		if f == nil {
			logger.Debugf("No primary file for image %d, skipping", img.ID)
			j.progress.Increment()
			continue
		}

		data, err := os.ReadFile(f.Base().Path)
		if err != nil {
			logger.Debugf("Error reading image file for image %d, skipping", img.ID)
			j.progress.Increment()
			continue
		}

		mediaType := http.DetectContentType(data)
		candidate := []ai.MultiImage{{
			Base64:    base64.StdEncoding.EncodeToString(data),
			MediaType: mediaType,
		}}

		present, confidence := j.matchPerformer(ctx, client, performer, refImage, candidate)
		if present && confidence >= minConfidence {
			if err := r.WithTxn(ctx, func(ctx context.Context) error {
				partial := models.NewImagePartial()
				partial.PerformerIDs = &models.UpdateIDs{
					IDs:  []int{performer.ID},
					Mode: models.RelationshipUpdateModeAdd,
				}
				_, err := r.Image.UpdatePartial(ctx, img.ID, partial)
				return err
			}); err != nil {
				logger.Warnf("Error adding performer %q to image %d: %v", performer.Name, img.ID, err)
			} else {
				count++
				logger.Infof("Performer %q matched image %d (confidence %.2f)", performer.Name, img.ID, confidence)
			}
		} else {
			logger.Debugf("Performer %q not matched in image %d (present=%v, confidence=%.2f)", performer.Name, img.ID, present, confidence)
		}

		j.progress.Increment()
	}

	return count, nil
}

// matchPerformer asks the vision model whether the performer in the reference
// photo appears in the provided candidate images.
func (j *AIPerformerClusterJob) matchPerformer(
	ctx context.Context,
	client *ai.Client,
	performer *models.Performer,
	refImage []byte,
	candidateImages []ai.MultiImage,
) (bool, float64) {
	refMediaType := http.DetectContentType(refImage)
	ref := ai.MultiImage{
		Base64:    base64.StdEncoding.EncodeToString(refImage),
		MediaType: refMediaType,
	}

	contentParts := []ai.ContentPart{
		{
			Type: "text",
			Text: fmt.Sprintf("This is the reference photo of the performer named %q.", performer.Name),
		},
		{
			Type:     "image_url",
			ImageURL: &ai.ImageURL{URL: "data:" + ref.MediaType + ";base64," + ref.Base64},
		},
	}

	for _, img := range candidateImages {
		contentParts = append(contentParts, ai.ContentPart{
			Type:     "image_url",
			ImageURL: &ai.ImageURL{URL: "data:" + img.MediaType + ";base64," + img.Base64},
		})
	}

	systemPrompt := "You are an expert at recognizing people across photographs. Answer with only valid JSON, no other text."
	userText := fmt.Sprintf(
		`Compare the person in the reference photo (performer %q) against the person(s) in the subsequent image(s).
Determine whether this specific person appears in any of the subsequent images. Use distinctive features such as facial structure, tattoos, scars, body type, and hairstyle.

Return ONLY valid JSON with this structure:
{"present": true or false, "confidence": 0.0 to 1.0}
- "present" is true only if you are confident the same specific person appears.
- "confidence" is your certainty from 0.0 (no idea) to 1.0 (certain).`,
		performer.Name,
	)
	contentParts = append(contentParts, ai.ContentPart{Type: "text", Text: userText})

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: contentParts},
	}

	req := ai.ChatCompletionRequest{
		Messages: messages,
	}

	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		logger.Warnf("Face match error for %q: %v", performer.Name, err)
		return false, 0
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return false, 0
	}

	cleanJSON := ai.ExtractJSON(content)
	if cleanJSON == "" {
		logger.Warnf("Error parsing face match response for %q: no JSON found", performer.Name)
		return false, 0
	}

	var result aiFaceMatchResult
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		logger.Warnf("Error parsing face match response for %q: %v", performer.Name, err)
		return false, 0
	}

	return result.Present, result.Confidence
}

func (j *AIPerformerClusterJob) sceneScreenshotsForCluster(ctx context.Context, s *models.Scene, r models.Repository) ([]ai.MultiImage, error) {
	if err := s.LoadPrimaryFile(ctx, r.File); err != nil {
		return nil, fmt.Errorf("loading primary file: %w", err)
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
		return 2
	})

	percentages := make([]float64, numShots)
	step := 1.0 / float64(numShots+1)
	for i := range numShots {
		percentages[i] = step * float64(i+1)
	}

	var images []ai.MultiImage
	var tmpFiles []string
	defer func() {
		for _, p := range tmpFiles {
			os.Remove(p)
		}
	}()

	for _, pct := range percentages {
		t := pct * duration
		if t < 0.1 {
			t = 0.1
		}
		if t > duration-0.5 {
			t = duration - 0.5
		}
		if t < 0 {
			continue
		}

		tmpFile, err := os.CreateTemp("", "stash-ai-cluster-*.jpg")
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

		if len(images) == numShots {
			break
		}
	}

	return images, nil
}
