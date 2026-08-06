package manager

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCosineSimilarity(t *testing.T) {
	assert.InDelta(t, 1.0, cosineSimilarity([]float32{1, 2, 3}, []float32{1, 2, 3}), 0.0001)
	assert.InDelta(t, 0.0, cosineSimilarity([]float32{1, 0}, []float32{0, 1}), 0.0001)
	assert.InDelta(t, 0.7071, cosineSimilarity([]float32{1, 1}, []float32{1, 0}), 0.001)
	assert.Equal(t, 0.0, cosineSimilarity([]float32{}, []float32{1, 2}))
	assert.Equal(t, 0.0, cosineSimilarity(nil, nil))
	assert.Equal(t, 0.0, cosineSimilarity([]float32{1}, []float32{1, 2}))
}

func TestAIPerformerSuggestionApply(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIPerformerSuggestion.On("FindByID", mock.Anything, int64(7)).Return(&models.AIPerformerSuggestion{
		ID:                7,
		SourcePerformerID: 1,
		TargetPerformerID: 2,
		Status:            models.SuggestionStatusPending,
	}, nil)
	db.Performer.On("Merge", mock.Anything, []int{1}, 2).Return(nil)
	db.AIPerformerSuggestion.On("UpdateStatus", mock.Anything, int64(7), models.SuggestionStatusAccepted).Return(nil)
	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	err := GetInstance().AIPerformerSuggestionApply(context.Background(), 7)

	require.NoError(t, err)
	db.AIPerformerSuggestion.AssertExpectations(t)
	db.Performer.AssertExpectations(t)
}

func TestAIPerformerSuggestionApply_NotPending(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIPerformerSuggestion.On("FindByID", mock.Anything, int64(7)).Return(&models.AIPerformerSuggestion{
		ID:                7,
		SourcePerformerID: 1,
		TargetPerformerID: 2,
		Status:            models.SuggestionStatusRejected,
	}, nil)
	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	err := GetInstance().AIPerformerSuggestionApply(context.Background(), 7)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestAIPerformerSuggestionReject(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIPerformerSuggestion.On("UpdateStatus", mock.Anything, int64(7), models.SuggestionStatusRejected).Return(nil)
	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	err := GetInstance().AIPerformerSuggestionReject(context.Background(), 7)

	require.NoError(t, err)
	db.AIPerformerSuggestion.AssertExpectations(t)
}
