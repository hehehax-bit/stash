package manager

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/pkg/logger"
)

// AI maintenance scheduling constants.
const (
	aiMaintenanceTickInterval = 15 * time.Minute

	aiTaskEmbeddingRefresh = "embedding refresh"
	aiTaskAudioAnalysis    = "audio analysis"
)

// aiMaintenanceScheduler periodically enqueues AI maintenance jobs according
// to the ai.scheduled_tasks configuration.
type aiMaintenanceScheduler struct {
	mu       sync.Mutex
	lastRun  map[string]time.Time
	stopCh   chan struct{}
	stopOnce sync.Once
}

func newAIMaintenanceScheduler() *aiMaintenanceScheduler {
	return &aiMaintenanceScheduler{
		lastRun: map[string]time.Time{},
		stopCh:  make(chan struct{}),
	}
}

// StartAIMaintenance starts the scheduled AI maintenance loop.
func (s *Manager) StartAIMaintenance() {
	if s.aiMaintenance != nil {
		return
	}
	s.aiMaintenance = newAIMaintenanceScheduler()

	go func() {
		// run once shortly after startup, then on the ticker
		time.Sleep(30 * time.Second)
		s.aiMaintenance.run(s)

		ticker := time.NewTicker(aiMaintenanceTickInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.aiMaintenance.run(s)
			case <-s.aiMaintenance.stopCh:
				return
			}
		}
	}()
}

// StopAIMaintenance stops the scheduled AI maintenance loop.
func (s *Manager) StopAIMaintenance() {
	if s.aiMaintenance == nil {
		return
	}
	s.aiMaintenance.stopOnce.Do(func() {
		close(s.aiMaintenance.stopCh)
	})
}

func (m *aiMaintenanceScheduler) run(s *Manager) {
	if !instance.Config.GetAIEnabled() {
		return
	}

	cfg := instance.Config.GetAIScheduledTasks()
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.EmbeddingRefreshHours > 0 {
		m.schedule(s, aiTaskEmbeddingRefresh, "AI Generating Embeddings...", cfg.EmbeddingRefreshHours, now, func(ctx context.Context) (int, error) {
			return s.AIEmbedding(ctx, AIEmbeddingInput{StaleOnly: true})
		})
	}
	if cfg.AudioAnalysisHours > 0 {
		m.schedule(s, aiTaskAudioAnalysis, "AI Analyzing Scene Audio...", cfg.AudioAnalysisHours, now, func(ctx context.Context) (int, error) {
			return s.AIAudioAnalyze(ctx, AIAudioAnalyzeInput{})
		})
	}
}

func (m *aiMaintenanceScheduler) schedule(s *Manager, task, jobDescription string, intervalHours int, now time.Time, enqueue func(context.Context) (int, error)) {
	interval := time.Duration(intervalHours) * time.Hour
	if last, ok := m.lastRun[task]; ok && now.Sub(last) < interval {
		return
	}
	if m.jobRunning(s, jobDescription) {
		return
	}

	jobID, err := enqueue(context.Background())
	if err != nil {
		logger.Warnf("Scheduled AI task %q failed to start: %v", task, err)
		return
	}
	m.lastRun[task] = now
	logger.Infof("Scheduled AI task %q started (job %d)", task, jobID)
}

// jobRunning reports whether a job with the given description is currently
// queued or running.
func (m *aiMaintenanceScheduler) jobRunning(s *Manager, jobDescription string) bool {
	for _, j := range s.JobManager.GetQueue() {
		if strings.Contains(j.Description, jobDescription) {
			return true
		}
	}
	return false
}
