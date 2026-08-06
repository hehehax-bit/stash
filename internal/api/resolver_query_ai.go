package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/stashapp/stash/internal/api/urlbuilders"
	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/models"
)

func (r *queryResolver) AiConfig(ctx context.Context) (*config.AIConfig, error) {
	cfg := manager.GetInstance().Config
	return &config.AIConfig{
		Enabled:                       cfg.GetAIEnabled(),
		BaseURL:                       cfg.GetAIBaseURL(),
		Endpoint:                      cfg.GetAIEndpoint(),
		Model:                         cfg.GetAIModel(),
		EmbeddingModel:                cfg.GetAIEmbeddingModel(),
		ImageEmbeddingModel:           cfg.GetAIImageEmbeddingModel(),
		SystemPrompt:                  cfg.GetAISystemPrompt(),
		Automatic1111Enabled:          cfg.GetAIAutomatic1111Enabled(),
		Automatic1111BaseURL:          cfg.GetAIAutomatic1111BaseURL(),
		Tag:                           cfg.GetAITag(),
		TranscriptionBaseURL:          cfg.GetAITranscriptionBaseURL(),
		TranscriptionModel:            cfg.GetAITranscriptionModel(),
		TranscriptionEndpoint:         cfg.GetAITranscriptionEndpoint(),
		PerformerClusterMinConfidence: cfg.GetAIPerformerClusterMinConfidence(),
		MaxTokens:                     cfg.GetAIMaxTokens(),
		SilenceNoiseThreshold:         cfg.GetAISilenceNoiseThreshold(),
		SilenceDurationMin:            cfg.GetAISilenceDurationMin(),
		FramesToSample:                cfg.GetAIFramesToSample(),
		TranslationLanguage:           cfg.GetAITranslationLanguage(),
		ScheduledTasks:                cfg.GetAIScheduledTasks(),
	}, nil
}

type AIEmbeddingStats struct {
	Scenes     int `json:"scenes"`
	Images     int `json:"images"`
	Performers int `json:"performers"`
	Studios    int `json:"studios"`
	Tags       int `json:"tags"`
	Galleries  int `json:"galleries"`
	Total      int `json:"total"`
}

func (r *queryResolver) AiEmbeddingStats(ctx context.Context) (*AIEmbeddingStats, error) {
	var counts map[string]int
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		counts, err = r.repository.Embedding.CountByEntityType(ctx)
		return err
	}); err != nil {
		return nil, err
	}

	stats := &AIEmbeddingStats{}
	for entityType, count := range counts {
		switch entityType {
		case "scene":
			stats.Scenes = count
		case "image":
			stats.Images = count
		case "performer":
			stats.Performers = count
		case "studio":
			stats.Studios = count
		case "tag":
			stats.Tags = count
		case "gallery":
			stats.Galleries = count
		}
		stats.Total += count
	}
	return stats, nil
}

func (r *queryResolver) AiChatSessions(ctx context.Context) ([]*models.AIChatSession, error) {
	var sessions []*models.AIChatSession
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		sessions, err = r.repository.AI.FindAll(ctx)
		return err
	}); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *queryResolver) AiChatHistory(ctx context.Context, sessionID string) ([]*models.AIChatMessage, error) {
	var messages []*models.AIChatMessage
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		messages, err = r.repository.AI.FindBySessionID(ctx, sessionID)
		return err
	}); err != nil {
		return nil, err
	}
	return messages, nil
}

type AISuggestion struct {
	ID         int64         `json:"id"`
	EntityType string        `json:"entity_type"`
	EntityID   string        `json:"entity_id"`
	Title      string        `json:"title"`
	Details    string        `json:"details"`
	Performers []string      `json:"performers"`
	Tags       []string      `json:"tags"`
	Status     string        `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
	Scene      *models.Scene `json:"scene,omitempty"`
	Image      *models.Image `json:"image,omitempty"`
}

type AIPerformerMergeSuggestion struct {
	ID         int64             `json:"id"`
	Confidence float64           `json:"confidence"`
	Status     string            `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Source     *models.Performer `json:"source,omitempty"`
	Target     *models.Performer `json:"target,omitempty"`
}

type AIPerformerAppearance struct {
	MarkerID   string        `json:"marker_id"`
	Title      string        `json:"title"`
	Seconds    float64       `json:"seconds"`
	EndSeconds *float64      `json:"end_seconds"`
	Screenshot string        `json:"screenshot"`
	Scene      *models.Scene `json:"scene,omitempty"`
}

func (r *queryResolver) AiPerformerAppearances(ctx context.Context, performerID string, limit *int) ([]*AIPerformerAppearance, error) {
	id, err := strconv.Atoi(performerID)
	if err != nil {
		return nil, fmt.Errorf("converting performer id: %w", err)
	}

	maxResults := 100
	if limit != nil && *limit > 0 {
		maxResults = *limit
	}

	baseURL, _ := ctx.Value(BaseURLCtxKey).(string)

	var markers []*models.SceneMarker
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		markers, err = r.repository.SceneMarker.FindByPerformerIDs(ctx, id, maxResults)
		return err
	}); err != nil {
		return nil, err
	}

	out := make([]*AIPerformerAppearance, 0, len(markers))
	for _, m := range markers {
		appearance := &AIPerformerAppearance{
			MarkerID: fmt.Sprintf("%d", m.ID),
			Title:    m.Title,
			Seconds:  m.Seconds,
		}
		if m.EndSeconds != nil {
			appearance.EndSeconds = m.EndSeconds
		}
		appearance.Screenshot = urlbuilders.NewSceneMarkerURLBuilder(baseURL, m).GetScreenshotURL()

		if err := r.withReadTxn(ctx, func(ctx context.Context) error {
			sc, err := r.repository.Scene.Find(ctx, m.SceneID)
			if err == nil {
				appearance.Scene = sc
			}
			return nil
		}); err != nil {
			return nil, err
		}

		out = append(out, appearance)
	}

	return out, nil
}

