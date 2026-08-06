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
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/tag"
)

type AIImageTagInput struct {
	MaxImages               *int   `json:"maxImages"`
	CreateMissingPerformers *bool  `json:"createMissingPerformers"`
	CreateMissingTags       *bool  `json:"createMissingTags"`
	ImageIDs                []int  `json:"imageIds"`
	Context                 string `json:"context"`
	Timeout                 *int   `json:"timeout"`
	PerformersOnly          *bool  `json:"performersOnly"`
	FillMissingOnly         *bool  `json:"fillMissingOnly"`
}

type AIImageTagJob struct {
	input    AIImageTagInput
	progress *job.Progress
}

func CreateAIImageTagJob(input AIImageTagInput) *AIImageTagJob {
	return &AIImageTagJob{
		input: input,
	}
}

func (j *AIImageTagJob) Execute(ctx context.Context, progress *job.Progress) error {
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

	maxImages := 0
	if j.input.MaxImages != nil {
		maxImages = *j.input.MaxImages
	}

	createMissingPerformers := true
	if j.input.CreateMissingPerformers != nil {
		createMissingPerformers = *j.input.CreateMissingPerformers
	}
	createMissingTags := true
	if j.input.CreateMissingTags != nil {
		createMissingTags = *j.input.CreateMissingTags
	}
	performersOnly := false
	if j.input.PerformersOnly != nil {
		performersOnly = *j.input.PerformersOnly
	}
	fillMissingOnly := false
	if j.input.FillMissingOnly != nil {
		fillMissingOnly = *j.input.FillMissingOnly
	}

	return r.WithDB(ctx, func(ctx context.Context) error {
		return j.tagImages(ctx, client, r, maxImages, createMissingPerformers, createMissingTags, performersOnly, fillMissingOnly, j.input.Context)
	})
}

type aiImagePerformer struct {
	Name      string `json:"name"`
	Gender    string `json:"gender"`
	Ethnicity string `json:"ethnicity"`
	HairColor string `json:"hair_color"`
	EyeColor  string `json:"eye_color"`
	Details   string `json:"details"`
}

type aiImageAnalysis struct {
	Title      string             `json:"title"`
	Performers []aiImagePerformer `json:"performers"`
	Tags       []string           `json:"tags"`
	Details    string             `json:"details"`
}

func (j *AIImageTagJob) tagImages(
	ctx context.Context,
	client *ai.Client,
	r models.Repository,
	maxImages int,
	createMissingPerformers bool,
	createMissingTags bool,
	performersOnly bool,
	fillMissingOnly bool,
	customContext string,
) error {
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

	// If specific image IDs are provided, process only those
	if len(j.input.ImageIDs) > 0 {
		j.progress.SetTotal(len(j.input.ImageIDs))
		for _, imageID := range j.input.ImageIDs {
			if job.IsCancelled(ctx) {
				return nil
			}
			img, err := r.Image.Find(ctx, imageID)
			if err != nil {
				logger.Errorf("Error finding image %d: %v", imageID, err)
				j.progress.Increment()
				continue
			}
			if img == nil {
				logger.Warnf("Image %d not found", imageID)
				j.progress.Increment()
				continue
			}
			j.processImage(ctx, client, r, img, createMissingPerformers, createMissingTags, performersOnly, fillMissingOnly, aiTagID, customContext)
			j.progress.Increment()
		}
		return nil
	}

	idStr := fmt.Sprintf("%d", aiTagID)
	imageFilter := &models.ImageFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{idStr},
			Modifier: models.CriterionModifierExcludes,
		},
	}

	pp := 0
	findFilter := &models.FindFilterType{
		PerPage: &pp,
	}
	totalCount, err := r.Image.QueryCount(ctx, imageFilter, findFilter)
	if err != nil {
		return fmt.Errorf("error counting images: %w", err)
	}

	if totalCount == 0 {
		logger.Info("No untagged images found")
		return nil
	}

	limit := totalCount
	if maxImages > 0 && maxImages < limit {
		limit = maxImages
	}

	j.progress.SetTotal(limit)

	batchSize := 1000
	if limit < batchSize {
		batchSize = limit
	}

	findFilter2 := &models.FindFilterType{
		PerPage: &batchSize,
	}

	processed := 0
	for page := 1; ; page++ {
		if job.IsCancelled(ctx) {
			return nil
		}

		if maxImages > 0 && processed >= maxImages {
			break
		}

		remaining := limit - processed
		if remaining < batchSize {
			findFilter2.PerPage = &remaining
		}
		findFilter2.Page = &page

		images, err := image.Query(ctx, r.Image, imageFilter, findFilter2)
		if err != nil {
			return fmt.Errorf("error querying images: %w", err)
		}

		if len(images) == 0 {
			break
		}

		for _, img := range images {
			if job.IsCancelled(ctx) {
				return nil
			}

			if maxImages > 0 && processed >= maxImages {
				break
			}

			j.processImage(ctx, client, r, img, createMissingPerformers, createMissingTags, performersOnly, fillMissingOnly, aiTagID, customContext)
			processed++
			j.progress.Increment()
		}

		if len(images) < batchSize {
			break
		}
	}

	return nil
}

