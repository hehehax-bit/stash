package manager

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type AIPerformerMergeSuggestInput struct {
	MaxPerformers *int     `json:"maxPerformers"`
	MinConfidence *float64 `json:"minConfidence"`
	Timeout       *int     `json:"timeout"`
}

type AIPerformerMergeSuggestJob struct {
	input    AIPerformerMergeSuggestInput
	progress *job.Progress
}

func CreateAIPerformerMergeSuggestJob(input AIPerformerMergeSuggestInput) *AIPerformerMergeSuggestJob {
	return &AIPerformerMergeSuggestJob{
		input: input,
	}
}

func (j *AIPerformerMergeSuggestJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	baseURL := instance.Config.GetAIBaseURL()
	model := instance.Config.GetAIImageEmbeddingModel()
	if model == "" {
		model = instance.Config.GetAIEmbeddingModel()
	}
	if model == "" {
		model = instance.Config.GetAIModel()
	}
	if model == "" {
		return fmt.Errorf("no embedding model configured")
	}

	client := ai.NewEmbeddingClient(baseURL, model, instance.Config.GetAIEndpoint())
	if j.input.Timeout != nil && *j.input.Timeout > 0 {
		client.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	minConfidence := 0.85
	if j.input.MinConfidence != nil && *j.input.MinConfidence > 0 {
		minConfidence = *j.input.MinConfidence
	}

	maxPerformers := 0
	if j.input.MaxPerformers != nil {
		maxPerformers = *j.input.MaxPerformers
	}

	r := instance.Repository

	var performers []*models.Performer
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		performers, _, err = r.Performer.Query(ctx, nil, &models.FindFilterType{PerPage: intPtr(999999)})
		return err
	}); err != nil {
		return fmt.Errorf("querying performers: %w", err)
	}

	if maxPerformers > 0 && len(performers) > maxPerformers {
		performers = performers[:maxPerformers]
	}

	j.progress.SetTotal(len(performers))

	type embedding struct {
		id   int
		vec  []float32
		name string
	}

	var embeddings []embedding
	for i, p := range performers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if job.IsCancelled(ctx) {
			return nil
		}

		var imageData []byte
		if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
			var err error
			imageData, err = r.Performer.GetImage(ctx, p.ID)
			return err
		}); err != nil {
			logger.Warnf("error loading image for performer %d: %v", p.ID, err)
			j.progress.Increment()
			continue
		}
		if len(imageData) == 0 {
			j.progress.Increment()
			continue
		}

		vec, err := client.EmbeddingImage(ctx, base64.StdEncoding.EncodeToString(imageData), "image/jpeg")
		if err != nil {
			logger.Warnf("error embedding performer %d: %v", p.ID, err)
			j.progress.Increment()
			continue
		}

		embeddings = append(embeddings, embedding{id: p.ID, vec: vec, name: p.Name})
		j.progress.SetProcessed(i + 1)
		j.progress.Increment()
	}

	if len(embeddings) < 2 {
		logger.Info("Performer merge suggestions: fewer than 2 performers with images, skipping")
		return nil
	}

	const maxSuggestions = 200
	suggested := 0

	for i := 0; i < len(embeddings); i++ {
		for k := i + 1; k < len(embeddings); k++ {
			if job.IsCancelled(ctx) {
				return nil
			}
			if suggested >= maxSuggestions {
				return nil
			}

			a := embeddings[i]
			b := embeddings[k]
			score := cosineSimilarity(a.vec, b.vec)
			if score < minConfidence {
				continue
			}

			sourceID, targetID := a.id, b.id
			if sourceID > targetID {
				sourceID, targetID = targetID, sourceID
			}

			var existing *models.AIPerformerSuggestion
			if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
				var err error
				existing, err = r.AIPerformerSuggestion.FindPair(ctx, sourceID, targetID)
				return err
			}); err != nil {
				logger.Warnf("error checking suggestion for performers %d/%d: %v", sourceID, targetID, err)
				continue
			}
			if existing != nil {
				continue
			}

			if err := r.WithTxn(ctx, func(ctx context.Context) error {
				return r.AIPerformerSuggestion.Create(ctx, &models.AIPerformerSuggestion{
					SourcePerformerID: sourceID,
					TargetPerformerID: targetID,
					Confidence:        score,
				})
			}); err != nil {
				logger.Warnf("error creating suggestion for performers %d/%d: %v", sourceID, targetID, err)
				continue
			}

			suggested++
			logger.Infof("Performer merge suggestion: %q (%d) and %q (%d), confidence %.3f",
				a.name, a.id, b.name, b.id, score)
		}
	}

	logger.Infof("Performer merge suggestions complete: %d new suggestions", suggested)
	return nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