type AIPerformerAudioStats struct {
	ScenesWithAudio int     `json:"scenes_with_audio"`
	MoanScenes      int     `json:"moan_scenes"`
	MoanRate        float64 `json:"moan_rate"`
	AvgSilence      float64 `json:"avg_silence"`
	AvgDuration     float64 `json:"avg_duration"`
}

type AIOHistoryLeaderboardEntry struct {
	Performer *models.Performer `json:"performer,omitempty"`
	OScenes   int               `json:"o_scenes"`
}

type AIMoodGroup struct {
	Mood       string `json:"mood"`
	SceneCount int    `json:"scene_count"`
}

func (r *queryResolver) AiMoodGroups(ctx context.Context) ([]*AIMoodGroup, error) {
	var found []*models.AIMoodCount
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AIMood.MoodCounts(ctx)
		return err
	}); err != nil {
		return nil, err
	}

	out := make([]*AIMoodGroup, 0, len(found))
	for _, e := range found {
		out = append(out, &AIMoodGroup{Mood: e.Mood, SceneCount: e.Count})
	}
	return out, nil
}

func (r *queryResolver) AiOHistoryLeaderboard(ctx context.Context, limit *int) ([]*AIOHistoryLeaderboardEntry, error) {
	maxResults := 10
	if limit != nil && *limit > 0 {
		maxResults = *limit
	}

	var found []*models.AOHistoryLeaderboardEntry
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.Scene.OHistoryLeaderboard(ctx, maxResults)
		return err
	}); err != nil {
		return nil, err
	}

	out := make([]*AIOHistoryLeaderboardEntry, 0, len(found))
	for _, e := range found {
		entry := &AIOHistoryLeaderboardEntry{OScenes: e.OScenes}
		if err := r.withReadTxn(ctx, func(ctx context.Context) error {
			p, err := r.repository.Performer.Find(ctx, e.PerformerID)
			if err == nil {
				entry.Performer = p
			}
			return nil
		}); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}

	return out, nil
}

type AISessionScene struct {
	SceneID    string  `json:"scene_id"`
	Title      string  `json:"title"`
	Duration   float64 `json:"duration"`
	Steam      int     `json:"steam"`
	BestMoment float64 `json:"best_moment"`
}

type AISessionPlan struct {
	Scenes       []AISessionScene `json:"scenes"`
	TotalMinutes float64          `json:"total_minutes"`
}

type AISavedMoment struct {
	MarkerID   string        `json:"marker_id"`
	Seconds    float64       `json:"seconds"`
	Title      string        `json:"title"`
	Screenshot string        `json:"screenshot"`
	CreatedAt  time.Time     `json:"created_at"`
	Scene      *models.Scene `json:"scene,omitempty"`
}

type AIOHistoryTimelineEntry struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func (r *queryResolver) AiOHistoryTimeline(ctx context.Context, days *int) ([]*AIOHistoryTimelineEntry, error) {
	n := 30
	if days != nil && *days > 0 {
		n = *days
	}

	var found []*models.AIOHistoryTimelineEntry
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.Scene.OHistoryTimeline(ctx, n)
		return err
	}); err != nil {
		return nil, err
	}

	out := make([]*AIOHistoryTimelineEntry, 0, len(found))
	for _, e := range found {
		out = append(out, &AIOHistoryTimelineEntry{Date: e.Date, Count: e.Count})
	}
	return out, nil
}

type AISavedPlan struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SceneIDs  []string  `json:"scene_ids"`
	CreatedAt time.Time `json:"created_at"`
}

type AISessionDescribeResult struct {
	DurationMinutes int      `json:"duration_minutes"`
	Moods           []string `json:"moods"`
	MinSteam        int      `json:"min_steam"`
	Vibe            string   `json:"vibe"`
	Ordering        string   `json:"ordering"`
}

func (r *queryResolver) AiSessionDescribe(ctx context.Context, text string) (*AISessionDescribeResult, error) {
	cfg := manager.GetInstance().Config
	baseURL := cfg.GetAIBaseURL()
	model := cfg.GetAIModel()
	if baseURL == "" || model == "" {
		return nil, fmt.Errorf("AI is not configured")
	}

	client := ai.NewClient(baseURL, model)
	intent, err := ai.ParseSessionIntent(ctx, client, text)
	if err != nil {
		return nil, err
	}

	duration := intent.DurationMinutes
	minSteam := intent.MinSteam
	ordering := intent.Ordering
	vibe := intent.Vibe

	return &AISessionDescribeResult{
		DurationMinutes: duration,
		Moods:           intent.Moods,
		MinSteam:        minSteam,
		Vibe:            vibe,
		Ordering:        ordering,
	}, nil
}

