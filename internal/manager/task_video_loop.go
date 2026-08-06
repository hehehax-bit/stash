package manager

import (
	"context"
	"fmt"
	"image"
	"math/bits"
	"os"
	"strconv"

	_ "image/jpeg"
	_ "image/png"

	"github.com/corona10/goimagehash"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
	"github.com/stashapp/stash/pkg/tag"
)

const loopTagName = "Looping"

type DetectLoopingInput struct {
	MaxScenes *int  `json:"maxScenes"`
	SceneIDs  []int `json:"sceneIds"`
}

type DetectLoopingJob struct {
	input    DetectLoopingInput
	progress *job.Progress
}

func CreateDetectLoopingJob(input DetectLoopingInput) *DetectLoopingJob {
	return &DetectLoopingJob{
		input: input,
	}
}

func (j *DetectLoopingJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	r := instance.Repository

	var loopTagID int
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		t, err := tag.ByName(ctx, r.Tag, loopTagName)
		if err != nil {
			return fmt.Errorf("finding loop tag: %w", err)
		}
		if t != nil {
			loopTagID = t.ID
			return nil
		}

		newTag := models.NewTag()
		newTag.Name = loopTagName
		if err := r.Tag.Create(ctx, &models.CreateTagInput{Tag: &newTag}); err != nil {
			return fmt.Errorf("creating loop tag: %w", err)
		}
		loopTagID = newTag.ID
		return nil
	}); err != nil {
		return err
	}

	return r.WithDB(ctx, func(ctx context.Context) error {
		return j.detectLoops(ctx, r, loopTagID)
	})
}

func (j *DetectLoopingJob) detectLoops(ctx context.Context, r models.Repository, loopTagID int) error {
	// If specific scene IDs are provided, process only those
	if len(j.input.SceneIDs) > 0 {
		scenes, err := r.Scene.FindMany(ctx, j.input.SceneIDs)
		if err != nil {
			return fmt.Errorf("finding scenes: %w", err)
		}
		var found []*models.Scene
		for _, s := range scenes {
			if s != nil {
				found = append(found, s)
			}
		}
		return j.processScenes(ctx, r, loopTagID, found)
	}

	maxScenes := 0
	if j.input.MaxScenes != nil {
		maxScenes = *j.input.MaxScenes
	}

	// skip scenes that are already tagged as looping
	sceneFilter := &models.SceneFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{strconv.Itoa(loopTagID)},
			Modifier: models.CriterionModifierExcludes,
			Depth:    newInt(0),
		},
	}

	pp := 0
	totalCount, err := r.Scene.QueryCount(ctx, sceneFilter, &models.FindFilterType{PerPage: &pp})
	if err != nil {
		return fmt.Errorf("counting scenes: %w", err)
	}

	limit := totalCount
	if maxScenes > 0 && maxScenes < limit {
		limit = maxScenes
	}

	scenes, err := scene.Query(ctx, r.Scene, sceneFilter, &models.FindFilterType{PerPage: &limit})
	if err != nil {
		return fmt.Errorf("querying scenes: %w", err)
	}

	return j.processScenes(ctx, r, loopTagID, scenes)
}

func (j *DetectLoopingJob) processScenes(ctx context.Context, r models.Repository, loopTagID int, scenes []*models.Scene) error {
	j.progress.SetTotal(len(scenes))
	flagged := 0
	failures := 0
	var firstErr error
	for _, s := range scenes {
		if job.IsCancelled(ctx) {
			return nil
		}

		var processErr error
		var isLoop bool
		j.progress.ExecuteTask("Checking scene for looping "+s.Path, func() {
			isLoop, processErr = j.checkSceneLoop(ctx, r, s, loopTagID)
		})
		if processErr != nil {
			failures++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", s.Path, processErr)
			}
			logger.Errorf("Error checking scene %q for looping: %v", s.Path, processErr)
		} else if isLoop {
			flagged++
		}
		j.progress.Increment()
	}

	if len(scenes) > 0 && failures == len(scenes) {
		return fmt.Errorf("loop detection failed for all %d scenes: %w", failures, firstErr)
	}

	logger.Infof("Loop detection complete: %d scenes checked, %d flagged as looping, %d failed",
		len(scenes), flagged, failures)
	return nil
}

func (j *DetectLoopingJob) checkSceneLoop(ctx context.Context, r models.Repository, s *models.Scene, loopTagID int) (bool, error) {
	if err := s.LoadPrimaryFile(ctx, r.File); err != nil {
		return false, fmt.Errorf("loading primary file: %w", err)
	}

	f := s.Files.Primary()
	if f == nil {
		return false, fmt.Errorf("scene has no file")
	}

	videoPath := f.Base().Path
	if videoPath == "" {
		return false, fmt.Errorf("scene has no path")
	}

	duration := f.Duration
	if duration < 2 {
		return false, nil
	}

	hashes, err := j.frameHashes(ctx, videoPath, duration)
	if err != nil {
		return false, fmt.Errorf("sampling frames: %w", err)
	}
	if len(hashes) < 4 {
		return false, nil
	}

	if period, ok := loopPeriodFromHashes(hashes, 20); ok {
		logger.Infof("Scene %q detected as looping (period %d)", s.Path, period)

		if err := r.WithTxn(ctx, func(ctx context.Context) error {
			partial := models.NewScenePartial()
			partial.TagIDs = &models.UpdateIDs{
				IDs:  []int{loopTagID},
				Mode: models.RelationshipUpdateModeAdd,
			}
			_, err := r.Scene.UpdatePartial(ctx, s.ID, partial)
			return err
		}); err != nil {
			return false, fmt.Errorf("updating scene: %w", err)
		}
		return true, nil
	}

	return false, nil
}

