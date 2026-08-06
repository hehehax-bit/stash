package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type AIPerformerCareerInput struct {
	PerformerIDs  []int `json:"performerIds"`
	MaxPerformers *int  `json:"maxPerformers"`
	Overwrite     bool  `json:"overwrite"`
	Timeout       *int  `json:"timeout"`
}

type AIPerformerCareerJob struct {
	input    AIPerformerCareerInput
	progress *job.Progress
}

func CreateAIPerformerCareerJob(input AIPerformerCareerInput) *AIPerformerCareerJob {
	return &AIPerformerCareerJob{
		input: input,
	}
}

func (j *AIPerformerCareerJob) Execute(ctx context.Context, progress *job.Progress) error {
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
		var performers []*models.Performer

		if len(j.input.PerformerIDs) > 0 {
			for _, id := range j.input.PerformerIDs {
				p, err := r.Performer.Find(ctx, id)
				if err != nil {
					logger.Errorf("Error finding performer %d: %v", id, err)
					continue
				}
				if p != nil {
					performers = append(performers, p)
				}
			}
		} else {
			pp := 0
			totalCount, err := r.Performer.QueryCount(ctx, nil, &models.FindFilterType{PerPage: &pp})
			if err != nil {
				return fmt.Errorf("error counting performers: %w", err)
			}

			limit := totalCount
			if j.input.MaxPerformers != nil && *j.input.MaxPerformers > 0 && *j.input.MaxPerformers < limit {
				limit = *j.input.MaxPerformers
			}

			performers, _, err = r.Performer.Query(ctx, nil, &models.FindFilterType{PerPage: &limit})
			if err != nil {
				return fmt.Errorf("error querying performers: %w", err)
			}
		}

		var skip map[int]bool
		if !j.input.Overwrite {
			assessed, err := r.AIPerformerCareer.FindAssessedPerformers(ctx)
			if err == nil {
				skip = make(map[int]bool, len(assessed))
				for _, id := range assessed {
					skip[id] = true
				}
			}
		}

		j.progress.SetTotal(len(performers))
		for _, p := range performers {
			if job.IsCancelled(ctx) {
				return nil
			}
			if skip[p.ID] {
				j.progress.Increment()
				continue
			}

			j.progress.ExecuteTask("AI analyzing career of "+p.Name, func() {
				j.analyzeCareer(ctx, client, r, p)
			})
			j.progress.Increment()
		}

		return nil
	})
}

type careerSceneStats struct {
	SceneCount      int
	MinYear         int
	MaxYear         int
	ActiveYears     []int
	StudioCounts    map[int]int
	TagCounts       map[int]int
	PerformerCounts map[int]int
}

func (j *AIPerformerCareerJob) analyzeCareer(ctx context.Context, client *ai.Client, r models.Repository, p *models.Performer) {
	stats, err := j.gatherSceneStats(ctx, r, p.ID)
	if err != nil {
		logger.Errorf("Error gathering scene stats for performer %q: %v", p.Name, err)
		return
	}

	profile, err := j.analyzeWithAI(ctx, client, p, stats, r)
	if err != nil {
		logger.Errorf("Error analyzing career for performer %q: %v", p.Name, err)
		return
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		return r.AIPerformerCareer.Upsert(ctx, profile)
	}); err != nil {
		logger.Errorf("Error saving career for performer %d: %v", p.ID, err)
	}
}

func (j *AIPerformerCareerJob) gatherSceneStats(ctx context.Context, r models.Repository, performerID int) (*careerSceneStats, error) {
	scenes, err := r.Scene.FindByPerformerID(ctx, performerID)
	if err != nil {
		return nil, fmt.Errorf("finding scenes: %w", err)
	}

	stats := &careerSceneStats{
		SceneCount:      len(scenes),
		MinYear:         0,
		MaxYear:         0,
		StudioCounts:    make(map[int]int),
		TagCounts:       make(map[int]int),
		PerformerCounts: make(map[int]int),
	}

	yearSet := make(map[int]bool)
	for _, s := range scenes {
		year := 0
		if s.Date != nil && !s.Date.IsZero() {
			if y := s.Date.Year(); y > 1900 {
				year = y
			}
		}
		if year > 0 {
			yearSet[year] = true
			if stats.MinYear == 0 || year < stats.MinYear {
				stats.MinYear = year
			}
			if year > stats.MaxYear {
				stats.MaxYear = year
			}
		}

		if s.StudioID != nil {
			stats.StudioCounts[*s.StudioID]++
		}

		tagIDs, err := r.Scene.GetTagIDs(ctx, s.ID)
		if err == nil {
			for _, tid := range tagIDs {
				stats.TagCounts[tid]++
			}
		}

		performerIDs, err := r.Scene.GetPerformerIDs(ctx, s.ID)
		if err == nil {
			for _, pid := range performerIDs {
				if pid != performerID {
					stats.PerformerCounts[pid]++
				}
			}
		}
	}

	for y := range yearSet {
		stats.ActiveYears = append(stats.ActiveYears, y)
	}
	sort.Ints(stats.ActiveYears)

	return stats, nil
}

