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

func TestLastRunes_ShortString(t *testing.T) {
	assert.Equal(t, "abc", lastRunes("abc", 5))
}

func TestLastRunes_Truncates(t *testing.T) {
	assert.Equal(t, "cde", lastRunes("abcde", 3))
}

func TestLastRunes_CountsRunesNotBytes(t *testing.T) {
	assert.Equal(t, "界", lastRunes("世界", 1))
}

func TestLastRunes_Empty(t *testing.T) {
	assert.Equal(t, "", lastRunes("", 3))
}

func TestSliceContains_Found(t *testing.T) {
	assert.True(t, sliceContains([]int{1, 2, 3}, 2))
}

func TestSliceContains_NotFound(t *testing.T) {
	assert.False(t, sliceContains([]int{1, 2, 3}, 9))
}

func TestSliceContains_Empty(t *testing.T) {
	assert.False(t, sliceContains([]int{}, 1))
	assert.False(t, sliceContains(nil, 1))
}

func TestFindTagForOrganize_ByName(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Blonde", true).Return(&models.Tag{ID: 3, Name: "Blonde"}, nil)

	tag, err := findTagForOrganize(context.Background(), db.Repository(), "Blonde")

	require.NoError(t, err)
	require.NotNil(t, tag)
	assert.Equal(t, 3, tag.ID)
}

func TestFindTagForOrganize_ByNameThenAlias(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Blonde", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "Blonde", true).Return(&models.Tag{ID: 4, Name: "Blonde Girl"}, nil)

	tag, err := findTagForOrganize(context.Background(), db.Repository(), "Blonde")

	require.NoError(t, err)
	require.NotNil(t, tag)
	assert.Equal(t, 4, tag.ID)
	db.Tag.AssertExpectations(t)
}

func TestFindTagForOrganize_NotFound(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Missing", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "Missing", true).Return(nil, nil)

	tag, err := findTagForOrganize(context.Background(), db.Repository(), "Missing")

	require.NoError(t, err)
	assert.Nil(t, tag)
}

func TestResolveOrCreateTag_Existing(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Anal", true).Return(&models.Tag{ID: 9, Name: "Anal"}, nil)

	j := &AITagOrganizeJob{}
	id, err := j.resolveOrCreateTag(context.Background(), db.Repository(), "Anal")

	require.NoError(t, err)
	assert.Equal(t, 9, id)
}

func TestResolveOrCreateTag_Creates(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "New Tag", true).Return(nil, nil)
	db.Tag.On("FindByAlias", mock.Anything, "New Tag", true).Return(nil, nil)
	db.Tag.On("Create", mock.Anything, mock.MatchedBy(func(input *models.CreateTagInput) bool {
		return input.Tag != nil && input.Tag.Name == "New Tag"
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.CreateTagInput).Tag.ID = 77
	}).Return(nil)

	j := &AITagOrganizeJob{}
	id, err := j.resolveOrCreateTag(context.Background(), db.Repository(), "New Tag")

	require.NoError(t, err)
	assert.Equal(t, 77, id)
	db.Tag.AssertNumberOfCalls(t, "FindByName", 2)
	db.Tag.AssertNumberOfCalls(t, "FindByAlias", 2)
}
