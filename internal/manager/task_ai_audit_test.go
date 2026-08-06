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

func TestAuditFindings(t *testing.T) {
	performers := []aiImagePerformer{
		{Name: "Jane Doe"},
		{Name: "unknown woman"},
	}

	// everything present: no findings
	findings := auditFindings("Title", "Details", "Title", "Details", performers, []string{"Jane Doe"})
	assert.Empty(t, findings)

	// missing title and details, unattached performer
	findings = auditFindings("", "", "Jane Doe blowjob", "Jane Doe performs.", performers, nil)
	require.Len(t, findings, 3)
	assert.Equal(t, "title", findings[0].Field)
	assert.Equal(t, "details", findings[1].Field)
	assert.Equal(t, "performers", findings[2].Field)
	assert.Equal(t, "Jane Doe", findings[2].AIValue)

	// generic performer names are ignored
	findings = auditFindings("", "", "", "", []aiImagePerformer{{Name: "woman"}}, nil)
	assert.Empty(t, findings)

	// case-insensitive performer comparison
	findings = auditFindings("", "", "", "", []aiImagePerformer{{Name: "Jane Doe"}}, []string{"jane doe"})
	assert.Empty(t, findings)
}

func TestAIAuditApply_Title(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIAudit.On("FindByID", mock.Anything, int64(5)).Return(&models.AIAudit{
		ID: 5, EntityType: "scene", EntityID: 1, Field: "title", AIValue: "New Title",
		Status: models.SuggestionStatusPending,
	}, nil)

	var updatedScene models.ScenePartial
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).
		Run(func(args mock.Arguments) {
			updatedScene = args.Get(2).(models.ScenePartial)
		}).
		Return(&models.Scene{ID: 1}, nil)
	db.AIAudit.On("UpdateStatus", mock.Anything, int64(5), models.SuggestionStatusAccepted).Return(nil)

	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	err := GetInstance().AIAuditApply(context.Background(), 5)

	require.NoError(t, err)
	require.NotNil(t, updatedScene.Title)
	assert.Equal(t, "New Title", updatedScene.Title.Value)
	db.AIAudit.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}

func TestAIAuditApply_Performer(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIAudit.On("FindByID", mock.Anything, int64(5)).Return(&models.AIAudit{
		ID: 5, EntityType: "image", EntityID: 1, Field: "performers", AIValue: "Jane Doe",
		Status: models.SuggestionStatusPending,
	}, nil)
	db.Performer.On("FindByNames", mock.Anything, []string{"Jane Doe"}, true).Return([]*models.Performer{
		{ID: 9, Name: "Jane Doe"},
	}, nil)

	var updatedImage models.ImagePartial
	db.Image.On("UpdatePartial", mock.Anything, 1, mock.Anything).
		Run(func(args mock.Arguments) {
			updatedImage = args.Get(2).(models.ImagePartial)
		}).
		Return(&models.Image{ID: 1}, nil)
	db.AIAudit.On("UpdateStatus", mock.Anything, int64(5), models.SuggestionStatusAccepted).Return(nil)

	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	err := GetInstance().AIAuditApply(context.Background(), 5)

	require.NoError(t, err)
	require.NotNil(t, updatedImage.PerformerIDs)
	assert.Equal(t, []int{9}, updatedImage.PerformerIDs.IDs)
	db.AIAudit.AssertExpectations(t)
	db.Performer.AssertExpectations(t)
	db.Image.AssertExpectations(t)
}

func TestAIAuditApplyAll(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIAudit.On("FindByStatus", mock.Anything, models.SuggestionStatusPending).Return([]*models.AIAudit{
		{ID: 5, EntityType: "scene", EntityID: 1, Field: "title", AIValue: "New Title", Status: models.SuggestionStatusPending},
	}, nil)
	db.AIAudit.On("FindByID", mock.Anything, int64(5)).Return(&models.AIAudit{
		ID: 5, EntityType: "scene", EntityID: 1, Field: "title", AIValue: "New Title", Status: models.SuggestionStatusPending,
	}, nil)
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).Return(&models.Scene{ID: 1}, nil)
	db.AIAudit.On("UpdateStatus", mock.Anything, int64(5), models.SuggestionStatusAccepted).Return(nil)

	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	applied, err := GetInstance().AIAuditApplyAll(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	db.AIAudit.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}

func TestAIAuditRejectAll(t *testing.T) {
	db := mocks.NewDatabase()
	db.AIAudit.On("UpdateStatusByStatus", mock.Anything, models.SuggestionStatusPending, models.SuggestionStatusRejected).Return(int64(3), nil)
	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	rejected, err := GetInstance().AIAuditRejectAll(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, rejected)
	db.AIAudit.AssertExpectations(t)
}

func TestAITranslationApplyAll(t *testing.T) {
	db := mocks.NewDatabase()
	db.AITranslation.On("FindByStatus", mock.Anything, models.SuggestionStatusPending).Return([]*models.AITranslation{
		{ID: 3, EntityType: "scene", EntityID: 1, Field: "details", TranslatedText: "Übersetzt", Status: models.SuggestionStatusPending},
	}, nil)
	db.AITranslation.On("FindByID", mock.Anything, int64(3)).Return(&models.AITranslation{
		ID: 3, EntityType: "scene", EntityID: 1, Field: "details", TranslatedText: "Übersetzt", Status: models.SuggestionStatusPending,
	}, nil)
	db.Scene.On("UpdatePartial", mock.Anything, 1, mock.Anything).Return(&models.Scene{ID: 1}, nil)
	db.AITranslation.On("UpdateStatus", mock.Anything, int64(3), models.SuggestionStatusAccepted).Return(nil)

	initClusterInstance(db)
	instance.Config.SetAIEnabled(true)

	applied, err := GetInstance().AITranslationApplyAll(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	db.AITranslation.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}

func TestReviewRejectAll(t *testing.T) {
	cases := []struct {
		name  string
		store string
		run   func() (int, error)
		setup func(db *mocks.Database)
	}{
		{
			name: "translation",
			run: func() (int, error) {
				return GetInstance().AITranslationRejectAll(context.Background())
			},
			setup: func(db *mocks.Database) {
				db.AITranslation.On("UpdateStatusByStatus", mock.Anything, models.SuggestionStatusPending, models.SuggestionStatusRejected).Return(int64(2), nil)
			},
		},
		{
			name: "merge suggestions",
			run: func() (int, error) {
				return GetInstance().AIPerformerSuggestionRejectAll(context.Background())
			},
			setup: func(db *mocks.Database) {
				db.AIPerformerSuggestion.On("UpdateStatusByStatus", mock.Anything, models.SuggestionStatusPending, models.SuggestionStatusRejected).Return(int64(2), nil)
			},
		},
		{
			name: "candidates",
			run: func() (int, error) {
				return GetInstance().AIPerformerCandidateRejectAll(context.Background())
			},
			setup: func(db *mocks.Database) {
				db.AIPerformerCandidate.On("UpdateStatusByStatus", mock.Anything, models.SuggestionStatusPending, models.SuggestionStatusRejected).Return(int64(2), nil)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := mocks.NewDatabase()
			tc.setup(db)
			initClusterInstance(db)
			instance.Config.SetAIEnabled(true)

			rejected, err := tc.run()
			require.NoError(t, err)
			assert.Equal(t, 2, rejected)
			db.AssertExpectations(t)
		})
	}
}