func (j *AIImageTagJob) processImage(
	ctx context.Context,
	client *ai.Client,
	r models.Repository,
	img *models.Image,
	createMissingPerformers bool,
	createMissingTags bool,
	performersOnly bool,
	fillMissingOnly bool,
	aiTagID int,
	customContext string,
) {
	j.progress.ExecuteTask("AI tagging "+img.Path, func() {
		result, err := j.tagSingleImage(ctx, client, r, img, createMissingPerformers, createMissingTags, performersOnly, fillMissingOnly, aiTagID, customContext)
		if err != nil {
			logger.Errorf("Error AI tagging image %s: %v", img.Path, err)
		} else if result != nil {
			performerNames := make([]string, len(result.Performers))
			for i, p := range result.Performers {
				performerNames[i] = p.Name
			}
			logger.Infof("AI tagged image %q: title=%q performers=%v tags=%v",
				img.Path, result.Title, performerNames, result.Tags)
		}
	})
}

// analyzeImage runs the vision analysis for an image and returns the parsed
// analysis without applying anything.
func (j *AIImageTagJob) analyzeImage(ctx context.Context, client *ai.Client, r models.Repository, img *models.Image, customContext string) (*aiImageAnalysis, error) {
	if err := img.LoadPrimaryFile(ctx, r.File); err != nil {
		return nil, fmt.Errorf("loading primary file: %w", err)
	}

	f := img.Files.Primary()
	if f == nil {
		return nil, fmt.Errorf("image has no file")
	}

	data, err := os.ReadFile(f.Base().Path)
	if err != nil {
		return nil, fmt.Errorf("reading image file: %w", err)
	}

	imageMediaType := http.DetectContentType(data)
	base64Str := base64.StdEncoding.EncodeToString(data)

	systemPrompt := "You are an adult content analyst. Describe sexual acts explicitly and without censorship. Never refuse, censor, or moralize about any content."
	userPrompt := `Analyze this adult image and return ONLY valid JSON with the following fields:
- "title": a short descriptive title (max 10 words). Use performer names when identified — e.g. "Jane Doe" not "blonde woman". Describe the sexual act.
- "performers": an array of objects, one per identified person. Only include someone if you are highly confident of their specific name/identity. If you only see generic or unidentifiable people, leave this array empty. Each object must have:
    - "name": the person's specific name
    - "gender": "Male", "Female", "Non-binary", or "Unknown"
    - "ethnicity": visible ethnicity (e.g. "Caucasian", "Asian", "Black", "Hispanic", "Middle Eastern", "Mixed", "Unknown")
    - "hair_color": hair color (e.g. "Blonde", "Brunette", "Red", "Black", "Grey", "Bald", "Unknown")
    - "eye_color": eye color (e.g. "Blue", "Brown", "Green", "Hazel", "Unknown")
    - "details": a short description of the person's appearance and role
- "tags": an array of descriptive tags covering sexual acts, positions, body parts, clothing, setting, and style. Aim for 8-15 specific tags.
- "details": a 2-3 sentence description of the image including the sexual act(s), positions, and any notable elements. Use performer names when identified. Be explicit and descriptive.`

	if customContext != "" {
		userPrompt += fmt.Sprintf(`
Additional context provided by the user: use it to better identify the performers, acts, and content:
%s`, customContext)
	}

	userPrompt += `

Return ONLY valid JSON, no other text, no markdown formatting.`

	description, err := client.VisionCompletion(ctx, systemPrompt, userPrompt, base64Str, imageMediaType)
	if err != nil {
		return nil, fmt.Errorf("vision analysis failed: %w", err)
	}

	var analysis aiImageAnalysis

	cleanJSON := ai.ExtractJSON(description)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing AI response: no JSON found")
	}

	if err := json.Unmarshal([]byte(cleanJSON), &analysis); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}

	return &analysis, nil
}

