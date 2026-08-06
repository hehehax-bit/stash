package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type GenerateGoonReelJob struct {
	sceneIDs         []int
	durationPerScene int
	progress         *job.Progress
}

func CreateGenerateGoonReelJob(sceneIDs []int, durationPerScene int) *GenerateGoonReelJob {
	return &GenerateGoonReelJob{
		sceneIDs:         sceneIDs,
		durationPerScene: durationPerScene,
	}
}

func (j *GenerateGoonReelJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if j.durationPerScene <= 0 {
		j.durationPerScene = 60
	}
	if j.durationPerScene > 300 {
		j.durationPerScene = 300
	}

	r := instance.Repository
	clipsDir := filepath.Join(instance.Config.GetConfigPath(), "clips")
	if err := os.MkdirAll(clipsDir, 0755); err != nil {
		return fmt.Errorf("creating clips directory: %w", err)
	}

	j.progress.SetTotal(len(j.sceneIDs))

	var clipPaths []string
	for _, sceneID := range j.sceneIDs {
		if job.IsCancelled(ctx) {
			return nil
		}

		var scene *models.Scene
		var bestMoment float64
		if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
			var err error
			scene, err = r.Scene.Find(ctx, sceneID)
			if err != nil {
				return err
			}
			perScene := 100
			markers, _, err := r.SceneMarker.Query(ctx, &models.SceneMarkerFilterType{
				Scenes: &models.MultiCriterionInput{
					Value:    []string{fmt.Sprintf("%d", sceneID)},
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
		}); err != nil {
			logger.Warnf("Goon reel: error loading scene %d: %v", sceneID, err)
			j.progress.Increment()
			continue
		}
		if scene == nil {
			j.progress.Increment()
			continue
		}

		path, err := generateClip(ctx, scene, bestMoment, j.durationPerScene, clipsDir)
		if err != nil {
			logger.Warnf("Goon reel: clip for scene %d failed: %v", sceneID, err)
			j.progress.Increment()
			continue
		}
		clipPaths = append(clipPaths, path)
		j.progress.Increment()
	}

	if len(clipPaths) == 0 {
		return fmt.Errorf("no clips could be generated")
	}

	reelPath := filepath.Join(clipsDir, fmt.Sprintf("goon_reel_%d.mp4", time.Now().Unix()))

	concatFile := filepath.Join(clipsDir, fmt.Sprintf("goon_reel_%d.txt", time.Now().Unix()))
	var b strings.Builder
	for _, p := range clipPaths {
		fmt.Fprintf(&b, "file '%s'\n", strings.ReplaceAll(p, "'", `'\''`))
	}
	if err := os.WriteFile(concatFile, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("writing concat file: %w", err)
	}
	defer os.Remove(concatFile)

	args := transcoder.Splice(concatFile, transcoder.SpliceOptions{
		OutputPath: reelPath,
		Format:     ffmpeg.FormatMP4,
		VideoCodec: &ffmpeg.VideoCodecLibX264,
		AudioCodec: ffmpeg.AudioCodecAAC,
	})
	if err := instance.FFMpeg.Generate(ctx, args); err != nil {
		return fmt.Errorf("splicing reel: %w", err)
	}

	logger.Infof("Goon reel generated: %s (%d clips)", reelPath, len(clipPaths))
	return nil
}
