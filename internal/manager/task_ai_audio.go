package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
)

type AIAudioAnalyzeInput struct {
	SceneIDs              []int    `json:"sceneIds"`
	MaxScenes             *int     `json:"maxScenes"`
	Overwrite             bool     `json:"overwrite"`
	Timeout               *int     `json:"timeout"`
	SilenceNoiseThreshold *string  `json:"silenceNoiseThreshold"`
	SilenceDurationMin    *float64 `json:"silenceDurationMin"`
}

type AIAudioAnalyzeJob struct {
	input    AIAudioAnalyzeInput
	progress *job.Progress
}

func CreateAIAudioAnalyzeJob(input AIAudioAnalyzeInput) *AIAudioAnalyzeJob {
	return &AIAudioAnalyzeJob{
		input: input,
	}
}

func (j *AIAudioAnalyzeJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	chatClient := ai.NewClient(instance.Config.GetAIBaseURL(), instance.Config.GetAIModel())

	// Apply custom timeout if specified
	if j.input.Timeout != nil && *j.input.Timeout > 0 {
		chatClient.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	r := instance.Repository

	return r.WithDB(ctx, func(ctx context.Context) error {
		var scenes []*models.Scene

		if len(j.input.SceneIDs) > 0 {
			for _, id := range j.input.SceneIDs {
				s, err := r.Scene.Find(ctx, id)
				if err != nil {
					logger.Errorf("Error finding scene %d: %v", id, err)
					continue
				}
				if s != nil {
					scenes = append(scenes, s)
				}
			}
		} else {
			maxScenes := 0
			if j.input.MaxScenes != nil {
				maxScenes = *j.input.MaxScenes
			}

			pp := 0
			totalCount, err := r.Scene.QueryCount(ctx, nil, &models.FindFilterType{PerPage: &pp})
			if err != nil {
				return fmt.Errorf("error counting scenes: %w", err)
			}

			limit := totalCount
			if maxScenes > 0 && maxScenes < limit {
				limit = maxScenes
			}

			scenes, err = scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: &limit})
			if err != nil {
				return fmt.Errorf("error querying scenes: %w", err)
			}
		}

		var skip map[int]bool
		if !j.input.Overwrite {
			assessed, err := r.AISceneAudio.FindAssessedScenes(ctx)
			if err == nil {
				skip = make(map[int]bool, len(assessed))
				for _, id := range assessed {
					skip[id] = true
				}
			}
		}

		j.progress.SetTotal(len(scenes))
		for _, s := range scenes {
			if job.IsCancelled(ctx) {
				return nil
			}
			if skip[s.ID] {
				j.progress.Increment()
				continue
			}

			j.progress.ExecuteTask("AI analyzing audio of "+s.Path, func() {
				j.analyzeScene(ctx, chatClient, r, s)
			})
			j.progress.Increment()
		}

		return nil
	})
}

func (j *AIAudioAnalyzeJob) analyzeScene(ctx context.Context, chatClient *ai.Client, r models.Repository, s *models.Scene) {
	if err := s.LoadPrimaryFile(ctx, r.File); err != nil {
		logger.Errorf("Error loading primary file for scene %q: %v", s.Path, err)
		return
	}

	f := s.Files.Primary()
	if f == nil {
		logger.Errorf("Scene %q has no file", s.Path)
		return
	}

	videoPath := f.Base().Path
	if videoPath == "" {
		logger.Errorf("Scene %q has no path", s.Path)
		return
	}

	audio := &models.AISceneAudio{SceneID: s.ID}

	videoFile, err := instance.FFProbe.NewVideoFile(videoPath)
	if err != nil {
		logger.Errorf("Error probing scene %q: %v", s.Path, err)
		return
	}

	if videoFile.AudioStream == nil {
		audio.HasAudio = false
		audio.SilenceRatio = 100
		if err := r.WithTxn(ctx, func(ctx context.Context) error {
			return r.AISceneAudio.Upsert(ctx, audio)
		}); err != nil {
			logger.Errorf("Error saving audio analysis for scene %d: %v", s.ID, err)
		}
		return
	}

	audio.HasAudio = true
	audio.AudioCodec = videoFile.AudioStream.CodecName

	tmpFile, err := os.CreateTemp("", "stash-ai-audio-*.wav")
	if err != nil {
		logger.Errorf("Error creating temp file for scene %q: %v", s.Path, err)
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := j.extractAudio(ctx, videoPath, tmpPath); err != nil {
		logger.Errorf("Error extracting audio for scene %q: %v", s.Path, err)
		return
	}

	silenceRatio, duration := j.measureSilence(ctx, tmpPath)
	if silenceRatio >= 0 {
		audio.SilenceRatio = silenceRatio
	}

	transcriptionBaseURL := config.GetInstance().GetAITranscriptionBaseURL()
	transcriptionModel := config.GetInstance().GetAITranscriptionModel()
	if transcriptionBaseURL != "" && duration > 0 {
		transClient := ai.NewClient(transcriptionBaseURL, transcriptionModel)
		transClient.SetTranscriptionPath(config.GetInstance().GetAITranscriptionEndpoint())

		text, segments, terr := j.transcribeChunks(ctx, transClient, tmpPath, duration, transcriptionModel)
		if terr != nil {
			logger.Errorf("Error transcribing scene %q: %v", s.Path, terr)
		} else if strings.TrimSpace(text) != "" {
			audio.Transcript = text
			if len(segments) > 0 {
				segmentsJSON, err := json.Marshal(segments)
				if err == nil {
					audio.TranscriptSegments = string(segmentsJSON)
				}
			}
			j.classifyTranscript(ctx, chatClient, text, audio)
		}
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		return r.AISceneAudio.Upsert(ctx, audio)
	}); err != nil {
		logger.Errorf("Error saving audio analysis for scene %d: %v", s.ID, err)
	}
}