func (r *queryResolver) AiSavedPlans(ctx context.Context) ([]*AISavedPlan, error) {
	var found []*models.AISavedPlan
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AISavedPlan.FindAll(ctx)
		return err
	}); err != nil {
		return nil, err
	}

	out := make([]*AISavedPlan, 0, len(found))
	for _, p := range found {
		item := &AISavedPlan{
			ID:        fmt.Sprintf("%d", p.ID),
			Name:      p.Name,
			CreatedAt: time.Unix(p.CreatedAt, 0),
		}
		for _, id := range p.SceneIDs {
			item.SceneIDs = append(item.SceneIDs, fmt.Sprintf("%d", id))
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *queryResolver) AiSavedMoments(ctx context.Context) ([]*AISavedMoment, error) {
	baseURL, _ := ctx.Value(BaseURLCtxKey).(string)

	var found []*models.AISavedMoment
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AISavedMoment.FindAll(ctx)
		return err
	}); err != nil {
		return nil, err
	}

	out := make([]*AISavedMoment, 0, len(found))
	for _, s := range found {
		var marker *models.SceneMarker
		if err := r.withReadTxn(ctx, func(ctx context.Context) error {
			var err error
			marker, err = r.repository.SceneMarker.Find(ctx, s.MarkerID)
			return err
		}); err != nil || marker == nil {
			continue
		}

		item := &AISavedMoment{
			MarkerID:  fmt.Sprintf("%d", s.MarkerID),
			Seconds:   marker.Seconds,
			Title:     marker.Title,
			CreatedAt: time.Unix(s.CreatedAt, 0),
		}
		item.Screenshot = urlbuilders.NewSceneMarkerURLBuilder(baseURL, marker).GetScreenshotURL()
		if err := r.withReadTxn(ctx, func(ctx context.Context) error {
			sc, err := r.repository.Scene.Find(ctx, marker.SceneID)
			if err == nil {
				item.Scene = sc
			}
			return nil
		}); err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	return out, nil
}

func (r *queryResolver) AiSessionBuild(ctx context.Context, input AISessionBuildInput) (*AISessionPlan, error) {
	performerIDs := make([]int, len(input.PerformerIds))
	for i, id := range input.PerformerIds {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("converting performer id: %w", err)
		}
		performerIDs[i] = n
	}

	vibe := ""
	if input.Vibe != nil {
		vibe = *input.Vibe
	}
	ordering := ""
	if input.Ordering != nil {
		ordering = *input.Ordering
	}

	scenes, totalMinutes, err := manager.GetInstance().AIBuildSession(ctx, manager.AISessionBuildInput{
		DurationMinutes: input.DurationMinutes,
		PerformerIDs:    performerIDs,
		Moods:           input.Moods,
		MinSteam:        input.MinSteam,
		Vibe:            vibe,
		Limit:           input.Limit,
		Ordering:        ordering,
		Ritual:          input.Ritual,
	})
	if err != nil {
		return nil, err
	}

	out := &AISessionPlan{TotalMinutes: totalMinutes}
	for _, sc := range scenes {
		out.Scenes = append(out.Scenes, AISessionScene{
			SceneID:    fmt.Sprintf("%d", sc.SceneID),
			Title:      sc.Title,
			Duration:   sc.Duration,
			Steam:      sc.Steam,
			BestMoment: sc.BestMoment,
		})
	}
	return out, nil
}