// frameHashes extracts frames at even intervals from the video, including the
// first and last frames (the loop seam), and computes a perceptual hash for
// each.
func (j *DetectLoopingJob) frameHashes(ctx context.Context, videoPath string, duration float64) ([]uint64, error) {
	const numFrames = 8

	var hashes []uint64
	var tmpFiles []string
	defer func() {
		for _, p := range tmpFiles {
			os.Remove(p)
		}
	}()

	for i := 0; i < numFrames; i++ {
		t := duration * (float64(i) / float64(numFrames-1))
		if t < 0.1 {
			t = 0.1
		}
		if i == numFrames-1 {
			// the last sample targets the loop seam (the final frame). Seeking
			// exactly at the end can produce no output, so back off in steps.
			var hash uint64
			var ok bool
			for _, off := range []float64{0.1, 0.3, 0.5} {
				hash, ok = j.extractFrameHash(ctx, videoPath, duration-off, &tmpFiles)
				if ok {
					break
				}
			}
			if !ok {
				return nil, fmt.Errorf("generating screenshot near end of video")
			}
			hashes = append(hashes, hash)
			continue
		}
		if t > duration-0.5 {
			t = duration - 0.5
		}

		hash, ok := j.extractFrameHash(ctx, videoPath, t, &tmpFiles)
		if !ok {
			return nil, fmt.Errorf("generating screenshot at %.1fs", t)
		}
		hashes = append(hashes, hash)
	}

	return hashes, nil
}

// extractFrameHash renders a single frame at the given time and returns its
// perceptual hash. The generated screenshot file is registered in tmpFiles for
// cleanup.
func (j *DetectLoopingJob) extractFrameHash(ctx context.Context, videoPath string, t float64, tmpFiles *[]string) (uint64, bool) {
	tmpFile, err := os.CreateTemp("", "stash-loop-*.jpg")
	if err != nil {
		return 0, false
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	*tmpFiles = append(*tmpFiles, tmpPath)

	opts := transcoder.ScreenshotOptions{
		OutputPath: tmpPath,
		OutputType: transcoder.ScreenshotOutputTypeImage2,
	}
	args := transcoder.ScreenshotTime(videoPath, t, opts)
	if err := instance.FFMpeg.Generate(ctx, args); err != nil {
		return 0, false
	}

	img, err := decodeImageFile(tmpPath)
	if err != nil {
		return 0, false
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return 0, false
	}
	return hash.GetHash(), true
}

func decodeImageFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	return img, err
}

// loopPeriodFromHashes reports whether a sequence of perceptual hashes sampled
// evenly across a video (including its first and last frames) indicates a
// looping video, returning the detected loop period in frame indices when
// found.
//
// A video is considered looping when:
//   - its first and last frames are nearly identical (the loop seam: playback
//     transitions seamlessly from the end back to the start) AND the content
//     just before the end continues into the content just after the start
//     (wrapped continuity), or
//   - its content repeats internally (every pair of frames separated by a
//     candidate period of n/2, n/3, or n/4 frames is nearly identical).
//
// Wrapped continuity distinguishes genuine loops from videos that merely fade
// to black (or another solid frame) at both ends: their seam matches but the
// surrounding content does not. Videos with no visual variation at all (a
// single repeated frame) are not flagged, since they are stills rather than
// loops.
func loopPeriodFromHashes(hashes []uint64, maxDistance int) (int, bool) {
	n := len(hashes)
	if n < 4 {
		return 0, false
	}

	// exclude stills: no variation between any pair of sampled frames
	hasVariation := false
	for i := 0; i < n; i++ {
		for k := i + 1; k < n; k++ {
			if hammingDistance(hashes[i], hashes[k]) > 0 {
				hasVariation = true
				break
			}
		}
		if hasVariation {
			break
		}
	}
	if !hasVariation {
		return 0, false
	}

	// the core looping signal: the end frame transitions seamlessly into the
	// first frame (the loop seam). Require wrapped continuity as well: the
	// frames just before the end must match the frames just after the start,
	// otherwise the matching seam is a coincidence (e.g. a fade to black).
	if hammingDistance(hashes[0], hashes[n-1]) <= maxDistance {
		wrapped := true
		for k := 1; k <= 3 && n-1-k >= k; k++ {
			if hammingDistance(hashes[n-1-k], hashes[k]) > maxDistance {
				wrapped = false
				break
			}
		}
		if wrapped {
			return n - 1, true
		}
	}

	for _, p := range []int{n / 2, n / 3, n / 4} {
		if p < 1 {
			continue
		}

		matched := true
		for i := 0; i+p < n; i++ {
			if hammingDistance(hashes[i], hashes[i+p]) > maxDistance {
				matched = false
				break
			}
		}
		if matched {
			return p, true
		}
	}

	return 0, false
}

func hammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}
