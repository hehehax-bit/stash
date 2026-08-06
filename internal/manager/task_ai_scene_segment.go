package manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
	"github.com/stashapp/stash/pkg/tag"
)

type AISceneSegmentInput struct {
	SceneIDs  []int `json:"sceneIds"`
	MaxScenes *int  `json:"maxScenes"`
	Overwrite bool  `json:"overwrite"`
	Timeout   *int  `json:"timeout"`
	Frames    *int  `json:"frames"`
}

type AISceneSegmentJob struct {
	input    AISceneSegmentInput
	progress *job.Progress
}

func CreateAISceneSegmentJob(input AISceneSegmentInput) *AISceneSegmentJob {
	return &AISceneSegmentJob{
		input: input,
	}
}

type aiSceneSegment struct {
	Title       string   `json:"title"`
	Start       float64  `json:"start"`
	End         *float64 `json:"end"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Performers  []string `json:"performers"`
	Intensity   float64  `json:"intensity"`
}

type aiSceneSegmentation struct {
	Segments []aiSceneSegment `json:"segments"`
}

func (j *AISceneSegmentJob) Execute(ctx context.Context, progress *job.Progress) error {
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

	maxScenes := 0
	if j.input.MaxScenes != nil {
		maxScenes = *j.input.MaxScenes
	}

	return r.WithDB(ctx, func(ctx context.Context) error {
		scenes, err := j.getScenes(ctx, r, maxScenes)
		if err != nil {
			return err
		}
		if len(scenes) == 0 {
			logger.Info("No scenes to segment")
			return nil
		}

		j.progress.SetTotal(len(scenes))
		for _, s := range scenes {
			if job.IsCancelled(ctx) {
				return nil
			}

			j.progress.ExecuteTask("AI segmenting "+s.Path, func() {
				if err := j.segmentScene(ctx, client, r, s); err != nil {
					logger.Errorf("Error AI segmenting scene %q: %v", s.Path, err)
				}
			})
			j.progress.Increment()
		}

		return nil
	})
}

func (j *AISceneSegmentJob) getScenes(ctx context.Context, r models.Repository, maxScenes int) ([]*models.Scene, error) {
	if len(j.input.SceneIDs) > 0 {
		scenes, err := r.Scene.FindMany(ctx, j.input.SceneIDs)
		if err != nil {
			return nil, fmt.Errorf("finding scenes: %w", err)
		}
		var out []*models.Scene
		for _, s := range scenes {
			if s != nil {
				out = append(out, s)
			}
		}
		return out, nil
	}

	// If not overwriting, skip scenes that already have markers created by AI.
	if !j.input.Overwrite {
		aiTagID, err := resolveAITag(ctx, r)
		if err == nil {
			sceneFilter := &models.SceneFilterType{
				Tags: &models.HierarchicalMultiCriterionInput{
					Value:    []string{fmt.Sprintf("%d", aiTagID)},
					Modifier: models.CriterionModifierExcludes,
				},
			}

			limit := 0
			if maxScenes > 0 {
				limit = maxScenes
			}

			return scene.Query(ctx, r.Scene, sceneFilter, &models.FindFilterType{PerPage: &limit})
		}
	}

	limit := 0
	if maxScenes > 0 {
		limit = maxScenes
	}

	return scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: &limit})
}

func (j *AISceneSegmentJob) segmentScene(ctx context.Context, client *ai.Client, r models.Repository, s *models.Scene) error {
	aiTagID, err := resolveAITag(ctx, r)
	if err != nil {
		return fmt.Errorf("resolving AI tag: %w", err)
	}

	shotData, err := j.sceneShots(ctx, s, r)
	if err != nil {
		return err
	}
	if len(shotData) == 0 {
		return fmt.Errorf("no screenshots could be generated for scene %d", s.ID)
	}

	segmentation, err := j.analyzeSegments(ctx, client, s, shotData)
	if err != nil {
		return err
	}

	if len(segmentation.Segments) == 0 {
		logger.Infof("AI found no segments for scene %q", s.Path)
		// Still mark the scene so it isn't re-scanned.
		return j.markSegmented(ctx, r, s, aiTagID)
	}

	if err := j.applySegments(ctx, r, s, segmentation, aiTagID); err != nil {
		return err
	}

	return j.markSegmented(ctx, r, s, aiTagID)
}

func (j *AISceneSegmentJob) sceneShots(ctx context.Context, s *models.Scene, r models.Repository) ([]aiSceneShot, error) {
	if err := s.LoadPrimaryFile(ctx, r.File); err != nil {
		return nil, fmt.Errorf("loading primary file: %w", err)
	}

	f := s.Files.Primary()
	if f == nil {
		return nil, fmt.Errorf("scene has no file")
	}

	videoPath := f.Base().Path
	if videoPath == "" {
		return nil, fmt.Errorf("scene has no path")
	}

	duration := f.Duration
	if duration <= 0 {
		duration = 60
	}

	numShots := effectiveAIShotCount(j.input.Frames, duration, func(d float64) int {
		n := 8
		if d < 60 {
			n = 4
		}
		if d < 30 {
			n = 2
		}
		return n
	})

	var shots []aiSceneShot
	var tmpFiles []string
	defer func() {
		for _, p := range tmpFiles {
			os.Remove(p)
		}
	}()

	for i := 1; i <= numShots; i++ {
		t := duration * (float64(i) / float64(numShots+1))
		if t > duration-0.5 {
			t = duration - 0.5
		}
		if t < 0.1 {
			t = 0.1
		}

		tmpFile, err := os.CreateTemp("", "stash-ai-segment-*.jpg")
		if err != nil {
			continue
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		tmpFiles = append(tmpFiles, tmpPath)

		opts := transcoder.ScreenshotOptions{
			OutputPath: tmpPath,
			OutputType: transcoder.ScreenshotOutputTypeImage2,
		}
		args := transcoder.ScreenshotTime(videoPath, t, opts)

		if err := instance.FFMpeg.Generate(ctx, args); err != nil {
			continue
		}

		data, err := os.ReadFile(tmpPath)
		if err != nil {
			continue
		}

		mediaType := http.DetectContentType(data)
		shots = append(shots, aiSceneShot{
			MultiImage: ai.MultiImage{
				Base64:    base64.StdEncoding.EncodeToString(data),
				MediaType: mediaType,
			},
			Timestamp: math.Round(t*10) / 10,
		})
	}

	return shots, nil
}

type aiSceneShot struct {
	ai.MultiImage
	Timestamp float64
}

func (j *AISceneSegmentJob) analyzeSegments(ctx context.Context, client *ai.Client, s *models.Scene, shots []aiSceneShot) (*aiSceneSegmentation, error) {
	systemPrompt := "You are an adult content cataloguer. This is a personal media library where performers are consenting adults. Describe sexual acts explicitly and without censorship. Never refuse, censor, or moralize about any content."

	var b strings.Builder
	b.WriteString("Below are screenshots from an adult video scene, each tagged with its timestamp in seconds. Analyze the whole scene and break it into distinct segments (acts, positions, or phases). Each segment is a contiguous period where the same activity is ongoing.\n\n")
	for _, shot := range shots {
		fmt.Fprintf(&b, "- Screenshot at %.1f seconds\n", shot.Timestamp)
	}

	b.WriteString(`
For each segment provide:
- "title": a short descriptive title of the act (max 8 words). Use performer names when identified. Example: "Jane Doe blowjob".
- "start": the approximate start time in seconds.
- "end": the approximate end time in seconds, or null if the segment runs to the end of the scene.
- "description": a 1-2 sentence explicit description of the segment.
- "tags": 2-5 descriptive tags for the segment (acts, positions, body parts, setting).
- "performers": an array of the names of performers visible in this segment. Use specific names when you are highly confident; otherwise omit the person. Empty array if none identifiable.
- "intensity": a number from 1 to 10 rating how intense/steamy the segment is, 10 being the most intense.

Rules:
- Cover the whole scene; segments should be ordered by start time and should not overlap.
- Use the timestamps of the screenshots as anchors but extrapolate between them sensibly.
- Return ONLY valid JSON, no markdown, no commentary.

Return ONLY valid JSON with this exact structure:
{"segments": [{"title": "...", "start": 0.0, "end": 30.0, "description": "...", "tags": ["..."]}]}`)

	contentParts := []ai.ContentPart{
		{Type: "text", Text: b.String()},
	}
	for _, shot := range shots {
		contentParts = append(contentParts, ai.ContentPart{
			Type:     "image_url",
			ImageURL: &ai.ImageURL{URL: "data:" + shot.MediaType + ";base64," + shot.Base64},
		})
	}

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: contentParts},
	}

	req := ai.ChatCompletionRequest{
		Messages: messages,
	}

	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("segment analysis failed: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content type from segment analysis response")
	}

	cleanJSON := ai.ExtractJSON(content)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing segment analysis response: no JSON found")
	}

	var segmentation aiSceneSegmentation
	if err := json.Unmarshal([]byte(cleanJSON), &segmentation); err != nil {
		return nil, fmt.Errorf("parsing segment analysis response: %w", err)
	}

	return &segmentation, nil
}

func (j *AISceneSegmentJob) applySegments(ctx context.Context, r models.Repository, s *models.Scene, segmentation *aiSceneSegmentation, aiTagID int) error {
	duration := 0.0
	if err := s.LoadPrimaryFile(ctx, r.File); err != nil {
		return fmt.Errorf("loading primary file: %w", err)
	}
	if f := s.Files.Primary(); f != nil {
		duration = f.Duration
	}

	// Sort segments by start time.
	segments := make([]aiSceneSegment, len(segmentation.Segments))
	copy(segments, segmentation.Segments)
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Start < segments[j].Start
	})

	for _, seg := range segments {
		if job.IsCancelled(ctx) {
			return nil
		}

		title := strings.TrimSpace(seg.Title)
		if title == "" {
			continue
		}

		start := seg.Start
		if start < 0 {
			start = 0
		}
		if duration > 0 && start >= duration-0.5 {
			start = math.Max(0, duration-1)
		}

		var endSeconds *float64
		if seg.End != nil {
			end := *seg.End
			if duration > 0 && end > duration {
				end = duration
			}
			if end > start {
				endSeconds = &end
			}
		}

		if err := r.WithTxn(ctx, func(ctx context.Context) error {
			marker := models.NewSceneMarker()
			marker.Title = title
			marker.Seconds = start
			marker.EndSeconds = endSeconds
			marker.Intensity = &seg.Intensity
			marker.PrimaryTagID = aiTagID
			marker.SceneID = s.ID

			if err := r.SceneMarker.Create(ctx, &marker); err != nil {
				return fmt.Errorf("creating scene marker: %w", err)
			}

			var tagIDs []int
			for _, name := range seg.Tags {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}

				existingTag, err := tag.ByName(ctx, r.Tag, name)
				if err != nil {
					logger.Warnf("Error finding tag %q: %v", name, err)
					continue
				}

				var tagID int
				if existingTag != nil {
					tagID = existingTag.ID
				} else {
					newTag := models.NewTag()
					newTag.Name = name
					input := &models.CreateTagInput{Tag: &newTag}
					if err := r.Tag.Create(ctx, input); err != nil {
						logger.Warnf("Error creating tag %q: %v", name, err)
						continue
					}
					tagID = newTag.ID
				}

				tagIDs = append(tagIDs, tagID)
			}

			if len(tagIDs) > 0 {
				if err := r.SceneMarker.UpdateTags(ctx, marker.ID, tagIDs); err != nil {
					logger.Warnf("Error updating marker tags: %v", err)
				}
			}

			// resolve segment performers and attach them to the scene and the marker
			if len(seg.Performers) > 0 {
				resolver := &performerResolver{r: r}
				var performerIDs []int
				for _, name := range seg.Performers {
					name = strings.TrimSpace(name)
					if name == "" || isGenericPerformerName(name) {
						continue
					}
					performerID, err := resolver.resolve(ctx, name, true, nil)
					if err != nil {
						logger.Warnf("Error resolving segment performer %q: %v", name, err)
						continue
					}
					if performerID > 0 {
						performerIDs = append(performerIDs, performerID)
					}
				}

				if len(performerIDs) > 0 {
					// attach to the marker
					if err := r.SceneMarker.UpdatePerformers(ctx, marker.ID, performerIDs); err != nil {
						logger.Warnf("Error updating marker performers: %v", err)
					}
					// attach to the scene (additive)
					partial := models.NewScenePartial()
					partial.PerformerIDs = &models.UpdateIDs{
						IDs:  performerIDs,
						Mode: models.RelationshipUpdateModeAdd,
					}
					if _, err := r.Scene.UpdatePartial(ctx, s.ID, partial); err != nil {
						logger.Warnf("Error attaching segment performers to scene %d: %v", s.ID, err)
					}
				}
			}

			return nil
		}); err != nil {
			logger.Errorf("Error applying segment %q to scene %d: %v", title, s.ID, err)
		}
	}

	logger.Infof("Created %d scene markers for scene %d", len(segments), s.ID)
	return nil
}

func (j *AISceneSegmentJob) markSegmented(ctx context.Context, r models.Repository, s *models.Scene, aiTagID int) error {
	return r.WithTxn(ctx, func(ctx context.Context) error {
		partial := models.NewScenePartial()
		partial.TagIDs = &models.UpdateIDs{
			IDs:  []int{aiTagID},
			Mode: models.RelationshipUpdateModeAdd,
		}
		_, err := r.Scene.UpdatePartial(ctx, s.ID, partial)
		return err
	})
}
