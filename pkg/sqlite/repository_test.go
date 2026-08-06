//go:build integration
// +build integration

package sqlite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRepositoryWiresAIFields ensures the Repository returned by Database.Repository
// populates all AI repository interfaces. Previously the AI fields were wired in
// NewDatabase but omitted from Repository(), leaving instance.Repository.AIMediaQuality
// etc. nil and causing nil pointer panics in the AI background jobs.
func TestRepositoryWiresAIFields(t *testing.T) {
	repo := db.Repository()

	assert.NotNil(t, repo.AISuggestion, "AISuggestion should be wired")
	assert.NotNil(t, repo.AIMediaQuality, "AIMediaQuality should be wired")
	assert.NotNil(t, repo.AISceneAudio, "AISceneAudio should be wired")
	assert.NotNil(t, repo.AIPerformerCareer, "AIPerformerCareer should be wired")
	assert.NotNil(t, repo.AIFileRename, "AIFileRename should be wired")
	assert.NotNil(t, repo.AI, "AI should be wired")
	assert.NotNil(t, repo.Embedding, "Embedding should be wired")
	assert.NotNil(t, repo.Memory, "Memory should be wired")
}
