package manager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AIAuditInput struct {
	EntityTypes []string `json:"entityTypes"`
	MaxItems    *int     `json:"maxItems"`
	Timeout     *int     `json:"timeout"`
}

type AIAuditJob struct {
	input    AIAuditInput
	progress *job.Progress
}

func CreateAIAuditJob(input AIAuditInput) *AIAuditJob {
	return &AIAuditJob{
		input: input,
	}
}

func (j *AIAuditJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	client := ai.NewClient(instance.Config.GetAIBaseURL(), instance.Config.GetAIModel())
	if j.input.Timeout != nil && *j.input.Timeout > 0 {
		client.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	entityTypes := j.input.EntityTypes
	if len(entityTypes) == 0 {
		entityTypes = []string{entityTypeScene, entityTypeImage}
	}

	maxItems := 0
	if j.input.MaxItems != nil {
		maxItems = *j.input.MaxItems
	}

	r := instance.Repository

	var aiTagID int
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		var err error
		aiTagID, err = resolveAITag(ctx, r)
		return err
	}); err != nil {
		return fmt.Errorf("resolving AI tag: %w", err)
	}

	sceneJob := &AISceneTagJob{}
	imageJob := &AIImageTagJob{}

	return r.WithDB(ctx, func(ctx context.Context) error {
		audited := 0
		findings := 0
		for _, entityType := range entityTypes {
			switch entityType {
			case entityTypeScene:
				count, fcount, err := j.auditScenes(ctx, client, r, sceneJob, aiTagID, maxItems)
				if err != nil {
					return err
				}
				audited += count
				findings += fcount
			case entityTypeImage:
				count, fcount, err := j.auditImages(ctx, client, r, imageJob, aiTagID, maxItems)
				if err != nil {
					return err
				}
				audited += count
				findings += fcount
			}
		}

		logger.Infof("AI audit complete: %d items audited, %d findings recorded", audited, findings)
		return nil
	})
}

func (j *AIAuditJob) auditScenes(ctx context.Context, client *ai.Client, r models.Repository, sceneJob *AISceneTagJob, aiTagID int, maxItems int) (int, int, error) {
	sceneFilter := &models.SceneFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{fmt.Sprintf("%d", aiTagID)},
			Modifier: models.CriterionModifierIncludes,
			Depth:    newInt(0),
		},
	}

	pp := 0
	var scenes []*models.Scene
	limit := 0
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		totalCount, err := r.Scene.QueryCount(ctx, sceneFilter, &models.FindFilterType{PerPage: &pp})
		if err != nil {
			return err
		}

		limit = totalCount
		if maxItems > 0 && maxItems < limit {
			limit = maxItems
		}

		scenes, err = scene.Query(ctx, r.Scene, sceneFilter, &models.FindFilterType{PerPage: &limit})
		return err
	}); err != nil {
		return 0, 0, fmt.Errorf("counting scenes: %w", err)
	}

	j.progress.SetTotal(limit)
	audited := 0
	stored := 0
	for i, s := range scenes {
		if job.IsCancelled(ctx) {
			return audited, stored, nil
		}

		analysis, aerr := sceneJob.analyzeScene(ctx, client, r, s, "")
		if aerr != nil {
			logger.Warnf("Error auditing scene %d: %v", s.ID, aerr)
			j.progress.Increment()
			continue
		}

		findings, ferr := j.sceneFindings(ctx, r, s, analysis)
		if ferr == nil && len(findings) > 0 {
			if err := r.WithTxn(ctx, func(ctx context.Context) error {
				for _, f := range findings {
					if err := r.AIAudit.Create(ctx, &models.AIAudit{
						EntityType:   entityTypeScene,
						EntityID:     s.ID,
						Field:        f.Field,
						CurrentValue: f.Current,
						AIValue:      f.AIValue,
					}); err != nil {
						logger.Warnf("Error storing audit finding for scene %d: %v", s.ID, err)
					} else {
						stored++
					}
				}
				return nil
			}); err != nil {
				logger.Warnf("Error storing audit findings for scene %d: %v", s.ID, err)
			}
		}
		audited++
		j.progress.SetProcessed(i + 1)
		j.progress.Increment()
	}

	return audited, stored, nil
}