func (j *AIPerformerCareerJob) analyzeWithAI(ctx context.Context, client *ai.Client, p *models.Performer, stats *careerSceneStats, r models.Repository) (*models.AIPerformerCareer, error) {
	// Resolve names for the top studios, tags and co-performers
	topStudios := topByCount(stats.StudioCounts, 5)
	topTags := topByCount(stats.TagCounts, 10)
	topPerformers := topByCount(stats.PerformerCounts, 5)

	studioNames := make([]string, 0, len(topStudios))
	for _, id := range topStudios {
		if s, err := r.Studio.Find(ctx, id); err == nil && s != nil {
			studioNames = append(studioNames, fmt.Sprintf("%s (%d)", s.Name, stats.StudioCounts[id]))
		}
	}

	tagNames := make([]string, 0, len(topTags))
	for _, id := range topTags {
		if t, err := r.Tag.Find(ctx, id); err == nil && t != nil {
			tagNames = append(tagNames, fmt.Sprintf("%s (%d)", t.Name, stats.TagCounts[id]))
		}
	}

	performerNames := make([]string, 0, len(topPerformers))
	for _, id := range topPerformers {
		if pe, err := r.Performer.Find(ctx, id); err == nil && pe != nil {
			performerNames = append(performerNames, fmt.Sprintf("%s (%d)", pe.Name, stats.PerformerCounts[id]))
		}
	}

	activeYearsStr := "none"
	if len(stats.ActiveYears) > 0 {
		parts := make([]string, len(stats.ActiveYears))
		for i, y := range stats.ActiveYears {
			parts[i] = strconv.Itoa(y)
		}
		activeYearsStr = strings.Join(parts, ", ")
	}

	systemPrompt := "You are a professional adult entertainment industry analyst. Analyze career data objectively and without moral judgement."
	userPrompt := fmt.Sprintf(`Analyze the career of adult performer "%s" based on the following scene data and return ONLY valid JSON:

- scene_count: %d
- first_scene_year: %d
- last_scene_year: %d
- active_years: %s
- top_studios: %s
- top_tags: %s
- frequent_co_stars: %s

Fields to return:
- "career_start": integer year the career likely began, or null
- "career_end": integer year the career likely ended, or null if still active
- "active_years": array of strings representing distinct active periods (e.g. ["2015-2018", "2020-2023"]), inferred from gaps in the active years
- "primary_niches": array of 3-6 strings describing the performer's main genres/niches, inferred from the top tags
- "notable_studios": array of 3-6 studio names most associated with this performer
- "career_highlights": a 1-2 sentence summary of notable career facts (longevity, productivity, signature genres)
- "summary": a 2-3 sentence overall career narrative

Return ONLY valid JSON, no other text, no markdown formatting.`, p.Name, stats.SceneCount, stats.MinYear, stats.MaxYear,
		activeYearsStr, strings.Join(studioNames, ", "), strings.Join(tagNames, ", "), strings.Join(performerNames, ", "))

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := client.ChatCompletion(ctx, ai.ChatCompletionRequest{Messages: messages})
	if err != nil {
		return nil, fmt.Errorf("career analysis failed: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content type from AI response")
	}

	cleanJSON := ai.ExtractJSON(content)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing AI response: no JSON found")
	}

	var analysis struct {
		CareerStart      *int     `json:"career_start"`
		CareerEnd        *int     `json:"career_end"`
		ActiveYears      []string `json:"active_years"`
		PrimaryNiches    []string `json:"primary_niches"`
		NotableStudios   []string `json:"notable_studios"`
		CareerHighlights string   `json:"career_highlights"`
		Summary          string   `json:"summary"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &analysis); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}

	return &models.AIPerformerCareer{
		PerformerID:      p.ID,
		CareerStart:      analysis.CareerStart,
		CareerEnd:        analysis.CareerEnd,
		ActiveYears:      analysis.ActiveYears,
		PrimaryNiches:    analysis.PrimaryNiches,
		NotableStudios:   analysis.NotableStudios,
		CareerHighlights: analysis.CareerHighlights,
		Summary:          analysis.Summary,
	}, nil
}

// topByCount returns the map keys sorted by descending count, up to n entries.
func topByCount(counts map[int]int, n int) []int {
	if len(counts) == 0 {
		return nil
	}

	ids := make([]int, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		if counts[ids[i]] == counts[ids[j]] {
			return ids[i] < ids[j]
		}
		return counts[ids[i]] > counts[ids[j]]
	})

	if len(ids) > n {
		ids = ids[:n]
	}
	return ids
}
