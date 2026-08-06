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

func TestCandidateClusterAssignment(t *testing.T) {
	j := &AIPerformerDiscoveryJob{}
	clusters := []*candidateCluster{}

	embed := func(profile string) ([]float32, error) {
		switch profile {
		case "blonde woman":
			return []float32{1, 0, 0, 0}, nil
		case "bald man":
			return []float32{0, 1, 0, 0}, nil
		}
		return []float32{0, 0, 1, 0}, nil
	}

	profiles := []personProfile{
		{Index: 1, Profile: "blonde woman"},
		{Index: 2, Profile: "blonde woman"},
		{Index: 3, Profile: "bald man"},
	}

	// assign across three separate entities
	for _, p := range profiles {
		require.NoError(t, j.assignProfilesWithEmbedder(context.Background(), entityTypeScene, 1, []personProfile{p}, &clusters, embed))
	}

	require.Len(t, clusters, 2)
	var sizes []int
	for _, c := range clusters {
		sizes = append(sizes, len(c.memberIDs))
	}
	assert.ElementsMatch(t, []int{1, 2}, sizes)
}

func TestAIPerformerCandidateApply_Create(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIPerformerCandidate.On("FindByID", mock.Anything, int64(4)).Return(&models.AIPerformerCandidate{
		ID: 4, Name: "Unknown Performer 1", MemberIDs: []int{1, 2}, EntityType: "scene",
		Status: models.SuggestionStatusPending,
	}, nil)
	db.Performer.On("Create", mock.Anything, mock.MatchedBy(func(input *models.CreatePerformerInput) bool {
		return input.Performer != nil && input.Performer.Name == "Unknown Performer 1"
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.CreatePerformerInput).Performer.ID = 42
	}).Return(nil)
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).Return(&models.Scene{ID: 1}, nil)
	db.Scene.On("UpdatePartial", mock.Anything, 2, mock.Anything).Return(&models.Scene{ID: 2}, nil)
	db.AIPerformerCandidate.On("UpdateStatus", mock.Anything, int64(4), models.SuggestionStatusAccepted).Return(nil)

	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	err := GetInstance().AIPerformerCandidateApply(context.Background(), 4, nil)

	require.NoError(t, err)
	db.Performer.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
	db.AIPerformerCandidate.AssertExpectations(t)
}

func TestAIPerformerCandidateApply_MergeInto(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIPerformerCandidate.On("FindByID", mock.Anything, int64(4)).Return(&models.AIPerformerCandidate{
		ID: 4, Name: "Unknown Performer 1", MemberIDs: []int{7}, EntityType: "image",
		Status: models.SuggestionStatusPending,
	}, nil)
	db.Image.On("UpdatePartial", mock.Anything, 7, mock.Anything).Return(&models.Image{ID: 7}, nil)
	db.AIPerformerCandidate.On("UpdateStatus", mock.Anything, int64(4), models.SuggestionStatusAccepted).Return(nil)

	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	target := 99
	err := GetInstance().AIPerformerCandidateApply(context.Background(), 4, &target)

	require.NoError(t, err)
	db.Performer.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	db.Image.AssertExpectations(t)
	db.AIPerformerCandidate.AssertExpectations(t)
}

func TestApplySegments_AttachesPerformers(t *testing.T) {
	db := mocks.NewDatabase()

	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	seg := &aiSceneSegmentation{Segments: []aiSceneSegment{
		{Title: "Intro", Start: 0, Description: "Intro", Tags: []string{}, Performers: []string{"Jane Doe"}},
	}}

	// scene files: LoadPrimaryFile with no files -> f nil -> duration stays 0
	s := &models.Scene{ID: 1}
	s.Files = models.NewRelatedVideoFiles(nil)

	db.SceneMarker.On("Create", mock.Anything, mock.MatchedBy(func(m *models.SceneMarker) bool { return true })).
		Run(func(args mock.Arguments) {
			args.Get(1).(*models.SceneMarker).ID = 11
		}).Return(nil)
	db.Performer.On("FindByNames", mock.Anything, []string{"Jane Doe"}, true).Return([]*models.Performer{
		{ID: 5, Name: "Jane Doe"},
	}, nil)
	db.SceneMarker.On("UpdatePerformers", mock.Anything, 11, []int{5}).Return(nil)
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).Return(&models.Scene{ID: 1}, nil)

	j := &AISceneSegmentJob{}
	err := j.applySegments(context.Background(), db.Repository(), s, seg, 99)

	require.NoError(t, err)
	db.SceneMarker.AssertExpectations(t)
	db.Performer.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}
