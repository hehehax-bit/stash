package manager

import (
	"context"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/job"
	"github.com/stretchr/testify/assert"
)

type jobExecFunc func(ctx context.Context, p *job.Progress) error

func (f jobExecFunc) Execute(ctx context.Context, p *job.Progress) error {
	return f(ctx, p)
}

func TestAIMaintenanceSchedule(t *testing.T) {
	mgr := &Manager{JobManager: job.NewManager()}
	m := newAIMaintenanceScheduler()
	now := time.Now()

	calls := 0
	enqueue := func(context.Context) (int, error) {
		calls++
		return 1, nil
	}

	// first run enqueues
	m.schedule(mgr, "task", "description", 24, now, enqueue)
	assert.Equal(t, 1, calls)

	// within the interval: skipped
	m.schedule(mgr, "task", "description", 24, now.Add(time.Hour), enqueue)
	assert.Equal(t, 1, calls)

	// past the interval: enqueued again
	m.schedule(mgr, "task", "description", 24, now.Add(25*time.Hour), enqueue)
	assert.Equal(t, 2, calls)
}

func TestAIMaintenanceJobRunningGuard(t *testing.T) {
	mgr := &Manager{JobManager: job.NewManager()}
	mgr.JobManager.AddWithType(context.Background(), "AI Generating Embeddings...", "ai", jobExecFunc(func(ctx context.Context, p *job.Progress) error { return nil }))

	m := newAIMaintenanceScheduler()
	calls := 0
	enqueue := func(context.Context) (int, error) {
		calls++
		return 1, nil
	}

	// a matching job is already queued: must not enqueue another
	m.schedule(mgr, "task", "AI Generating Embeddings...", 24, time.Now(), enqueue)
	assert.Equal(t, 0, calls)
}
