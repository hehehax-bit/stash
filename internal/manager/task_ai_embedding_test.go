package manager

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIntPtr(t *testing.T) {
	got := intPtr(42)
	require.NotNil(t, got)
	assert.Equal(t, 42, *got)
}

func TestBuildSceneText_AllFields(t *testing.T) {
	studioID := 3
	date := models.Date{Time: time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)}
	scene := &models.Scene{
		ID:           1,
		Title:        "Some Title",
		Details:      "Some details.",
		Director:     "Director X",
		Code:         "ABC-123",
		Date:         &date,
		StudioID:     &studioID,
		TagIDs:       models.NewRelatedIDs([]int{7, 8}),
		PerformerIDs: models.NewRelatedIDs([]int{5}),
	}

	db := mocks.NewDatabase()
	db.Scene.On("Find", mock.Anything, 1).Return(scene, nil)
	db.Studio.On("Find", mock.Anything, 3).Return(&models.Studio{ID: 3, Name: "Studio S"}, nil)
	db.Scene.On("GetTagIDs", mock.Anything, 1).Return([]int{7, 8}, nil)
	db.Tag.On("FindMany", mock.Anything, []int{7, 8}).Return([]*models.Tag{
		{ID: 7, Name: "Anal"},
		{ID: 8, Name: "Blonde"},
	}, nil)
	db.Scene.On("GetPerformerIDs", mock.Anything, 1).Return([]int{5}, nil)
	db.Performer.On("FindMany", mock.Anything, []int{5}).Return([]*models.Performer{
		{ID: 5, Name: "Jane Doe"},
	}, nil)

	text, err := buildSceneText(context.Background(), db.Repository(), 1)

	require.NoError(t, err)
	assert.Contains(t, text, "Title: Some Title")
	assert.Contains(t, text, "Details: Some details.")
	assert.Contains(t, text, "Director: Director X")
	assert.Contains(t, text, "Code: ABC-123")
	assert.Contains(t, text, "Date: 2020-01-15")
	assert.Contains(t, text, "Studio: Studio S")
	assert.Contains(t, text, "Tags: Anal, Blonde")
	assert.Contains(t, text, "Performers: Jane Doe")
}

func TestBuildSceneText_Minimal(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("Find", mock.Anything, 1).Return(&models.Scene{ID: 1}, nil)
	db.Scene.On("GetTagIDs", mock.Anything, 1).Return([]int{}, nil)
	db.Scene.On("GetPerformerIDs", mock.Anything, 1).Return([]int{}, nil)

	text, err := buildSceneText(context.Background(), db.Repository(), 1)

	require.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestBuildPerformerText_AllFields(t *testing.T) {
	gender := models.GenderEnumFemale
	p := &models.Performer{
		ID:             1,
		Name:           "Jane Doe",
		Aliases:        models.NewRelatedStrings([]string{"JD"}),
		Gender:         &gender,
		Ethnicity:      "Caucasian",
		Country:        "USA",
		Details:        "Some details.",
		Disambiguation: "JR",
	}

	db := mocks.NewDatabase()
	db.Performer.On("Find", mock.Anything, 1).Return(p, nil)
	db.Performer.On("GetAliases", mock.Anything, 1).Return([]string{"JD"}, nil)

	text, err := buildPerformerText(context.Background(), db.Repository(), 1)

	require.NoError(t, err)
	assert.Contains(t, text, "Name: Jane Doe")
	assert.Contains(t, text, "Aliases: JD")
	assert.Contains(t, text, "Gender: FEMALE")
	assert.Contains(t, text, "Ethnicity: Caucasian")
	assert.Contains(t, text, "Country: USA")
	assert.Contains(t, text, "Details: Some details.")
	assert.Contains(t, text, "Also known as: JR")
}

func TestBuildTagText(t *testing.T) {
	tag := &models.Tag{ID: 1, Name: "Anal", Description: "A tag."}

	db := mocks.NewDatabase()
	db.Tag.On("Find", mock.Anything, 1).Return(tag, nil)
	db.Tag.On("GetAliases", mock.Anything, 1).Return([]string{"Anal Sex"}, nil)

	text, err := buildTagText(context.Background(), db.Repository(), 1)

	require.NoError(t, err)
	assert.True(t, strings.Contains(text, "Name: Anal"))
	assert.True(t, strings.Contains(text, "Description: A tag."))
	assert.True(t, strings.Contains(text, "Aliases: Anal Sex"))
}

func TestBuildSceneText_NotFound(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("Find", mock.Anything, 1).Return(nil, nil)

	text, err := buildSceneText(context.Background(), db.Repository(), 1)

	require.NoError(t, err)
	assert.Equal(t, "", text)
}
