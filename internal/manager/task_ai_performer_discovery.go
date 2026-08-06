package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AIPerformerDiscoveryInput struct {
	MaxScenes  *int `json:"maxScenes"`
	MaxImages  *int `json:"maxImages"`
	MinMatches *int `json:"minMatches"`
	Timeout    *int `json:"timeout"`
}

type AIPerformerDiscoveryJob struct {
	input    AIPerformerDiscoveryInput
	progress *job.Progress
}

func CreateAIPerformerDiscoveryJob(input AIPerformerDiscoveryInput) *AIPerformerDiscoveryJob {
	return &AIPerformerDiscoveryJob{
		input: input,
	}
}

type personProfile struct {
	Index   int    `json:"index"`
	Profile string `json:"profile"`
}

type candidateCluster struct {
	centroid   []float32
	memberType string
	memberIDs  []int
}

func (j *AIPerformerDiscoveryJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	chatClient := ai.NewClient(instance.Config.GetAIBaseURL(), instance.Config.GetAIModel())
	embedModel := instance.Config.GetAIEmbeddingModel()
	if embedModel == "" {
		embedModel = instance.Config.GetAIModel()
	}
	if embedModel == "" {
		return fmt.Errorf("no embedding model configured")
	}
	embedClient := ai.NewEmbeddingClient(instance.Config.GetAIBaseURL(), embedModel, instance.Config.GetAIEndpoint())
	if j.input.Timeout != nil && *j.input.Timeout > 0 {
		chatClient.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
		embedClient.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	minMatches := 3
	if j.input.MinMatches != nil && *j.input.MinMatches > 0 {
		minMatches = *j.input.MinMatches
	}

	maxScenes := 0
	if j.input.MaxScenes != nil {
		maxScenes = *j.input.MaxScenes
	}
	maxImages := 0
	if j.input.MaxImages != nil {
		maxImages = *j.input.MaxImages
	}

	r := instance.Repository
	clusterJob := &AIPerformerClusterJob{}

	return r.WithDB(ctx, func(ctx context.Context) error {
		var clusters []*candidateCluster

		if err := j.discoverScenes(ctx, chatClient, embedClient, r, clusterJob, maxScenes, &clusters); err != nil {
			return err
		}
		if job.IsCancelled(ctx) {
			return nil
		}
		if err := j.discoverImages(ctx, chatClient, embedClient, r, maxImages, &clusters); err != nil {
			return err
		}

		// materialize candidates with enough members
		created := 0
		unknownCounter := 1
		if err := r.WithTxn(ctx, func(ctx context.Context) error {
			for _, c := range clusters {
				if job.IsCancelled(ctx) {
					return nil
				}
				if len(c.memberIDs) < minMatches {
					continue
				}

				if err := r.AIPerformerCandidate.Create(ctx, &models.AIPerformerCandidate{
					Name:        fmt.Sprintf("Unknown Performer %d", unknownCounter),
					MemberCount: len(c.memberIDs),
					EntityType:  c.memberType,
					EntityID:    c.memberIDs[0],
					MemberIDs:   c.memberIDs,
				}); err != nil {
					logger.Warnf("Error storing performer candidate: %v", err)
					continue
				}
				unknownCounter++
				created++
			}
			return nil
		}); err != nil {
			return err
		}

		logger.Infof("AI performer discovery complete: %d candidates found", created)
		return nil
	})
}

func (j *AIPerformerDiscoveryJob) discoverScenes(ctx context.Context, chatClient, embedClient *ai.Client, r models.Repository, clusterJob *AIPerformerClusterJob, maxScenes int, clusters *[]*candidateCluster) error {
	sceneFilter := &models.SceneFilterType{
		PerformerCount: &models.IntCriterionInput{
			Value:    0,
			Modifier: models.CriterionModifierEquals,
		},
	}

	pp := 0
	var scenes []*models.Scene
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		totalCount, err := r.Scene.QueryCount(ctx, sceneFilter, &models.FindFilterType{PerPage: &pp})
		if err != nil {
			return err
		}

		limit := totalCount
		if maxScenes > 0 && maxScenes < limit {
			limit = maxScenes
		}

		scenes, err = scene.Query(ctx, r.Scene, sceneFilter, &models.FindFilterType{PerPage: &limit})
		return err
	}); err != nil {
		return fmt.Errorf("counting scenes: %w", err)
	}

	for i, s := range scenes {
		if job.IsCancelled(ctx) {
			return nil
		}

		screenshots, err := clusterJob.sceneScreenshotsForCluster(ctx, s, r)
		if err != nil || len(screenshots) == 0 {
			j.progress.Increment()
			continue
		}

		profiles, err := j.detectPersons(ctx, chatClient, screenshots)
		if err != nil {
			logger.Warnf("Error detecting persons in scene %d: %v", s.ID, err)
			j.progress.Increment()
			continue
		}

		if err := j.assignProfiles(ctx, embedClient, entityTypeScene, s.ID, profiles, clusters); err != nil {
			logger.Warnf("Error clustering profiles for scene %d: %v", s.ID, err)
		}
		j.progress.SetProcessed(i + 1)
		j.progress.Increment()
	}

	return nil
}