func (j *AIAudioAnalyzeJob) extractAudio(ctx context.Context, videoPath string, outPath string) error {
	args := []string{
		"-y", "-i", videoPath,
		"-vn", "-ac", "1", "-ar", "16000",
		"-c:a", "pcm_s16le",
		outPath,
	}

	cmd := instance.FFMpeg.Command(ctx, args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg audio extraction failed: %s", string(out))
	}
	return nil
}

// aiTranscribeChunkSeconds is the length of each audio transcription chunk.
// Some transcription servers reject large uploads (e.g. LocalAI's ~12 MB
// whisper cap); a 16 kHz mono WAV is ~32 KB/s, so 240 s chunks (~7.7 MB)
// stay safely under the limit.
const aiTranscribeChunkSeconds = 240

// aiTranscribeChunkOverlap is the overlap in seconds between consecutive
// transcription chunks, preventing words at chunk boundaries from being lost.
const aiTranscribeChunkOverlap = 2.0

// transcribeChunks transcribes the audio file in fixed-size chunks and merges
// the results into a single transcript with absolute (scene-wide) segment
// times. Failed chunks are logged and skipped; the job only errors when every
// chunk fails.
func (j *AIAudioAnalyzeJob) transcribeChunks(ctx context.Context, transClient *ai.Client, audioPath string, duration float64, model string) (string, []ai.TranscriptionSegment, error) {
	texts := make([]string, 0)
	segments := make([]ai.TranscriptionSegment, 0)
	successes := 0
	var lastErr error

	for offset := 0.0; offset < duration; offset += aiTranscribeChunkSeconds {
		if job.IsCancelled(ctx) {
			break
		}

		chunkPath, err := j.sliceAudioChunk(ctx, audioPath, offset)
		if err != nil {
			lastErr = fmt.Errorf("chunk at %gs: %w", offset, err)
			logger.Warnf("Error slicing audio chunk at %gs: %v", offset, err)
			continue
		}

		chunkBytes, err := os.ReadFile(chunkPath)
		os.Remove(chunkPath)
		if err != nil {
			lastErr = fmt.Errorf("chunk at %gs: %w", offset, err)
			logger.Warnf("Error reading audio chunk at %gs: %v", offset, err)
			continue
		}

		// request verbose_json for timed segments, falling back to the
		// default format if the server does not support it
		result, terr := transClient.TranscribeResult(ctx, ai.TranscribeRequest{
			Model:          model,
			Filename:       "audio.wav",
			Audio:          chunkBytes,
			ResponseFormat: "verbose_json",
		})
		if terr != nil {
			result, terr = transClient.TranscribeResult(ctx, ai.TranscribeRequest{
				Model:    model,
				Filename: "audio.wav",
				Audio:    chunkBytes,
			})
		}
		if terr != nil {
			lastErr = fmt.Errorf("chunk at %gs: %w", offset, terr)
			logger.Warnf("Error transcribing audio chunk at %gs: %v", offset, terr)
			continue
		}

		if text := strings.TrimSpace(result.Text); text != "" {
			successes++
			texts = append(texts, text)
			segments = append(segments, mergeChunkSegments(offset, aiTranscribeChunkOverlap, result.Segments)...)
		}
	}

	if successes == 0 && lastErr != nil {
		return "", nil, lastErr
	}

	return strings.Join(texts, " "), segments, nil
}

