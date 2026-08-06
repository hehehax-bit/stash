package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

type GenerateHighlightClipJob struct {
	sceneID  int
	markerID int
	duration int
}

func CreateGenerateHighlightClipJob(sceneID, markerID, duration int) *GenerateHighlightClipJob {
	return &GenerateHighlightClipJob{
		sceneID:  sceneID,
		markerID: markerID,
		duration: duration,
	}
}

func (j *GenerateHighlightClipJob) Execute(ctx context.Context, progress *job.Progress) error {
	if j.duration <= 0 {
		j.duration = 60
	}
	if j.duration > 300 {
		j.duration = 300
	}

	r := instance.Repository

	var scene *models.Scene
	var marker *models.SceneMarker
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		scene, err = r.Scene.Find(ctx, j.sceneID)
		if err != nil {
			return err
		}
		marker, err = r.SceneMarker.Find(ctx, j.markerID)
		return err
	}); err != nil {
		return fmt.Errorf("loading scene and marker: %w", err)
	}
	if scene == nil {
		return fmt.Errorf("scene %d not found", j.sceneID)
	}
	if marker == nil {
		return fmt.Errorf("marker %d not found", j.markerID)
	}

	if err := scene.LoadPrimaryFile(ctx, r.File); err != nil {
		return fmt.Errorf("loading primary file: %w", err)
	}
	f := scene.Files.Primary()
	if f == nil {
		return fmt.Errorf("scene has no file")
	}

	clipsDir := filepath.Join(instance.Config.GetConfigPath(), "clips")
	if err := os.MkdirAll(clipsDir, 0755); err != nil {
		return fmt.Errorf("creating clips directory: %w", err)
	}

	outputPath, err := generateClip(ctx, scene, marker.Seconds, j.duration, clipsDir)
	if err != nil {
		return err
	}

	logger.Infof("Highlight clip generated: %s", outputPath)
	return nil
}

// generateClip transcodes a clip around startSeconds from the scene's primary
// file into the given directory and returns the output path.
func generateClip(ctx context.Context, scene *models.Scene, startSeconds float64, duration int, clipsDir string) (string, error) {
	r := instance.Repository
	if err := scene.LoadPrimaryFile(ctx, r.File); err != nil {
		return "", fmt.Errorf("loading primary file: %w", err)
	}
	f := scene.Files.Primary()
	if f == nil {
		return "", fmt.Errorf("scene has no file")
	}

	if duration <= 0 {
		duration = 60
	}
	if duration > 300 {
		duration = 300
	}

	outputPath := filepath.Join(clipsDir, fmt.Sprintf("scene_%d_clip_%d.mp4", scene.ID, time.Now().UnixNano()%1000000))

	start := startSeconds
	if start < 2 {
		start = 0
	} else {
		start -= 2
	}
	if f.Duration > 0 && start+float64(duration) > f.Duration {
		duration = int(f.Duration - start)
	}
	if duration <= 0 {
		return "", fmt.Errorf("marker is at the very end of the scene")
	}

	args := transcoder.Transcode(f.Base().Path, transcoder.TranscodeOptions{
		OutputPath: outputPath,
		Format:     ffmpeg.FormatMP4,
		AudioCodec: ffmpeg.AudioCodecAAC,
		StartTime:  start,
		Duration:   float64(duration),
		SlowSeek:   true,
	})

	if err := instance.FFMpeg.Generate(ctx, args); err != nil {
		return "", fmt.Errorf("generating clip: %w", err)
	}

	return outputPath, nil
}
