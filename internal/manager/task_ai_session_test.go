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

func TestAIBuildSession(t *testing.T) {
	db := mocks.NewDatabase()
	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	queryResult := models.NewSceneQueryResult(db.Scene)
	db.Scene.On("Query", mock.Anything, mock.MatchedBy(func(o models.SceneQueryOptions) bool {
		return o.SceneFilter != nil && o.SceneFilter.SteamScore != nil
	})).Return(queryResult, nil)
	db.Scene.On("FindMany", mock.Anything, mock.Anything).Return([]*models.Scene{
		{ID: 1, Title: "Rough night"},
		{ID: 2, Title: "Slow evening"},
	}, nil)
	db.Scene.On("GetSteamScores", mock.Anything, mock.Anything).Return(map[int]int{1: 8, 2: 7}, nil)
	db.SceneMarker.On("Query", mock.Anything, mock.Anything, mock.Anything).Return([]*models.SceneMarker{}, 0, nil)

	scenes, totalMinutes, err := GetInstance().AIBuildSession(context.Background(), AISessionBuildInput{
		DurationMinutes: 1,
		MinSteam:        intPtr(6),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, len(scenes))
	assert.GreaterOrEqual(t, totalMinutes, 1.0)
	assert.Equal(t, 1, scenes[0].SceneID)
	assert.Equal(t, "Rough night", scenes[0].Title)
	assert.Equal(t, 8, scenes[0].Steam)
	db.Scene.AssertExpectations(t)
}