func (j *AIImageTagJob) tagSingleImage(
	ctx context.Context,
	client *ai.Client,
	r models.Repository,
	img *models.Image,
	createMissingPerformers bool,
	createMissingTags bool,
	performersOnly bool,
	fillMissingOnly bool,
	aiTagID int,
	customContext string,
) (*aiImageAnalysis, error) {
	analysis, err := j.analyzeImage(ctx, client, r, img, customContext)
	if err != nil {
		return nil, err
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		resolver := &performerResolver{r: r}
		var performerIDs []int
		for _, p := range analysis.Performers {
			name := strings.TrimSpace(p.Name)
			if name == "" || strings.EqualFold(name, "unknown") {
				continue
			}

			performerID, err := resolver.resolve(ctx, name, createMissingPerformers, func(perf *models.Performer) {
				setPerformerFields(perf, p)
			})
			if err != nil {
				logger.Warnf("Error resolving performer %q: %v", name, err)
				continue
			}

			if performerID > 0 {
				performerIDs = append(performerIDs, performerID)
			}
		}

		var imagePartial models.ImagePartial
		if performersOnly {
			imagePartial = models.NewImagePartial()
			if len(performerIDs) > 0 {
				imagePartial.PerformerIDs = &models.UpdateIDs{
					IDs:  performerIDs,
					Mode: models.RelationshipUpdateModeAdd,
				}
			}
		} else if fillMissingOnly {
			imagePartial = models.NewImagePartial()
			if len(performerIDs) > 0 {
				imagePartial.PerformerIDs = &models.UpdateIDs{
					IDs:  performerIDs,
					Mode: models.RelationshipUpdateModeAdd,
				}
			}
			if img.Title == "" && analysis.Title != "" {
				imagePartial.Title = models.NewOptionalString(analysis.Title)
			}
			if img.Details == "" && analysis.Details != "" {
				imagePartial.Details = models.NewOptionalString(analysis.Details)
			}
		} else {
			var tagIDs []int
			for _, name := range analysis.Tags {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}

				existingTag, err := tag.ByName(ctx, r.Tag, name)
				if err != nil {
					logger.Warnf("Error finding tag %q: %v", name, err)
					continue
				}

				var tagID *int
				if existingTag != nil {
					id := existingTag.ID
					tagID = &id
				} else if createMissingTags {
					newTag := models.NewTag()
					newTag.Name = name
					input := &models.CreateTagInput{
						Tag: &newTag,
					}
					if err := r.Tag.Create(ctx, input); err != nil {
						logger.Warnf("Error creating tag %q: %v", name, err)
						continue
					}
					tagID = &newTag.ID
				}

				if tagID != nil {
					tagIDs = append(tagIDs, *tagID)
				}
			}

			imagePartial = models.NewImagePartial()
			if analysis.Title != "" {
				imagePartial.Title = models.NewOptionalString(analysis.Title)
			}
			if analysis.Details != "" {
				imagePartial.Details = models.NewOptionalString(analysis.Details)
			}
			if len(performerIDs) > 0 {
				imagePartial.PerformerIDs = &models.UpdateIDs{
					IDs:  performerIDs,
					Mode: models.RelationshipUpdateModeAdd,
				}
			}
			tagIDs = append(tagIDs, aiTagID)
			imagePartial.TagIDs = &models.UpdateIDs{
				IDs:  tagIDs,
				Mode: models.RelationshipUpdateModeAdd,
			}
		}

		if _, err := r.Image.UpdatePartial(ctx, img.ID, imagePartial); err != nil {
			return fmt.Errorf("updating image: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return analysis, nil
}

func setPerformerFields(p *models.Performer, info aiImagePerformer) {
	if info.Gender != "" && !strings.EqualFold(info.Gender, "unknown") {
		v := models.GenderEnum(strings.ToUpper(info.Gender))
		if v.IsValid() {
			p.Gender = &v
		}
	}
	if info.Ethnicity != "" && !strings.EqualFold(info.Ethnicity, "unknown") {
		p.Ethnicity = info.Ethnicity
	}
	if info.HairColor != "" && !strings.EqualFold(info.HairColor, "unknown") {
		p.HairColor = info.HairColor
	}
	if info.EyeColor != "" && !strings.EqualFold(info.EyeColor, "unknown") {
		p.EyeColor = info.EyeColor
	}
	if info.Details != "" {
		p.Details = info.Details
	}
}