func (r *queryResolver) AiBestMoment(ctx context.Context, sceneID string) (*float64, error) {
	if _, err := strconv.Atoi(sceneID); err != nil {
		return nil, fmt.Errorf("converting scene id: %w", err)
	}

	var best *float64
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		perPage := 100
		markers, _, err := r.repository.SceneMarker.Query(ctx, &models.SceneMarkerFilterType{
			Scenes: &models.MultiCriterionInput{
				Value:    []string{sceneID},
				Modifier: models.CriterionModifierIncludes,
			},
		}, &models.FindFilterType{PerPage: &perPage})
		if err != nil {
			return err
		}

		var bestIntensity float64
		for _, m := range markers {
			if m.Intensity != nil && *m.Intensity > bestIntensity {
				bestIntensity = *m.Intensity
				seconds := m.Seconds
				best = &seconds
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return best, nil
}

func (r *queryResolver) AiPerformerAudioStats(ctx context.Context, performerID string) (*AIPerformerAudioStats, error) {
	id, err := strconv.Atoi(performerID)
	if err != nil {
		return nil, fmt.Errorf("converting performer id: %w", err)
	}

	var stats *models.AIPerformerAudioStats
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		stats, err = r.repository.AISceneAudio.StatsByPerformer(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}
	if stats == nil {
		stats = &models.AIPerformerAudioStats{}
	}

	return &AIPerformerAudioStats{
		ScenesWithAudio: stats.ScenesWithAudio,
		MoanScenes:      stats.MoanScenes,
		MoanRate:        stats.MoanRate,
		AvgSilence:      stats.AvgSilence,
		AvgDuration:     stats.AvgDuration,
	}, nil
}

type AIMoanLeaderboardEntry struct {
	Performer  *models.Performer `json:"performer,omitempty"`
	Scenes     int               `json:"scenes"`
	MoanScenes int               `json:"moan_scenes"`
	MoanRate   float64           `json:"moan_rate"`
	AvgSilence float64           `json:"avg_silence"`
}

func (r *queryResolver) AiMoanLeaderboard(ctx context.Context, limit *int) ([]*AIMoanLeaderboardEntry, error) {
	maxResults := 10
	if limit != nil && *limit > 0 {
		maxResults = *limit
	}

	var found []*models.AIMoanLeaderboardEntry
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AISceneAudio.MoanLeaderboard(ctx, maxResults)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	out := make([]*AIMoanLeaderboardEntry, 0, len(found))
	for _, e := range found {
		entry := &AIMoanLeaderboardEntry{
			Scenes:     e.Scenes,
			MoanScenes: e.MoanScenes,
			MoanRate:   e.MoanRate,
			AvgSilence: e.AvgSilence,
		}
		if err := r.withReadTxn(ctx, func(ctx context.Context) error {
			p, err := r.repository.Performer.Find(ctx, e.PerformerID)
			if err == nil {
				entry.Performer = p
			}
			return nil
		}); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}

	return out, nil
}

func (r *queryResolver) AiPerformerMergeSuggestions(ctx context.Context, status *string) ([]*AIPerformerMergeSuggestion, error) {
	filterStatus := models.SuggestionStatusPending
	if status != nil && *status != "" {
		filterStatus = *status
	}

	var found []*models.AIPerformerSuggestion
	var out []*AIPerformerMergeSuggestion
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AIPerformerSuggestion.FindByStatus(ctx, filterStatus)
		if err != nil {
			return err
		}

		out = make([]*AIPerformerMergeSuggestion, 0, len(found))
		for _, s := range found {
			item := &AIPerformerMergeSuggestion{
				ID:         s.ID,
				Confidence: s.Confidence,
				Status:     s.Status,
				CreatedAt:  time.Unix(s.CreatedAt, 0),
				UpdatedAt:  time.Unix(s.UpdatedAt, 0),
			}
			src, err := r.repository.Performer.Find(ctx, s.SourcePerformerID)
			if err == nil {
				item.Source = src
			}
			tgt, err := r.repository.Performer.Find(ctx, s.TargetPerformerID)
			if err == nil {
				item.Target = tgt
			}
			out = append(out, item)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}

type AIAudit struct {
	ID           int64         `json:"id"`
	EntityType   string        `json:"entity_type"`
	EntityID     string        `json:"entity_id"`
	Field        string        `json:"field"`
	CurrentValue string        `json:"current_value"`
	AIValue      string        `json:"ai_value"`
	Status       string        `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	Scene        *models.Scene `json:"scene,omitempty"`
	Image        *models.Image `json:"image,omitempty"`
}

func (r *queryResolver) AiAudits(ctx context.Context, status *string) ([]*AIAudit, error) {
	filterStatus := models.SuggestionStatusPending
	if status != nil && *status != "" {
		filterStatus = *status
	}

	var found []*models.AIAudit
	var out []*AIAudit
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AIAudit.FindByStatus(ctx, filterStatus)
		if err != nil {
			return err
		}

		out = make([]*AIAudit, 0, len(found))
		for _, a := range found {
			item := &AIAudit{
				ID:           a.ID,
				EntityType:   a.EntityType,
				EntityID:     fmt.Sprintf("%d", a.EntityID),
				Field:        a.Field,
				CurrentValue: a.CurrentValue,
				AIValue:      a.AIValue,
				Status:       a.Status,
				CreatedAt:    time.Unix(a.CreatedAt, 0),
				UpdatedAt:    time.Unix(a.UpdatedAt, 0),
			}
			switch a.EntityType {
			case "scene":
				sc, err := r.repository.Scene.Find(ctx, a.EntityID)
				if err == nil {
					item.Scene = sc
				}
			case "image":
				img, err := r.repository.Image.Find(ctx, a.EntityID)
				if err == nil {
					item.Image = img
				}
			}
			out = append(out, item)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}

type AITranslation struct {
	ID             int64             `json:"id"`
	EntityType     string            `json:"entity_type"`
	EntityID       string            `json:"entity_id"`
	Language       string            `json:"language"`
	Field          string            `json:"field"`
	TranslatedText string            `json:"translated_text"`
	Status         string            `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Scene          *models.Scene     `json:"scene,omitempty"`
	Image          *models.Image     `json:"image,omitempty"`
	Performer      *models.Performer `json:"performer,omitempty"`
}

type AIPerformerCandidate struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	MemberCount int           `json:"member_count"`
	EntityType  string        `json:"entity_type"`
	EntityID    string        `json:"entity_id"`
	MemberIDs   []string      `json:"member_ids"`
	Status      string        `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Scene       *models.Scene `json:"scene,omitempty"`
	Image       *models.Image `json:"image,omitempty"`
}

func (r *queryResolver) AiPerformerCandidates(ctx context.Context, status *string) ([]*AIPerformerCandidate, error) {
	filterStatus := models.SuggestionStatusPending
	if status != nil && *status != "" {
		filterStatus = *status
	}

	var found []*models.AIPerformerCandidate
	var out []*AIPerformerCandidate
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AIPerformerCandidate.FindByStatus(ctx, filterStatus)
		if err != nil {
			return err
		}

		out = make([]*AIPerformerCandidate, 0, len(found))
		for _, c := range found {
			item := &AIPerformerCandidate{
				ID:          c.ID,
				Name:        c.Name,
				Description: c.Description,
				MemberCount: c.MemberCount,
				EntityType:  c.EntityType,
				EntityID:    fmt.Sprintf("%d", c.EntityID),
				Status:      c.Status,
				CreatedAt:   time.Unix(c.CreatedAt, 0),
				UpdatedAt:   time.Unix(c.UpdatedAt, 0),
			}
			for _, id := range c.MemberIDs {
				item.MemberIDs = append(item.MemberIDs, fmt.Sprintf("%d", id))
			}
			switch c.EntityType {
			case "scene":
				sc, err := r.repository.Scene.Find(ctx, c.EntityID)
				if err == nil {
					item.Scene = sc
				}
			case "image":
				img, err := r.repository.Image.Find(ctx, c.EntityID)
				if err == nil {
					item.Image = img
				}
			}
			out = append(out, item)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *queryResolver) AiTranslations(ctx context.Context, status *string) ([]*AITranslation, error) {
	filterStatus := models.SuggestionStatusPending
	if status != nil && *status != "" {
		filterStatus = *status
	}

	var found []*models.AITranslation
	var out []*AITranslation
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AITranslation.FindByStatus(ctx, filterStatus)
		if err != nil {
			return err
		}

		out = make([]*AITranslation, 0, len(found))
		for _, t := range found {
			item := &AITranslation{
				ID:             t.ID,
				EntityType:     t.EntityType,
				EntityID:       fmt.Sprintf("%d", t.EntityID),
				Language:       t.Language,
				Field:          t.Field,
				TranslatedText: t.TranslatedText,
				Status:         t.Status,
				CreatedAt:      time.Unix(t.CreatedAt, 0),
				UpdatedAt:      time.Unix(t.UpdatedAt, 0),
			}
			switch t.EntityType {
			case "scene":
				sc, err := r.repository.Scene.Find(ctx, t.EntityID)
				if err == nil {
					item.Scene = sc
				}
			case "image":
				img, err := r.repository.Image.Find(ctx, t.EntityID)
				if err == nil {
					item.Image = img
				}
			case "performer":
				p, err := r.repository.Performer.Find(ctx, t.EntityID)
				if err == nil {
					item.Performer = p
				}
			}
			out = append(out, item)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *queryResolver) AiSuggestions(ctx context.Context, status *string, entityType *string) ([]*AISuggestion, error) {
	var suggestions []*models.AISuggestion
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		switch {
		case status != nil && *status != "":
			suggestions, err = r.repository.AISuggestion.FindByStatus(ctx, *status)
		case entityType != nil && *entityType != "":
			suggestions, err = r.repository.AISuggestion.FindPendingByEntityType(ctx, *entityType)
		default:
			suggestions, err = r.repository.AISuggestion.FindByStatus(ctx, models.SuggestionStatusPending)
		}
		return err
	}); err != nil {
		return nil, err
	}

	var out []*AISuggestion
	for _, s := range suggestions {
		if entityType != nil && *entityType != "" && s.EntityType != *entityType {
			continue
		}

		sg := &AISuggestion{
			ID:         s.ID,
			EntityType: s.EntityType,
			EntityID:   fmt.Sprintf("%d", s.EntityID),
			Title:      s.Title,
			Details:    s.Details,
			Performers: s.Performers,
			Tags:       s.Tags,
			Status:     s.Status,
			CreatedAt:  time.Unix(s.CreatedAt, 0),
			UpdatedAt:  time.Unix(s.UpdatedAt, 0),
		}

		err := r.withReadTxn(ctx, func(ctx context.Context) error {
			switch s.EntityType {
			case "scene":
				sc, err := r.repository.Scene.Find(ctx, s.EntityID)
				if err == nil && sc != nil {
					sg.Scene = sc
				}
			case "image":
				img, err := r.repository.Image.Find(ctx, s.EntityID)
				if err == nil && img != nil {
					sg.Image = img
				}
			}
			return nil
		})
		if err != nil {
			continue
		}

		out = append(out, sg)
	}

	return out, nil
}

type AIMediaQuality struct {
	ID            int64         `json:"id"`
	EntityType    string        `json:"entity_type"`
	EntityID      string        `json:"entity_id"`
	QualityScore  int           `json:"quality_score"`
	VisualClarity int           `json:"visual_clarity"`
	Lighting      int           `json:"lighting"`
	Composition   int           `json:"composition"`
	CameraWork    int           `json:"camera_work"`
	Notes         string        `json:"notes"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	Scene         *models.Scene `json:"scene,omitempty"`
	Image         *models.Image `json:"image,omitempty"`
}

func (r *queryResolver) AiMediaQuality(ctx context.Context, entityType string, entityID string) (*AIMediaQuality, error) {
	id, err := strconv.Atoi(entityID)
	if err != nil {
		return nil, fmt.Errorf("converting entity id: %w", err)
	}

	var q *models.AIMediaQuality
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		q, err = r.repository.AIMediaQuality.FindByEntity(ctx, entityType, id)
		return err
	}); err != nil {
		return nil, err
	}

	if q == nil {
		return nil, nil
	}

	out := &AIMediaQuality{
		ID:            q.ID,
		EntityType:    q.EntityType,
		EntityID:      fmt.Sprintf("%d", q.EntityID),
		QualityScore:  q.QualityScore,
		VisualClarity: q.VisualClarity,
		Lighting:      q.Lighting,
		Composition:   q.Composition,
		CameraWork:    q.CameraWork,
		Notes:         q.Notes,
		CreatedAt:     time.Unix(q.CreatedAt, 0),
		UpdatedAt:     time.Unix(q.UpdatedAt, 0),
	}

	_ = r.withReadTxn(ctx, func(ctx context.Context) error {
		switch q.EntityType {
		case "scene":
			sc, err := r.repository.Scene.Find(ctx, q.EntityID)
			if err == nil && sc != nil {
				out.Scene = sc
			}
		case "image":
			img, err := r.repository.Image.Find(ctx, q.EntityID)
			if err == nil && img != nil {
				out.Image = img
			}
		}
		return nil
	})

	return out, nil
}

type AITranscriptSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type AISceneAudio struct {
	ID                 int64                 `json:"id"`
	SceneID            int                   `json:"scene_id"`
	HasAudio           bool                  `json:"has_audio"`
	SilenceRatio       int                   `json:"silence_ratio"`
	Music              bool                  `json:"music"`
	Speech             bool                  `json:"speech"`
	Moans              bool                  `json:"moans"`
	Ambient            bool                  `json:"ambient"`
	Transcript         string                `json:"transcript"`
	TranscriptSegments []AITranscriptSegment `json:"transcript_segments"`
	Summary            string                `json:"summary"`
	AudioCodec         string                `json:"audio_codec"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
	Scene              *models.Scene         `json:"scene,omitempty"`
}

func sceneAudioToDTO(a *models.AISceneAudio) *AISceneAudio {
	var segments []AITranscriptSegment
	if a.TranscriptSegments != "" {
		_ = json.Unmarshal([]byte(a.TranscriptSegments), &segments)
	}
	if segments == nil {
		segments = []AITranscriptSegment{}
	}

	return &AISceneAudio{
		ID:                 a.ID,
		SceneID:            a.SceneID,
		HasAudio:           a.HasAudio,
		SilenceRatio:       a.SilenceRatio,
		Music:              a.Music,
		Speech:             a.Speech,
		Moans:              a.Moans,
		Ambient:            a.Ambient,
		Transcript:         a.Transcript,
		TranscriptSegments: segments,
		Summary:            a.Summary,
		AudioCodec:         a.AudioCodec,
		CreatedAt:          time.Unix(a.CreatedAt, 0),
		UpdatedAt:          time.Unix(a.UpdatedAt, 0),
	}
}

func (r *queryResolver) AiSceneAudio(ctx context.Context, sceneID string) (*AISceneAudio, error) {
	id, err := strconv.Atoi(sceneID)
	if err != nil {
		return nil, fmt.Errorf("converting scene id: %w", err)
	}

	var a *models.AISceneAudio
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		a, err = r.repository.AISceneAudio.FindBySceneID(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}

	if a == nil {
		return nil, nil
	}

	out := sceneAudioToDTO(a)

	_ = r.withReadTxn(ctx, func(ctx context.Context) error {
		sc, err := r.repository.Scene.Find(ctx, a.SceneID)
		if err == nil && sc != nil {
			out.Scene = sc
		}
		return nil
	})

	return out, nil
}

func (r *queryResolver) AiSceneAudioSearch(ctx context.Context, query string, limit *int) ([]*AISceneAudio, error) {
	maxResults := 50
	if limit != nil && *limit > 0 {
		maxResults = *limit
	}

	var found []*models.AISceneAudio
	var out []*AISceneAudio
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		found, err = r.repository.AISceneAudio.SearchByTranscript(ctx, query, maxResults)
		if err != nil {
			return err
		}

		out = make([]*AISceneAudio, 0, len(found))
		for _, a := range found {
			item := sceneAudioToDTO(a)
			sc, err := r.repository.Scene.Find(ctx, a.SceneID)
			if err == nil && sc != nil {
				item.Scene = sc
			}
			out = append(out, item)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}

type AIPerformerCareer struct {
	ID               int64             `json:"id"`
	PerformerID      int               `json:"performer_id"`
	CareerStart      *int              `json:"career_start"`
	CareerEnd        *int              `json:"career_end"`
	ActiveYears      []string          `json:"active_years"`
	PrimaryNiches    []string          `json:"primary_niches"`
	NotableStudios   []string          `json:"notable_studios"`
	CareerHighlights string            `json:"career_highlights"`
	Summary          string            `json:"summary"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	Performer        *models.Performer `json:"performer,omitempty"`
}