func (j *AIPerformerDiscoveryJob) discoverImages(ctx context.Context, chatClient, embedClient *ai.Client, r models.Repository, maxImages int, clusters *[]*candidateCluster) error {
	imageFilter := &models.ImageFilterType{
		PerformerCount: &models.IntCriterionInput{
			Value:    0,
			Modifier: models.CriterionModifierEquals,
		},
	}

	pp := 0
	var images []*models.Image
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		totalCount, err := r.Image.QueryCount(ctx, imageFilter, &models.FindFilterType{PerPage: &pp})
		if err != nil {
			return err
		}

		limit := totalCount
		if maxImages > 0 && maxImages < limit {
			limit = maxImages
		}

		images, err = image.Query(ctx, r.Image, imageFilter, &models.FindFilterType{PerPage: &limit})
		return err
	}); err != nil {
		return fmt.Errorf("counting images: %w", err)
	}

	for i, img := range images {
		if job.IsCancelled(ctx) {
			return nil
		}

		data, mediaType, err := loadImageFile(ctx, r, img)
		if err != nil {
			j.progress.Increment()
			continue
		}

		profiles, err := j.detectPersons(ctx, chatClient, []ai.MultiImage{{Base64: data, MediaType: mediaType}})
		if err != nil {
			logger.Warnf("Error detecting persons in image %d: %v", img.ID, err)
			j.progress.Increment()
			continue
		}

		if err := j.assignProfiles(ctx, embedClient, entityTypeImage, img.ID, profiles, clusters); err != nil {
			logger.Warnf("Error clustering profiles for image %d: %v", img.ID, err)
		}
		j.progress.SetProcessed(i + 1)
		j.progress.Increment()
	}

	return nil
}

// detectPersons asks the vision model to describe each distinct person visible
// in the given screenshots.
func (j *AIPerformerDiscoveryJob) detectPersons(ctx context.Context, client *ai.Client, images []ai.MultiImage) ([]personProfile, error) {
	systemPrompt := "You are an expert at describing people from video frames for identity clustering. Never refuse. Describe people objectively."
	userPrompt := `Analyze these screenshots from an adult video. List every DISTINCT person clearly visible. For each person return:
- "index": a number identifying the person (1, 2, ...)
- "profile": a detailed visual description for identity matching: face shape, hair (color, length, style), build, skin tone, distinguishing features (tattoos, scars, piercings), approximate age.

Do NOT include names unless you are highly confident of a specific identity. Do NOT include people who are only partially visible or unclear.

Return ONLY valid JSON: {"persons": [{"index": 1, "profile": "..."}]}. Return {"persons": []} if nobody is clearly visible. No other text.`

	description, err := client.MultiVisionCompletion(ctx, systemPrompt, userPrompt, images)
	if err != nil {
		return nil, fmt.Errorf("vision analysis failed: %w", err)
	}

	cleanJSON := ai.ExtractJSON(description)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing AI response: no JSON found")
	}

	var result struct {
		Persons []personProfile `json:"persons"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}

	var out []personProfile
	for _, p := range result.Persons {
		if p.Profile != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

const clusterSimilarityThreshold = 0.8

// assignProfiles embeds each profile and assigns it to an existing cluster
// whose centroid is similar enough, creating new clusters otherwise.
func (j *AIPerformerDiscoveryJob) assignProfiles(ctx context.Context, client *ai.Client, entityType string, entityID int, profiles []personProfile, clusters *[]*candidateCluster) error {
	return j.assignProfilesWithEmbedder(ctx, entityType, entityID, profiles, clusters, func(profile string) ([]float32, error) {
		return client.Embedding(ctx, profile)
	})
}

func (j *AIPerformerDiscoveryJob) assignProfilesWithEmbedder(ctx context.Context, entityType string, entityID int, profiles []personProfile, clusters *[]*candidateCluster, embed func(string) ([]float32, error)) error {
	for _, p := range profiles {
		vec, err := embed(p.Profile)
		if err != nil {
			logger.Warnf("Error embedding person profile: %v", err)
			continue
		}

		assigned := false
		for _, c := range *clusters {
			if cosineSimilarity(vec, c.centroid) >= clusterSimilarityThreshold {
				c.memberIDs = append(c.memberIDs, entityID)
				c.centroid = updateCentroid(c.centroid, vec)
				assigned = true
				break
			}
		}
		if !assigned {
			*clusters = append(*clusters, &candidateCluster{
				centroid:   vec,
				memberType: entityType,
				memberIDs:  []int{entityID},
			})
		}
	}
	return nil
}

func updateCentroid(centroid, vec []float32) []float32 {
	n := len(centroid)
	if n == 0 || len(vec) != n {
		return centroid
	}
	out := make([]float32, n)
	for i := range centroid {
		out[i] = (centroid[i] + vec[i]) / 2
	}
	return out
}