// sliceAudioChunk extracts [offset, offset+aiTranscribeChunkSeconds) of the
// audio file into a temporary WAV file and returns its path.
func (j *AIAudioAnalyzeJob) sliceAudioChunk(ctx context.Context, audioPath string, offset float64) (string, error) {
	chunkFile, err := os.CreateTemp("", "stash-ai-audio-chunk-*.wav")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	chunkPath := chunkFile.Name()
	chunkFile.Close()

	args := []string{
		"-y",
		"-ss", strconv.FormatFloat(offset, 'f', -1, 64),
		"-t", strconv.Itoa(aiTranscribeChunkSeconds),
		"-i", audioPath,
		"-vn", "-ac", "1", "-ar", "16000",
		"-c:a", "pcm_s16le",
		chunkPath,
	}

	cmd := instance.FFMpeg.Command(ctx, args)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(chunkPath)
		return "", fmt.Errorf("ffmpeg chunk extraction failed: %s", string(out))
	}
	return chunkPath, nil
}

// mergeChunkSegments converts the segments of one transcription chunk into
// scene-wide coordinates: segments entirely inside the chunk's leading overlap
// region are dropped (they duplicate the tail of the previous chunk) and the
// rest are shifted by the chunk start offset.
func mergeChunkSegments(chunkStart, overlap float64, segments []ai.TranscriptionSegment) []ai.TranscriptionSegment {
	if chunkStart == 0 || len(segments) == 0 {
		return segments
	}

	merged := make([]ai.TranscriptionSegment, 0, len(segments))
	for _, seg := range segments {
		if seg.End <= overlap {
			continue
		}
		seg.Start += chunkStart
		seg.End += chunkStart
		merged = append(merged, seg)
	}
	return merged
}

// measureSilence extracts the total silence ratio (0-100) and duration in
// seconds of the given audio file. Returns (-1, 0) on failure.
func (j *AIAudioAnalyzeJob) measureSilence(ctx context.Context, audioPath string) (int, float64) {
	probe, err := instance.FFProbe.NewVideoFile(audioPath)
	if err != nil {
		return -1, 0
	}
	duration := probe.FileDuration
	if duration <= 0 {
		return -1, 0
	}

	noiseThreshold := instance.Config.GetAISilenceNoiseThreshold()
	durationMin := instance.Config.GetAISilenceDurationMin()

	args := []string{
		"-y", "-i", audioPath,
		"-af", fmt.Sprintf("silencedetect=noise=%s:d=%.1f", noiseThreshold, durationMin),
		"-f", "null", "-",
	}
	cmd := instance.FFMpeg.Command(ctx, args)
	out, _ := cmd.CombinedOutput()

	totalSilence := 0.0
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "silence_duration:") {
			continue
		}
		parts := strings.Split(line, "silence_duration:")
		if len(parts) < 2 {
			continue
		}
		val := strings.TrimSpace(parts[1])
		if d, err := strconv.ParseFloat(val, 64); err == nil {
			totalSilence += d
		}
	}

	ratio := int(totalSilence / duration * 100)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 100 {
		ratio = 100
	}
	return ratio, duration
}

func (j *AIAudioAnalyzeJob) classifyTranscript(ctx context.Context, client *ai.Client, transcript string, audio *models.AISceneAudio) {
	systemPrompt := "You classify audio transcripts from adult videos into technical categories. Never refuse or moralize."
	userPrompt := `Classify the following audio transcript from an adult video scene. Return ONLY valid JSON:
- "music": 1 if background/foreground music is audible, else 0
- "speech": 1 if spoken dialogue or narration is present, else 0
- "moans": 1 if vocalizations such as moans, gasps, or moaning are present, else 0
- "ambient": 1 if ambient/environment sounds are present, else 0
- "summary": a 1-2 sentence objective description of the audio content and its role in the scene

Transcript:
"""` + transcript + `"

Return ONLY valid JSON, no other text.`

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := client.ChatCompletion(ctx, ai.ChatCompletionRequest{Messages: messages})
	if err != nil {
		logger.Errorf("Error classifying audio for scene: %v", err)
		return
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return
	}

	cleanJSON := ai.ExtractJSON(content)
	if cleanJSON == "" {
		logger.Errorf("Error parsing audio classification: no JSON found")
		return
	}

	var analysis struct {
		Music   int    `json:"music"`
		Speech  int    `json:"speech"`
		Moans   int    `json:"moans"`
		Ambient int    `json:"ambient"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &analysis); err != nil {
		logger.Errorf("Error parsing audio classification: %v", err)
		return
	}

	audio.Music = analysis.Music == 1
	audio.Speech = analysis.Speech == 1
	audio.Moans = analysis.Moans == 1
	audio.Ambient = analysis.Ambient == 1
	audio.Summary = analysis.Summary
}