func (r *queryResolver) AiPerformerCareer(ctx context.Context, performerID string) (*AIPerformerCareer, error) {
	id, err := strconv.Atoi(performerID)
	if err != nil {
		return nil, fmt.Errorf("converting performer id: %w", err)
	}

	var c *models.AIPerformerCareer
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		c, err = r.repository.AIPerformerCareer.FindByPerformerID(ctx, id)
		return err
	}); err != nil {
		return nil, err
	}

	if c == nil {
		return nil, nil
	}

	out := &AIPerformerCareer{
		ID:               c.ID,
		PerformerID:      c.PerformerID,
		CareerStart:      c.CareerStart,
		CareerEnd:        c.CareerEnd,
		ActiveYears:      c.ActiveYears,
		PrimaryNiches:    c.PrimaryNiches,
		NotableStudios:   c.NotableStudios,
		CareerHighlights: c.CareerHighlights,
		Summary:          c.Summary,
		CreatedAt:        time.Unix(c.CreatedAt, 0),
		UpdatedAt:        time.Unix(c.UpdatedAt, 0),
	}

	_ = r.withReadTxn(ctx, func(ctx context.Context) error {
		perf, err := r.repository.Performer.Find(ctx, c.PerformerID)
		if err == nil && perf != nil {
			out.Performer = perf
		}
		return nil
	})

	return out, nil
}