func (j *AIAuditJob) auditImages(ctx context.Context, client *ai.Client, r models.Repository, imageJob *AIImageTagJob, aiTagID int, maxItems int) (int, int, error) {
	imageFilter := &models.ImageFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{fmt.Sprintf("%d", aiTagID)},
			Modifier: models.CriterionModifierIncludes,
			Depth:    newInt(0),
		},
	}

	pp := 0
	var images []*models.Image
	limit := 0
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		totalCount, err := r.Image.QueryCount(ctx, imageFilter, &models.FindFilterType{PerPage: &pp})
		if err != nil {
			return err
		}

		limit = totalCount
		if maxItems > 0 && maxItems < limit {
			limit = maxItems
		}

		images, err = image.Query(ctx, r.Image, imageFilter, &models.FindFilterType{PerPage: &limit})
		return err
	}); err != nil {
		return 0, 0, fmt.Errorf("counting images: %w", err)
	}

	j.progress.SetTotal(limit)
	audited := 0
	stored := 0
	for i, img := range images {
		if job.IsCancelled(ctx) {
			return audited, stored, nil
		}

		analysis, aerr := imageJob.analyzeImage(ctx, client, r, img, "")
		if aerr != nil {
			logger.Warnf("Error auditing image %d: %v", img.ID, aerr)
			j.progress.Increment()
			continue
		}

		findings, ferr := j.imageFindings(ctx, r, img, analysis)
		if ferr == nil && len(findings) > 0 {
			if err := r.WithTxn(ctx, func(ctx context.Context) error {
				for _, f := range findings {
					if err := r.AIAudit.Create(ctx, &models.AIAudit{
						EntityType:   entityTypeImage,
						EntityID:     img.ID,
						Field:        f.Field,
						CurrentValue: f.Current,
						AIValue:      f.AIValue,
					}); err != nil {
						logger.Warnf("Error storing audit finding for image %d: %v", img.ID, err)
					} else {
						stored++
					}
				}
				return nil
			}); err != nil {
				logger.Warnf("Error storing audit findings for image %d: %v", img.ID, err)
			}
		}
		audited++
		j.progress.SetProcessed(i + 1)
		j.progress.Increment()
	}

	return audited, stored, nil
}

// AIAuditFinding is a single discrepancy between the stored data and the AI's
// analysis of an entity.
type AIAuditFinding struct {
	Field   string
	Current string
	AIValue string
}

func (j *AIAuditJob) sceneFindings(ctx context.Context, r models.Repository, s *models.Scene, analysis *aiSceneAnalysis) ([]AIAuditFinding, error) {
	attached, err := attachedPerformerNames(ctx, r, s.ID, r.Scene.GetPerformerIDs, r.Performer.FindMany)
	if err != nil {
		return nil, err
	}
	return auditFindings(s.Title, s.Details, analysis.Title, analysis.Details, analysis.Performers, attached), nil
}

func (j *AIAuditJob) imageFindings(ctx context.Context, r models.Repository, img *models.Image, analysis *aiImageAnalysis) ([]AIAuditFinding, error) {
	attached, err := attachedPerformerNames(ctx, r, img.ID, r.Image.GetPerformerIDs, r.Performer.FindMany)
	if err != nil {
		return nil, err
	}
	return auditFindings(img.Title, img.Details, analysis.Title, analysis.Details, analysis.Performers, attached), nil
}

// auditFindings compares stored title/details and attached performer names
// against the AI's analysis. Returns one finding per discrepancy: empty
// title/details that the AI can fill, and identified performers that are not
// attached. Generic performer names are ignored.
func auditFindings(currentTitle, currentDetails, aiTitle, aiDetails string, performers []aiImagePerformer, attachedNames []string) []AIAuditFinding {
	var out []AIAuditFinding

	if strings.TrimSpace(currentTitle) == "" && strings.TrimSpace(aiTitle) != "" {
		out = append(out, AIAuditFinding{Field: "title", AIValue: strings.TrimSpace(aiTitle)})
	}
	if strings.TrimSpace(currentDetails) == "" && strings.TrimSpace(aiDetails) != "" {
		out = append(out, AIAuditFinding{Field: "details", AIValue: strings.TrimSpace(aiDetails)})
	}

	for _, p := range performers {
		name := strings.TrimSpace(p.Name)
		if name == "" || isGenericPerformerName(name) {
			continue
		}

		attached := false
		for _, n := range attachedNames {
			if strings.EqualFold(n, name) {
				attached = true
				break
			}
		}
		if !attached {
			out = append(out, AIAuditFinding{Field: "performers", AIValue: name})
		}
	}

	return out
}

func attachedPerformerNames(ctx context.Context, r models.Repository, entityID int, getIDs func(context.Context, int) ([]int, error), findMany func(context.Context, []int) ([]*models.Performer, error)) ([]string, error) {
	ids, err := getIDs(ctx, entityID)
	if err != nil {
		return nil, err
	}
	perfs, err := findMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(perfs))
	for _, p := range perfs {
		if p != nil {
			names = append(names, p.Name)
		}
	}
	return names, nil
}
