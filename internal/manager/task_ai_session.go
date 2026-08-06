package manager

import (
	"context"
	"fmt"
	"sort"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/models"
)

type AISessionBuildInput struct {
	DurationMinutes int      `json:"duration_minutes"`
	PerformerIDs    []int    `json:"performer_ids"`
	Moods           []string `json:"moods"`
	MinSteam        *int     `json:"min_steam"`
	Vibe            string   `json:"vibe"`
	Limit           *int     `json:"limit"`
}

type AISessionScene struct {
	SceneID    int     `json:"scene_id"`
	Title      string  `json:"title"`
	Duration   float64 `json:"duration"`
	Steam      int     `json:"steam"`
	BestMoment float64 `json:"best_moment"`
}

// AIBuildSession selects scenes for a goon session: candidates are filtered by
// steam score, moods, and performers, optionally boosted by embedding
// similarity to a vibe description, then greedily packed up to the requested
// duration.
func (s *Manager) AIBuildSession(ctx context.Context, input AISessionBuildInput) ([]AISessionScene, float64, error) {
	r := s.Repository

	minSteam := 6
	if input.MinSteam != nil {
		minSteam = *input.MinSteam
	}
	maxScenes := 20
	if input.Limit != nil && *input.Limit > 0 {
		maxScenes = *input.Limit
	}

	filter := &models.SceneFilterType{
		SteamScore: &models.IntCriterionInput{
			Value:    minSteam,
			Modifier: models.CriterionModifierGreaterThan,
		},
	}
	if len(input.Moods) > 0 {
		filter.Moods = &models.MultiCriterionInput{
			Value:    input.Moods,
			Modifier: models.CriterionModifierIncludes,
		}
	}
	if len(input.PerformerIDs) > 0 {
		values := make([]string, len(input.PerformerIDs))
		for i, id := range input.PerformerIDs {
			values[i] = fmt.Sprintf("%d", id)
		}
		filter.Performers = &models.MultiCriterionInput{
			Value:    values,
			Modifier: models.CriterionModifierIncludes,
		}
	}

	// embedding boost when a vibe is given
	vibeBoost := map[int]float64{}
	if input.Vibe != "" {
		model := instance.Config.GetAIEmbeddingModel()
		if model == "" {
			model = instance.Config.GetAIModel()
		}
		if baseURL := instance.Config.GetAIBaseURL(); baseURL != "" && model != "" {
			client := ai.NewClient(baseURL, model)
			if vec, err := client.Embedding(ctx, input.Vibe); err == nil {
				if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
					similar, err := r.Embedding.SearchSimilar(ctx, "scene", model, vec, 200)
					if err != nil {
						return err
					}
					for _, sim := range similar {
						if sim.Score > 0.2 {
							vibeBoost[sim.EntityID] = sim.Score
						}
					}
					return nil
				}); err != nil {
					return nil, 0, fmt.Errorf("vibe search: %w", err)
				}
			}
		}
	}

	perPage := -1
	var candidates []*models.Scene
	var steamScores map[int]int
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		result, err := r.Scene.Query(ctx, models.SceneQueryOptions{
			SceneFilter: filter,
			QueryOptions: models.QueryOptions{
				FindFilter: &models.FindFilterType{PerPage: &perPage},
			},
		})
		if err != nil {
			return err
		}
		candidates, err = result.Resolve(ctx)
		if err != nil {
			return err
		}
		ids := make([]int, len(candidates))
		for i, c := range candidates {
			ids[i] = c.ID
		}
		steamScores, err = r.Scene.GetSteamScores(ctx, ids)
		return err
	}); err != nil {
		return nil, 0, fmt.Errorf("querying scenes: %w", err)
	}

	type candidate struct {
		scene *models.Scene
		score float64
	}
	var scored []candidate
	for _, c := range candidates {
		score := float64(steamScores[c.ID]) / 10.0
		if boost, ok := vibeBoost[c.ID]; ok {
			score += boost * 0.6
		}
		scored = append(scored, candidate{scene: c, score: score})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })

	// pack greedily up to the duration budget
	budget := float64(input.DurationMinutes) * 60
	var plan []AISessionScene
	total := 0.0
	for _, c := range scored {
		if len(plan) >= maxScenes {
			break
		}
		if total >= budget {
			break
		}

		sc := c.scene
		duration := 0.0
		if err := sc.LoadPrimaryFile(ctx, r.File); err == nil {
			if f := sc.Files.Primary(); f != nil {
				duration = f.Duration
			}
		}
		// cap each scene's contribution
		contrib := duration
		if contrib > 120 {
			contrib = 120
		}
		if contrib <= 0 {
			contrib = 60
		}
		if total+contrib > budget && total > 0 {
			continue
		}

		bestMoment := 0.0
		perScene := 100
		_ = r.WithReadTxn(ctx, func(ctx context.Context) error {
			markers, _, err := r.SceneMarker.Query(ctx, &models.SceneMarkerFilterType{
				Scenes: &models.MultiCriterionInput{
					Value:    []string{fmt.Sprintf("%d", sc.ID)},
					Modifier: models.CriterionModifierIncludes,
				},
			}, &models.FindFilterType{PerPage: &perScene})
			if err != nil {
				return err
			}
			var bestIntensity float64
			for _, m := range markers {
				if m.Intensity != nil && *m.Intensity > bestIntensity {
					bestIntensity = *m.Intensity
					bestMoment = m.Seconds
				}
			}
			return nil
		})

		plan = append(plan, AISessionScene{
			SceneID:    sc.ID,
			Title:      sc.Title,
			Duration:   duration,
			Steam:      steamScores[sc.ID],
			BestMoment: bestMoment,
		})
		total += contrib
	}

	if len(plan) == 0 {
		return nil, 0, fmt.Errorf("no scenes match the session criteria")
	}

	return plan, total / 60, nil
}