type AIFileRename struct {
	ID            int64         `json:"id"`
	EntityType    string        `json:"entity_type"`
	EntityID      int           `json:"entity_id"`
	CurrentName   string        `json:"current_name"`
	SuggestedName string        `json:"suggested_name"`
	Status        string        `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	Scene         *models.Scene `json:"scene,omitempty"`
	Image         *models.Image `json:"image,omitempty"`
}

func (r *queryResolver) AiFileRenames(ctx context.Context, status *string) ([]*AIFileRename, error) {
	var renames []*models.AIFileRename
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		if status == nil {
			renames, err = r.repository.AIFileRename.FindPendingByEntityType(ctx, "scene")
			if err != nil {
				return err
			}
			imageRenames, err := r.repository.AIFileRename.FindPendingByEntityType(ctx, "image")
			if err != nil {
				return err
			}
			renames = append(renames, imageRenames...)
			return nil
		}
		renames, err = r.repository.AIFileRename.FindByStatus(ctx, *status)
		return err
	}); err != nil {
		return nil, err
	}

	if len(renames) == 0 {
		return []*AIFileRename{}, nil
	}

	out := make([]*AIFileRename, 0, len(renames))
	for _, re := range renames {
		item := &AIFileRename{
			ID:            re.ID,
			EntityType:    re.EntityType,
			EntityID:      re.EntityID,
			CurrentName:   re.CurrentName,
			SuggestedName: re.SuggestedName,
			Status:        re.Status,
			CreatedAt:     time.Unix(re.CreatedAt, 0),
			UpdatedAt:     time.Unix(re.UpdatedAt, 0),
		}

		_ = r.withReadTxn(ctx, func(ctx context.Context) error {
			switch re.EntityType {
			case "scene":
				sc, err := r.repository.Scene.Find(ctx, re.EntityID)
				if err == nil && sc != nil {
					item.Scene = sc
				}
			case "image":
				img, err := r.repository.Image.Find(ctx, re.EntityID)
				if err == nil && img != nil {
					item.Image = img
				}
			}
			return nil
		})

		out = append(out, item)
	}

	return out, nil
}

type SemanticSearchInput struct {
	Query          string   `json:"query"`
	EntityTypes    []string `json:"entity_types"`
	Limit          *int     `json:"limit"`
	Model          string   `json:"model"`
	Image          string   `json:"image"`
	ImageMediaType string   `json:"image_media_type"`
}

type SemanticSearchResult struct {
	EntityType string            `json:"entity_type"`
	EntityID   string            `json:"entity_id"`
	Score      float64           `json:"score"`
	Scene      *models.Scene     `json:"scene,omitempty"`
	Performer  *models.Performer `json:"performer,omitempty"`
	Image      *models.Image     `json:"image,omitempty"`
	Gallery    *models.Gallery   `json:"gallery,omitempty"`
	Studio     *models.Studio    `json:"studio,omitempty"`
	Tag        *models.Tag       `json:"tag,omitempty"`
}

// loadSemanticSearchEntity loads the entity referenced by the result into its
// typed field. The caller must be inside a read transaction.
func loadSemanticSearchEntity(ctx context.Context, r models.Repository, result *SemanticSearchResult) {
	entityID, err := strconv.Atoi(result.EntityID)
	if err != nil {
		return
	}

	switch result.EntityType {
	case "scene":
		s, err := r.Scene.Find(ctx, entityID)
		if err == nil && s != nil {
			result.Scene = s
		}
	case "performer":
		p, err := r.Performer.Find(ctx, entityID)
		if err == nil && p != nil {
			result.Performer = p
		}
	case "image":
		img, err := r.Image.Find(ctx, entityID)
		if err == nil && img != nil {
			result.Image = img
		}
	case "gallery":
		g, err := r.Gallery.Find(ctx, entityID)
		if err == nil && g != nil {
			result.Gallery = g
		}
	case "studio":
		st, err := r.Studio.Find(ctx, entityID)
		if err == nil && st != nil {
			result.Studio = st
		}
	case "tag":
		t, err := r.Tag.Find(ctx, entityID)
		if err == nil && t != nil {
			result.Tag = t
		}
	}
}

func (r *queryResolver) SemanticSearch(ctx context.Context, args SemanticSearchInput) ([]*SemanticSearchResult, error) {
	if !manager.GetInstance().AIService.IsEnabled() {
		return nil, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	mgr := manager.GetInstance()
	cfg := mgr.Config

	imageSearch := args.Image != ""

	model := args.Model
	if model == "" {
		if imageSearch {
			model = cfg.GetAIImageEmbeddingModel()
			if model == "" {
				model = cfg.GetAIEmbeddingModel()
			}
		} else {
			model = cfg.GetAIEmbeddingModel()
		}
		if model == "" {
			model = cfg.GetAIModel()
		}
	}
	if model == "" {
		return nil, fmt.Errorf("no embedding model configured")
	}

	baseURL := cfg.GetAIBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("AI base URL is not configured")
	}

	client := ai.NewEmbeddingClient(baseURL, model, cfg.GetAIEndpoint())

	var queryEmbedding []float32
	var err error
	if imageSearch {
		mediaType := args.ImageMediaType
		if mediaType == "" {
			mediaType = "image/jpeg"
		}
		queryEmbedding, err = client.EmbeddingImage(ctx, args.Image, mediaType)
	} else {
		queryEmbedding, err = client.Embedding(ctx, args.Query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	entityTypes := args.EntityTypes
	if len(entityTypes) == 0 {
		entityTypes = []string{"scene", "performer", "image", "gallery", "studio", "tag"}
	}

	limit := 20
	if args.Limit != nil && *args.Limit > 0 {
		limit = *args.Limit
	}

	var allResults []*SemanticSearchResult

	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		for _, entityType := range entityTypes {
			similar, err := r.repository.Embedding.SearchSimilar(ctx, entityType, model, queryEmbedding, limit)
			if err != nil {
				return fmt.Errorf("searching %s: %w", entityType, err)
			}

			for _, sim := range similar {
				result := &SemanticSearchResult{
					EntityType: entityType,
					EntityID:   fmt.Sprintf("%d", sim.EntityID),
					Score:      sim.Score,
				}

				loadSemanticSearchEntity(ctx, r.repository, result)

				allResults = append(allResults, result)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return allResults, nil
}

// SimilarToEntityResult is the GraphQL return type for similarToEntity.
type SimilarToEntityResult = SemanticSearchResult

func (r *queryResolver) SimilarToEntity(ctx context.Context, entityType string, entityID string, model *string, limit *int) ([]*SemanticSearchResult, error) {
	if !manager.GetInstance().AIService.IsEnabled() {
		return nil, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	cfg := manager.GetInstance().Config

	embedModel := ""
	if model != nil {
		embedModel = *model
	}
	if embedModel == "" {
		embedModel = cfg.GetAIEmbeddingModel()
		if embedModel == "" {
			embedModel = cfg.GetAIModel()
		}
	}
	if embedModel == "" {
		return nil, fmt.Errorf("no embedding model configured")
	}

	id, err := strconv.Atoi(entityID)
	if err != nil {
		return nil, fmt.Errorf("converting entity id: %w", err)
	}

	var queryEmbedding []float32
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		queryEmbedding, err = r.repository.Embedding.FindByEntity(ctx, entityType, id, embedModel)
		return err
	}); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if queryEmbedding == nil {
		// no embedding for this entity: nothing to compare against
		return []*SemanticSearchResult{}, nil
	}

	maxResults := 20
	if limit != nil && *limit > 0 {
		maxResults = *limit
	}

	var results []*SemanticSearchResult

	err = r.withReadTxn(ctx, func(ctx context.Context) error {
		similar, err := r.repository.Embedding.SearchSimilar(ctx, entityType, embedModel, queryEmbedding, maxResults+1)
		if err != nil {
			return fmt.Errorf("searching %s: %w", entityType, err)
		}

		for _, sim := range similar {
			if sim.EntityID == id {
				continue
			}

			result := &SemanticSearchResult{
				EntityType: entityType,
				EntityID:   fmt.Sprintf("%d", sim.EntityID),
				Score:      sim.Score,
			}

			loadSemanticSearchEntity(ctx, r.repository, result)

			results = append(results, result)
			if len(results) >= maxResults {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

type SemanticDuplicateInput struct {
	EntityTypes  []string `json:"entity_types"`
	Threshold    *float64 `json:"threshold"`
	MinGroupSize *int     `json:"min_group_size"`
	Model        string   `json:"model"`
	Timeout      *int     `json:"timeout"`
}

type SemanticDuplicateEntity struct {
	EntityID  string            `json:"entity_id"`
	Scene     *models.Scene     `json:"scene,omitempty"`
	Performer *models.Performer `json:"performer,omitempty"`
	Image     *models.Image     `json:"image,omitempty"`
	Gallery   *models.Gallery   `json:"gallery,omitempty"`
	Studio    *models.Studio    `json:"studio,omitempty"`
	Tag       *models.Tag       `json:"tag,omitempty"`
}

type SemanticDuplicateGroup struct {
	EntityType string                     `json:"entity_type"`
	Entities   []*SemanticDuplicateEntity `json:"entities"`
}

func (r *queryResolver) SemanticDuplicates(ctx context.Context, input SemanticDuplicateInput) ([]*SemanticDuplicateGroup, error) {
	cfg := manager.GetInstance().Config

	model := input.Model
	if model == "" {
		model = cfg.GetAIEmbeddingModel()
		if model == "" {
			model = cfg.GetAIModel()
		}
	}
	if model == "" {
		return nil, fmt.Errorf("no embedding model configured")
	}

	threshold := 0.9
	if input.Threshold != nil {
		threshold = *input.Threshold
	}

	minGroupSize := 2
	if input.MinGroupSize != nil {
		minGroupSize = *input.MinGroupSize
	}

	entityTypes := input.EntityTypes
	if len(entityTypes) == 0 {
		entityTypes = []string{"scene", "performer", "image", "gallery", "studio", "tag"}
	}

	var groups []*SemanticDuplicateGroup

	for _, entityType := range entityTypes {
		var dupGroups []models.DuplicateGroup
		if err := r.withReadTxn(ctx, func(ctx context.Context) error {
			var err error
			dupGroups, err = r.repository.Embedding.FindNearDuplicates(ctx, entityType, model, threshold, minGroupSize, 10000)
			return err
		}); err != nil {
			continue
		}

		for _, g := range dupGroups {
			group := &SemanticDuplicateGroup{
				EntityType: entityType,
			}

			err := r.withReadTxn(ctx, func(ctx context.Context) error {
				for _, entityID := range g.EntityIDs {
					entity := &SemanticDuplicateEntity{
						EntityID: fmt.Sprintf("%d", entityID),
					}

					switch entityType {
					case "scene":
						s, err := r.repository.Scene.Find(ctx, entityID)
						if err == nil && s != nil {
							entity.Scene = s
						}
					case "performer":
						p, err := r.repository.Performer.Find(ctx, entityID)
						if err == nil && p != nil {
							entity.Performer = p
						}
					case "image":
						img, err := r.repository.Image.Find(ctx, entityID)
						if err == nil && img != nil {
							entity.Image = img
						}
					case "gallery":
						g2, err := r.repository.Gallery.Find(ctx, entityID)
						if err == nil && g2 != nil {
							entity.Gallery = g2
						}
					case "studio":
						st, err := r.repository.Studio.Find(ctx, entityID)
						if err == nil && st != nil {
							entity.Studio = st
						}
					case "tag":
						t, err := r.repository.Tag.Find(ctx, entityID)
						if err == nil && t != nil {
							entity.Tag = t
						}
					}

					group.Entities = append(group.Entities, entity)
				}
				return nil
			})
			if err != nil {
				continue
			}

			groups = append(groups, group)
		}
	}

	return groups, nil
}
