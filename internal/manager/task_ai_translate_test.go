package manager

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTranslateEntityText(t *testing.T) {
	server := newAIChatServer(t, `{"title": "Jane Doe blowjob", "details": "Jane Doe performt einen Blowjob."}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	result, err := translateEntityText(context.Background(), client, "Jane Doe blowjob", "Jane Doe performs a blowjob.", "German")

	require.NoError(t, err)
	assert.Equal(t, "Jane Doe blowjob", result["title"])
	assert.Contains(t, result["details"], "Blowjob")
}

func TestTranslateEntityText_EmptyInput(t *testing.T) {
	server := newAIChatServer(t, `{}`)
	defer server.Close()

	client := ai.NewClient(server.URL, "test-model")
	result, err := translateEntityText(context.Background(), client, "", "", "German")

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestAITranslationApply_SceneDetails(t *testing.T) {
	db := mocks.NewDatabase()
	db.AITranslation.On("FindByID", mock.Anything, int64(3)).Return(&models.AITranslation{
		ID: 3, EntityType: "scene", EntityID: 1, Field: "details", TranslatedText: "Übersetzter Text",
		Status: models.SuggestionStatusPending,
	}, nil)

	var updatedScene models.ScenePartial
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).
		Run(func(args mock.Arguments) {
			updatedScene = args.Get(2).(models.ScenePartial)
		}).
		Return(&models.Scene{ID: 1}, nil)
	db.AITranslation.On("UpdateStatus", mock.Anything, int64(3), models.SuggestionStatusAccepted).Return(nil)

	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	err := GetInstance().AITranslationApply(context.Background(), 3)

	require.NoError(t, err)
	require.NotNil(t, updatedScene.Details)
	assert.Equal(t, "Übersetzter Text", updatedScene.Details.Value)
	db.AITranslation.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}
