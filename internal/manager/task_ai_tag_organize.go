package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sliceutil/stringslice"
	"github.com/stashapp/stash/pkg/tag"
)

type AITagOrganizeInput struct {
	MaxTags    *int  `json:"maxTags"`
	AllowMerge *bool `json:"allowMerge"`
	Timeout    *int  `json:"timeout"`
}

type AITagOrganizeJob struct {
	input    AITagOrganizeInput
	progress *job.Progress
}

func CreateAITagOrganizeJob(input AITagOrganizeInput) *AITagOrganizeJob {
	return &AITagOrganizeJob{
		input: input,
	}
}

func (j *AITagOrganizeJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	baseURL := instance.Config.GetAIBaseURL()
	model := instance.Config.GetAIModel()
	client := ai.NewClient(baseURL, model)

	if j.input.Timeout != nil {
		client.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	r := instance.Repository

	return r.WithDB(ctx, func(ctx context.Context) error {
		return j.organize(ctx, client, r)
	})
}

type aiTagOrganizePlan struct {
	Merges    []aiTagMerge     `json:"merges"`
	Aliases   []aiTagAliases   `json:"aliases"`
	Hierarchy []aiTagHierarchy `json:"hierarchy"`
}

type aiTagMerge struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type aiTagAliases struct {
	Tag     string   `json:"tag"`
	Aliases []string `json:"aliases"`
}

type aiTagHierarchy struct {
	Tag    string `json:"tag"`
	Parent string `json:"parent"`
}

func (j *AITagOrganizeJob) organize(ctx context.Context, client *ai.Client, r models.Repository) error {
	allowMerge := true
	if j.input.AllowMerge != nil {
		allowMerge = *j.input.AllowMerge
	}

	maxTags := 0
	if j.input.MaxTags != nil {
		maxTags = *j.input.MaxTags
	}

	tags, err := r.Tag.All(ctx)
	if err != nil {
		return fmt.Errorf("querying tags: %w", err)
	}

	if len(tags) == 0 {
		logger.Info("No tags to organize")
		return nil
	}

	for _, t := range tags {
		if err := t.LoadAliases(ctx, r.Tag); err != nil {
			return fmt.Errorf("loading aliases for tag %d: %w", t.ID, err)
		}
	}

	if maxTags > 0 && len(tags) > maxTags {
		tags = tags[:maxTags]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Current tags (%d):\n", len(tags))
	for i, t := range tags {
		aliases := t.Aliases.List()
		if len(aliases) == 0 {
			fmt.Fprintf(&b, "%d. %q (no aliases)\n", i+1, t.Name)
		} else {
			fmt.Fprintf(&b, "%d. %q (aliases: %s)\n", i+1, t.Name, strings.Join(aliases, ", "))
		}
	}
	tagList := b.String()

	systemPrompt := "You are an expert cataloguer organizing the tag hierarchy of a personal adult media library. Return ONLY valid JSON with no markdown, no commentary, no code fences."

	userPrompt := `Below is the current list of tags, each with its existing aliases.

Your task is to propose a comprehensive organization plan:

1. "merges": identify tags that are duplicates or synonyms of each other (e.g. "POV" vs "POV sex", "BDSM" vs "Bondage"). For each group, pick the most canonical name as the destination. Each entry is {"source": "name", "destination": "name"}. Only merge when you are confident they refer to the same concept; do not be aggressive.

2. "aliases": for each tag, propose additional aliases (common synonyms, abbreviations, plural forms, alternative spellings). Each entry is {"tag": "name", "aliases": ["alias1", "alias2"]}. Do not propose an alias that is already an alias of another tag or is another tag's name.

3. "hierarchy": propose parent/child relationships to organize the tags into a sensible hierarchy (e.g. "Blowjob" is a child of "Oral"). Each entry is {"tag": "name", "parent": "name"}. A tag may have multiple parents. Avoid deep chains.

Rules:
- Only reference tag names from the list above, EXCEPT merge destinations, which may be a new canonical name.
- A merge destination that does not exist in the list will be created as a new tag; its name must not collide with an existing tag name or alias.
- If a category needs no changes, use an empty array.

` + tagList + `

Return ONLY valid JSON with this exact structure:
{"merges": [{"source": "name", "destination": "name"}], "aliases": [{"tag": "name", "aliases": ["a", "b"]}], "hierarchy": [{"tag": "name", "parent": "name"}]}`

	var plan aiTagOrganizePlan
	j.progress.ExecuteTask("Asking AI for organization plan", func() {
		err = j.planTags(ctx, client, systemPrompt, userPrompt, &plan)
	})
	if err != nil {
		return fmt.Errorf("getting organization plan: %w", err)
	}

	if !allowMerge {
		plan.Merges = nil
	}

	totalOps := len(plan.Merges) + len(plan.Aliases) + len(plan.Hierarchy)
	if totalOps == 0 {
		logger.Info("AI tag organization found no changes")
		return nil
	}

	j.progress.SetTotal(totalOps)

	mergeCount := 0
	aliasCount := 0
	hierarchyCount := 0

	j.progress.ExecuteTask("Organizing tags", func() {
		mergeCount, err = j.applyMerges(ctx, r, plan.Merges)
		if err != nil {
			logger.Errorf("Error applying tag merges: %v", err)
		}

		aliasCount, err = j.applyAliases(ctx, r, plan.Aliases)
		if err != nil {
			logger.Errorf("Error applying tag aliases: %v", err)
		}

		hierarchyCount, err = j.applyHierarchy(ctx, r, plan.Hierarchy)
		if err != nil {
			logger.Errorf("Error applying tag hierarchy: %v", err)
		}
	})

	logger.Infof("AI tag organization complete: %d merges, %d alias updates, %d hierarchy updates", mergeCount, aliasCount, hierarchyCount)

	return nil
}

func (j *AITagOrganizeJob) planTags(ctx context.Context, client *ai.Client, systemPrompt, userPrompt string, plan *aiTagOrganizePlan) error {
	req := ai.ChatCompletionRequest{
		Messages: []ai.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	text, err := j.streamPlan(ctx, client, req)
	if err != nil {
		// the AI server may not support streaming; fall back to a
		// non-streaming request.
		logger.Info("AI streaming unavailable, falling back to non-streaming request")
		response, err2 := client.ChatCompletion(ctx, req)
		if err2 != nil {
			return fmt.Errorf("chat completion: %w", err2)
		}

		description, ok := response.Choices[0].Message.Content.(string)
		if !ok {
			return fmt.Errorf("unexpected content type from response")
		}
		text = description
	}

	cleanJSON := ai.ExtractJSON(text)
	if cleanJSON == "" {
		return fmt.Errorf("parsing AI response: no JSON found")
	}

	if err := json.Unmarshal([]byte(cleanJSON), plan); err != nil {
		return fmt.Errorf("parsing AI response: %w", err)
	}

	return nil
}

func (j *AITagOrganizeJob) streamPlan(ctx context.Context, client *ai.Client, req ai.ChatCompletionRequest) (string, error) {
	const tailRunes = 80

	var full strings.Builder
	tokenCount := 0

	text, err := client.ChatCompletionStream(ctx, req, func(chunk string) {
		full.WriteString(chunk)
		tokenCount += utf8.RuneCountInString(chunk)

		desc := fmt.Sprintf("Asking AI for organization plan - %d tokens", tokenCount)
		if tail := lastRunes(full.String(), tailRunes); tail != "" {
			desc += " - ..." + tail
		}
		j.progress.SetTaskDescription(desc)
	})
	if err != nil {
		return "", err
	}

	return text, nil
}

func lastRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return string(r)
	}
	return string(r[len(r)-n:])
}

func (j *AITagOrganizeJob) applyMerges(ctx context.Context, r models.Repository, merges []aiTagMerge) (int, error) {
	count := 0
	for _, m := range merges {
		if job.IsCancelled(ctx) {
			break
		}

		sourceName := strings.TrimSpace(m.Source)
		destName := strings.TrimSpace(m.Destination)
		if sourceName != "" && destName != "" && !strings.EqualFold(sourceName, destName) {
			err := r.WithTxn(ctx, func(ctx context.Context) error {
				t, err := findTagForOrganize(ctx, r, sourceName)
				if err != nil {
					return err
				}
				if t == nil {
					logger.Warnf("Skipping merge: source tag %q not found", sourceName)
					return nil
				}
				if strings.EqualFold(t.Name, destName) {
					return nil
				}

				destID, err := j.resolveOrCreateTag(ctx, r, destName)
				if err != nil {
					return err
				}

				if err := r.Tag.Merge(ctx, []int{t.ID}, destID); err != nil {
					return fmt.Errorf("merging %q into %q: %w", sourceName, destName, err)
				}
				return nil
			})
			if err != nil {
				logger.Warnf("Error merging %q into %q: %v", sourceName, destName, err)
			} else {
				count++
			}
		}

		j.progress.Increment()
	}
	return count, nil
}

func (j *AITagOrganizeJob) applyAliases(ctx context.Context, r models.Repository, aliasUpdates []aiTagAliases) (int, error) {
	count := 0
	for _, u := range aliasUpdates {
		if job.IsCancelled(ctx) {
			break
		}

		tagName := strings.TrimSpace(u.Tag)
		aliases := stringslice.UniqueExcludeFold(stringslice.TrimSpace(u.Aliases), tagName)
		if tagName != "" && len(aliases) > 0 {
			err := r.WithTxn(ctx, func(ctx context.Context) error {
				t, err := findTagForOrganize(ctx, r, tagName)
				if err != nil {
					return err
				}
				if t == nil {
					logger.Warnf("Skipping alias update: tag %q not found", tagName)
					return nil
				}

				partial := models.NewTagPartial()
				partial.Aliases = &models.UpdateStrings{Values: aliases, Mode: models.RelationshipUpdateModeAdd}

				if err := tag.ValidateUpdate(ctx, t.ID, partial, r.Tag); err != nil {
					return fmt.Errorf("validating alias update for %q: %w", t.Name, err)
				}
				if _, err := r.Tag.UpdatePartial(ctx, t.ID, partial); err != nil {
					return fmt.Errorf("updating aliases for %q: %w", t.Name, err)
				}
				return nil
			})
			if err != nil {
				logger.Warnf("Error applying aliases to %q: %v", tagName, err)
			} else {
				count++
			}
		}

		j.progress.Increment()
	}
	return count, nil
}

func (j *AITagOrganizeJob) applyHierarchy(ctx context.Context, r models.Repository, hierarchy []aiTagHierarchy) (int, error) {
	count := 0
	for _, h := range hierarchy {
		if job.IsCancelled(ctx) {
			break
		}

		tagName := strings.TrimSpace(h.Tag)
		parentName := strings.TrimSpace(h.Parent)
		if tagName != "" && parentName != "" && !strings.EqualFold(tagName, parentName) {
			err := r.WithTxn(ctx, func(ctx context.Context) error {
				t, err := findTagForOrganize(ctx, r, tagName)
				if err != nil {
					return err
				}
				if t == nil {
					logger.Warnf("Skipping hierarchy update: tag %q not found", tagName)
					return nil
				}

				parent, err := findTagForOrganize(ctx, r, parentName)
				if err != nil {
					return err
				}
				if parent == nil {
					logger.Warnf("Skipping hierarchy update: parent tag %q not found", parentName)
					return nil
				}

				if err := t.LoadParentIDs(ctx, r.Tag); err != nil {
					return err
				}
				if sliceContains(t.ParentIDs.List(), parent.ID) {
					return nil
				}

				partial := models.NewTagPartial()
				partial.ParentIDs = &models.UpdateIDs{IDs: []int{parent.ID}, Mode: models.RelationshipUpdateModeAdd}

				if err := tag.ValidateUpdate(ctx, t.ID, partial, r.Tag); err != nil {
					return fmt.Errorf("validating hierarchy update for %q: %w", t.Name, err)
				}
				if _, err := r.Tag.UpdatePartial(ctx, t.ID, partial); err != nil {
					return fmt.Errorf("updating hierarchy for %q: %w", t.Name, err)
				}
				return nil
			})
			if err != nil {
				logger.Warnf("Error applying hierarchy %q -> %q: %v", tagName, parentName, err)
			} else {
				count++
			}
		}

		j.progress.Increment()
	}
	return count, nil
}

func (j *AITagOrganizeJob) resolveOrCreateTag(ctx context.Context, r models.Repository, name string) (int, error) {
	t, err := findTagForOrganize(ctx, r, name)
	if err != nil {
		return 0, err
	}
	if t != nil {
		return t.ID, nil
	}

	newTag := models.NewTag()
	newTag.Name = name
	newTag.ParentIDs = models.NewRelatedIDs([]int{})
	newTag.ChildIDs = models.NewRelatedIDs([]int{})
	newTag.Aliases = models.NewRelatedStrings([]string{})
	input := &models.CreateTagInput{
		Tag: &newTag,
	}
	if err := tag.ValidateCreate(ctx, newTag, r.Tag); err != nil {
		return 0, fmt.Errorf("validating destination tag %q: %w", name, err)
	}
	if err := r.Tag.Create(ctx, input); err != nil {
		return 0, fmt.Errorf("creating destination tag %q: %w", name, err)
	}
	return newTag.ID, nil
}

func findTagForOrganize(ctx context.Context, r models.Repository, name string) (*models.Tag, error) {
	t, err := tag.ByName(ctx, r.Tag, name)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return t, nil
	}
	return tag.ByAlias(ctx, r.Tag, name)
}

func sliceContains(ids []int, id int) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}
