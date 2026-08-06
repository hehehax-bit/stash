package manager

import (
	"testing"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stretchr/testify/assert"
)

func TestMergeChunkSegments_FirstChunk(t *testing.T) {
	segments := []ai.TranscriptionSegment{
		{ID: 0, Start: 0, End: 1, Text: "a"},
		{ID: 1, Start: 1.5, End: 3, Text: "b"},
	}

	merged := mergeChunkSegments(0, aiTranscribeChunkOverlap, segments)

	assert.Len(t, merged, 2)
	assert.InDelta(t, 0, merged[0].Start, 0.0001)
	assert.InDelta(t, 1, merged[0].End, 0.0001)
	assert.InDelta(t, 1.5, merged[1].Start, 0.0001)
	assert.InDelta(t, 3, merged[1].End, 0.0001)
}

func TestMergeChunkSegments_DropsOverlapRegion(t *testing.T) {
	segments := []ai.TranscriptionSegment{
		{ID: 0, Start: 0, End: 1.5, Text: "dup"},
		{ID: 1, Start: 1.8, End: 2.0, Text: "dup2"},
		{ID: 2, Start: 2.1, End: 4, Text: "kept"},
		{ID: 3, Start: 5, End: 6, Text: "kept2"},
	}

	merged := mergeChunkSegments(240, 2.0, segments)

	assert.Len(t, merged, 2)
	assert.InDelta(t, 242.1, merged[0].Start, 0.0001)
	assert.InDelta(t, 244, merged[0].End, 0.0001)
	assert.InDelta(t, 245, merged[1].Start, 0.0001)
	assert.InDelta(t, 246, merged[1].End, 0.0001)
}

func TestMergeChunkSegments_KeepsBoundaryCrossing(t *testing.T) {
	segments := []ai.TranscriptionSegment{
		{ID: 0, Start: 1.5, End: 3.5, Text: "crosses"},
	}

	merged := mergeChunkSegments(240, 2.0, segments)

	assert.Len(t, merged, 1)
	assert.InDelta(t, 241.5, merged[0].Start, 0.0001)
	assert.InDelta(t, 243.5, merged[0].End, 0.0001)
}

func TestMergeChunkSegments_Empty(t *testing.T) {
	assert.Nil(t, mergeChunkSegments(240, 2.0, nil))
}
