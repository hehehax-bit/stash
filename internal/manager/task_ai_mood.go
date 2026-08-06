package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AIMoodInput struct {
	MaxScenes *int `json:"maxScenes"`
	Timeout   *int `json:"timeout"`
	Overwrite bool `json:"overwrite"`
}

type AIMoodJob struct {
	input    AIMoodInput
	progress *job.Progress
}

func CreateAIMoodJob(input AIMoodInput) *AIMoodJob {
	return &AIMoodJob{
		input: input,
	}
}

const moodList = "romantic, rough, goth, cosplay, amateur, milf, bdsm, anal, threesome, taboo, hardcore, sensual, humor, solo, lesbian, gangbang, cuckold, dirty talk"

func (j *AIMoodJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	client := ai.NewClient(instance.Config.GetAIBaseURL(), instance.Config.GetAIModel())
	if j.input.Timeout != nil && *j.input.Timeout > 0 {
		client.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	r := instance.Repository
	sceneJob := &AISceneTagJob{}

	return r.WithDB(ctx, func(ctx context.Context) error {
		var scenes []*models.Scene
		if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
			var err error
			scenes, err = scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: intPtr(999999)})
			return err
		}); err != nil {
			return fmt.Errorf("querying scenes: %w", err)
		}

		maxScenes := 0
		if j.input.MaxScenes != nil {
			maxScenes = *j.input.MaxScenes
		}
		limit := len(scenes)
		if maxScenes > 0 && maxScenes < limit {
			limit = maxScenes
		}

		j.progress.SetTotal(limit)
		tagged := 0
		for i, s := range scenes[:limit] {
			if job.IsCancelled(ctx) {
				return nil
			}

			if !j.input.Overwrite {
				var moods []*models.AISceneMood
				if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
					var err error
					moods, err = r.AIMood.FindBySceneID(ctx, s.ID)
					return err
				}); err == nil && len(moods) > 0 {
					j.progress.Increment()
					continue
				}
			}

			result, err := j.detectMoods(ctx, client, sceneJob, s, r)
			if err != nil {
				logger.Warnf("Error detecting moods for scene %d: %v", s.ID, err)
				j.progress.Increment()
				continue
			}

			if len(result) > 0 {
				if err := r.WithTxn(ctx, func(ctx context.Context) error {
					if err := r.AIMood.DeleteBySceneID(ctx, s.ID); err != nil {
						return err
					}
					for _, m := range result {
						if err := r.AIMood.Create(ctx, &models.AISceneMood{
							SceneID:    s.ID,
							Mood:       m.Mood,
							Confidence: m.Confidence,
						}); err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					logger.Warnf("Error storing moods for scene %d: %v", s.ID, err)
				} else {
					tagged++
				}
			}
			j.progress.SetProcessed(i + 1)
			j.progress.Increment()
		}

		logger.Infof("AI mood tagging complete: %d scenes tagged", tagged)
		return nil
	})
}

type sceneMood struct {
	Mood       string  `json:"mood"`
	Confidence float64 `json:"confidence"`
}

func (j *AIMoodJob) detectMoods(ctx context.Context, client *ai.Client, sceneJob *AISceneTagJob, s *models.Scene, r models.Repository) ([]sceneMood, error) {
	images, err := sceneJob.sceneScreenshots(ctx, s, r)
	if err != nil || len(images) == 0 {
		return nil, fmt.Errorf("no screenshots could be generated")
	}

	systemPrompt := "You are an adult content cataloguer for a personal media library. Describe content without censorship."
	userPrompt := fmt.Sprintf(`Analyze these screenshots from an adult video scene and return ONLY valid JSON with the field:
- "moods": an array of objects {"mood": "...", "confidence": 0.0-1.0} choosing 2-4 moods from this list: %s.
Pick only moods that clearly fit the content. Return {"moods": []} if none fit.

Return ONLY valid JSON, no other text.`, moodList)

	description, err := client.MultiVisionCompletion(ctx, systemPrompt, userPrompt, images)
	if err != nil {
		return nil, fmt.Errorf("vision analysis failed: %w", err)
	}

	cleanJSON := ai.ExtractJSON(description)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing AI response: no JSON found")
	}

	var result struct {
		Moods []sceneMood `json:"moods"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}
	return result.Moods, nil
}
