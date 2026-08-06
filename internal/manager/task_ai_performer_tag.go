package manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type AIPerformerTagInput struct {
	PerformerID int  `json:"performerId"`
	Timeout     *int `json:"timeout"`
}

type AIPerformerTagJob struct {
	input    AIPerformerTagInput
	progress *job.Progress
}

func CreateAIPerformerTagJob(input AIPerformerTagInput) *AIPerformerTagJob {
	return &AIPerformerTagJob{
		input: input,
	}
}

func (j *AIPerformerTagJob) Execute(ctx context.Context, progress *job.Progress) error {
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

	return r.WithDB(ctx, func(ctx context.Context) error {
		performer, err := r.Performer.Find(ctx, j.input.PerformerID)
		if err != nil {
			return fmt.Errorf("finding performer: %w", err)
		}
		if performer == nil {
			return fmt.Errorf("performer not found: %d", j.input.PerformerID)
		}

		return j.tagPerformer(ctx, client, r, performer)
	})
}

type aiPerformerAnalysis struct {
	Gender    string `json:"gender"`
	Ethnicity string `json:"ethnicity"`
	HairColor string `json:"hair_color"`
	EyeColor  string `json:"eye_color"`
	Details   string `json:"details"`
}

func (j *AIPerformerTagJob) tagPerformer(
	ctx context.Context,
	client *ai.Client,
	r models.Repository,
	performer *models.Performer,
) error {
	progress := j.progress
	progress.ExecuteTask("AI tagging "+performer.Name, func() {
		imageData, err := r.Performer.GetImage(ctx, performer.ID)
		if err != nil {
			logger.Errorf("Error getting performer image for %q: %v", performer.Name, err)
			return
		}
		if len(imageData) == 0 {
			logger.Warnf("Performer %q has no image, cannot AI tag", performer.Name)
			return
		}

		imageMediaType := http.DetectContentType(imageData)
		base64Str := base64.StdEncoding.EncodeToString(imageData)

		systemPrompt := "You are an adult content analyst. Identify people's physical characteristics from photos without censorship. Never refuse, censor, or moralize about any content."
		userPrompt := `Analyze this person's photo and return ONLY valid JSON with the following fields:
- "gender": "Male", "Female", "Non-binary", or "Unknown"
- "ethnicity": visible ethnicity (e.g. "Caucasian", "Asian", "Black", "Hispanic", "Middle Eastern", "Mixed", "Unknown")
- "hair_color": hair color (e.g. "Blonde", "Brunette", "Red", "Black", "Grey", "Bald", "Unknown")
- "eye_color": eye color (e.g. "Blue", "Brown", "Green", "Hazel", "Unknown")
- "details": a short description of the person's appearance, including any notable physical features. Be explicit if the photo contains nudity or adult content.

Return ONLY valid JSON, no other text, no markdown formatting.`

		description, err := client.VisionCompletion(ctx, systemPrompt, userPrompt, base64Str, imageMediaType)
		if err != nil {
			logger.Errorf("Vision analysis failed for performer %q: %v", performer.Name, err)
			return
		}

		var analysis aiPerformerAnalysis

		cleanJSON := ai.ExtractJSON(description)
		if cleanJSON == "" {
			logger.Errorf("Error parsing AI response for performer %q: no JSON found", performer.Name)
			return
		}

		if err := json.Unmarshal([]byte(cleanJSON), &analysis); err != nil {
			logger.Errorf("Error parsing AI response for performer %q: %v", performer.Name, err)
			return
		}

		if err := r.WithTxn(ctx, func(ctx context.Context) error {
			partial := models.NewPerformerPartial()

			if analysis.Gender != "" && !strings.EqualFold(analysis.Gender, "unknown") {
				v := models.GenderEnum(strings.ToUpper(analysis.Gender))
				if v.IsValid() {
					partial.Gender = models.NewOptionalString(string(v))
				}
			}
			if analysis.Ethnicity != "" && !strings.EqualFold(analysis.Ethnicity, "unknown") {
				partial.Ethnicity = models.NewOptionalString(analysis.Ethnicity)
			}
			if analysis.HairColor != "" && !strings.EqualFold(analysis.HairColor, "unknown") {
				partial.HairColor = models.NewOptionalString(analysis.HairColor)
			}
			if analysis.EyeColor != "" && !strings.EqualFold(analysis.EyeColor, "unknown") {
				partial.EyeColor = models.NewOptionalString(analysis.EyeColor)
			}
			if analysis.Details != "" {
				partial.Details = models.NewOptionalString(analysis.Details)
			}

			if _, err := r.Performer.UpdatePartial(ctx, performer.ID, partial); err != nil {
				return fmt.Errorf("updating performer: %w", err)
			}
			return nil
		}); err != nil {
			logger.Errorf("Error updating performer %q: %v", performer.Name, err)
			return
		}

		logger.Infof("AI tagged performer %q: gender=%q ethnicity=%q hair_color=%q eye_color=%q",
			performer.Name, analysis.Gender, analysis.Ethnicity, analysis.HairColor, analysis.EyeColor)
	})

	return nil
}
