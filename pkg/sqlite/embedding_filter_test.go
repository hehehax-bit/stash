//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSceneQueryHasEmbedding(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.Embedding.Set(ctx, "scene", sceneIDs[sceneIdxWithMarkers], "test-model", []float32{0.1, 0.2}))

		hasEmbedding := "true"
		sceneFilter := models.SceneFilterType{
			HasEmbedding: &hasEmbedding,
			ID: &models.IntCriterionInput{
				Value:    sceneIDs[sceneIdxWithMarkers],
				Modifier: models.CriterionModifierEquals,
			},
		}

		scenes := queryScene(ctx, t, db.Scene, &sceneFilter, &models.FindFilterType{})
		assert.Len(t, scenes, 1)
		assert.Equal(t, sceneIDs[sceneIdxWithMarkers], scenes[0].ID)

		hasEmbedding = "false"
		scenes = queryScene(ctx, t, db.Scene, &sceneFilter, &models.FindFilterType{})
		assert.Len(t, scenes, 0)

		return nil
	})
}

func TestImageQueryHasEmbedding(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.Embedding.Set(ctx, "image", imageIDs[imageIdxWithTag], "test-model", []float32{0.1, 0.2}))

		hasEmbedding := "true"
		imageFilter := models.ImageFilterType{
			HasEmbedding: &hasEmbedding,
			ID: &models.IntCriterionInput{
				Value:    imageIDs[imageIdxWithTag],
				Modifier: models.CriterionModifierEquals,
			},
		}

		images := queryImages(ctx, t, db.Image, &imageFilter, &models.FindFilterType{})
		assert.Len(t, images, 1)
		assert.Equal(t, imageIDs[imageIdxWithTag], images[0].ID)

		hasEmbedding = "false"
		images = queryImages(ctx, t, db.Image, &imageFilter, &models.FindFilterType{})
		assert.Len(t, images, 0)

		return nil
	})
}

func TestPerformerQueryHasEmbedding(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.Embedding.Set(ctx, "performer", performerIDs[performerIdxWithGallery], "test-model", []float32{0.1, 0.2}))

		hasEmbedding := "true"
		name := getPerformerStringValue(performerIdxWithGallery, "Name")
		performerFilter := models.PerformerFilterType{
			HasEmbedding: &hasEmbedding,
			Name: &models.StringCriterionInput{
				Value:    name,
				Modifier: models.CriterionModifierEquals,
			},
		}

		performers := queryPerformers(ctx, t, &performerFilter, &models.FindFilterType{})
		assert.Len(t, performers, 1)
		assert.Equal(t, performerIDs[performerIdxWithGallery], performers[0].ID)

		hasEmbedding = "false"
		performers = queryPerformers(ctx, t, &performerFilter, &models.FindFilterType{})
		assert.Len(t, performers, 0)

		return nil
	})
}

func TestEmbeddingStoreHasEmbeddingAndCounts(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.Embedding.Set(ctx, "scene", sceneIDs[sceneIdxWithMarkers], "m1", []float32{0.1, 0.2}))
		require.NoError(t, db.Embedding.Set(ctx, "scene", sceneIDs[sceneIdxWithGallery], "m1", []float32{0.3, 0.4}))
		require.NoError(t, db.Embedding.Set(ctx, "scene", sceneIDs[sceneIdxWithGallery], "m2", []float32{0.5, 0.6}))
		require.NoError(t, db.Embedding.Set(ctx, "image", imageIDs[imageIdxWithTag], "m1", []float32{0.7, 0.8}))

		has, err := db.Embedding.HasEmbedding(ctx, "scene", sceneIDs[sceneIdxWithMarkers])
		require.NoError(t, err)
		assert.True(t, has)

		// scene with gallery has embeddings under two models
		has, err = db.Embedding.HasEmbedding(ctx, "scene", sceneIDs[sceneIdxWithGallery])
		require.NoError(t, err)
		assert.True(t, has)

		has, err = db.Embedding.HasEmbedding(ctx, "scene", sceneIDs[sceneIdxWithGroup])
		require.NoError(t, err)
		assert.False(t, has)

		counts, err := db.Embedding.CountByEntityType(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, counts["scene"])
		assert.Equal(t, 1, counts["image"])
		assert.NotContains(t, counts, "performer")

		return nil
	})
}

func TestAISceneAudioSearchByTranscript(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.AISceneAudio.Upsert(ctx, &models.AISceneAudio{
			SceneID:    sceneIDs[sceneIdxWithMarkers],
			HasAudio:   true,
			Transcript: "The performer talks about the scene at length.",
		}))
		require.NoError(t, db.AISceneAudio.Upsert(ctx, &models.AISceneAudio{
			SceneID:    sceneIDs[sceneIdxWithGroup],
			HasAudio:   true,
			Transcript: "Only music plays here.",
		}))

		results, err := db.AISceneAudio.SearchByTranscript(ctx, "talks about", 10)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, sceneIDs[sceneIdxWithMarkers], results[0].SceneID)

		results, err = db.AISceneAudio.SearchByTranscript(ctx, "no such text", 10)
		require.NoError(t, err)
		assert.Empty(t, results)

		results, err = db.AISceneAudio.SearchByTranscript(ctx, "music", 1)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, sceneIDs[sceneIdxWithGroup], results[0].SceneID)

		return nil
	})
}

func TestAIPerformerSuggestionStore(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.AIPerformerSuggestion.Create(ctx, &models.AIPerformerSuggestion{
			SourcePerformerID: performerIDs[performerIdxWithScene],
			TargetPerformerID: performerIDs[performerIdx1WithScene],
			Confidence:        0.92,
		}))

		// upsert on the same pair refreshes confidence
		require.NoError(t, db.AIPerformerSuggestion.Create(ctx, &models.AIPerformerSuggestion{
			SourcePerformerID: performerIDs[performerIdxWithScene],
			TargetPerformerID: performerIDs[performerIdx1WithScene],
			Confidence:        0.95,
		}))

		pair, err := db.AIPerformerSuggestion.FindPair(ctx, performerIDs[performerIdxWithScene], performerIDs[performerIdx1WithScene])
		require.NoError(t, err)
		require.NotNil(t, pair)
		assert.Equal(t, models.SuggestionStatusPending, pair.Status)
		assert.InDelta(t, 0.95, pair.Confidence, 0.0001)

		require.NoError(t, db.AIPerformerSuggestion.Create(ctx, &models.AIPerformerSuggestion{
			SourcePerformerID: performerIDs[performerIdx2WithScene],
			TargetPerformerID: performerIDs[performerIdx1WithScene],
			Confidence:        0.88,
		}))

		pending, err := db.AIPerformerSuggestion.FindByStatus(ctx, models.SuggestionStatusPending)
		require.NoError(t, err)
		require.Len(t, pending, 2)

		require.NoError(t, db.AIPerformerSuggestion.UpdateStatus(ctx, pair.ID, models.SuggestionStatusRejected))
		byID, err := db.AIPerformerSuggestion.FindByID(ctx, pair.ID)
		require.NoError(t, err)
		assert.Equal(t, models.SuggestionStatusRejected, byID.Status)

		require.NoError(t, db.AIPerformerSuggestion.DeleteByPerformerID(ctx, performerIDs[performerIdx1WithScene]))
		remaining, err := db.AIPerformerSuggestion.FindByStatus(ctx, models.SuggestionStatusPending)
		require.NoError(t, err)
		assert.Empty(t, remaining)

		return nil
	})
}

func TestAIAuditStore(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.AIAudit.Create(ctx, &models.AIAudit{
			EntityType: "scene", EntityID: sceneIDs[sceneIdxWithMarkers], Field: "title",
			AIValue: "New Title",
		}))

		// creating another pending finding for the same entity/field replaces it
		require.NoError(t, db.AIAudit.Create(ctx, &models.AIAudit{
			EntityType: "scene", EntityID: sceneIDs[sceneIdxWithMarkers], Field: "title",
			AIValue: "Better Title",
		}))

		pending, err := db.AIAudit.FindByStatus(ctx, models.SuggestionStatusPending)
		require.NoError(t, err)
		require.Len(t, pending, 1)
		assert.Equal(t, "Better Title", pending[0].AIValue)

		require.NoError(t, db.AIAudit.Create(ctx, &models.AIAudit{
			EntityType: "image", EntityID: imageIDs[imageIdxWithTag], Field: "performers",
			AIValue: "Jane Doe",
		}))

		pending, err = db.AIAudit.FindByStatus(ctx, models.SuggestionStatusPending)
		require.NoError(t, err)
		require.Len(t, pending, 2)

		require.NoError(t, db.AIAudit.UpdateStatus(ctx, pending[0].ID, models.SuggestionStatusAccepted))
		byID, err := db.AIAudit.FindByID(ctx, pending[0].ID)
		require.NoError(t, err)
		assert.Equal(t, models.SuggestionStatusAccepted, byID.Status)

		return nil
	})
}

func TestAITranslationStore(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.AITranslation.Create(ctx, &models.AITranslation{
			EntityType: "scene", EntityID: sceneIDs[sceneIdxWithMarkers], Language: "de", Field: "title",
			TranslatedText: "Neuer Titel",
		}))

		// upsert on the same entity/language/field
		require.NoError(t, db.AITranslation.Create(ctx, &models.AITranslation{
			EntityType: "scene", EntityID: sceneIDs[sceneIdxWithMarkers], Language: "de", Field: "title",
			TranslatedText: "Besserer Titel",
		}))

		byEntity, err := db.AITranslation.FindByEntity(ctx, "scene", sceneIDs[sceneIdxWithMarkers], "de")
		require.NoError(t, err)
		require.NotNil(t, byEntity)
		assert.Equal(t, "Besserer Titel", byEntity.TranslatedText)
		assert.Equal(t, models.SuggestionStatusPending, byEntity.Status)

		require.NoError(t, db.AITranslation.Create(ctx, &models.AITranslation{
			EntityType: "image", EntityID: imageIDs[imageIdxWithTag], Language: "de", Field: "details",
			TranslatedText: "Einzelheiten",
		}))

		pending, err := db.AITranslation.FindByStatus(ctx, models.SuggestionStatusPending)
		require.NoError(t, err)
		require.Len(t, pending, 2)

		require.NoError(t, db.AITranslation.UpdateStatus(ctx, byEntity.ID, models.SuggestionStatusAccepted))
		byID, err := db.AITranslation.FindByID(ctx, byEntity.ID)
		require.NoError(t, err)
		assert.Equal(t, models.SuggestionStatusAccepted, byID.Status)

		return nil
	})
}

func TestEmbeddingFindStale(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		require.NoError(t, db.Embedding.Set(ctx, "scene", sceneIDs[sceneIdxWithMarkers], "m1", []float32{0.1, 0.2}))

		// no metadata change yet: not stale
		stale, err := db.Embedding.FindStale(ctx, "scene", "m1")
		require.NoError(t, err)
		assert.Empty(t, stale)

		// touch the scene's updated_at via its store, waiting so the stored
		// updated_at (second precision) is strictly newer than the embedding
		time.Sleep(1100 * time.Millisecond)
		partial := models.NewScenePartial()
		partial.Details = models.NewOptionalString("audit touch")
		_, err = db.Scene.UpdatePartial(ctx, sceneIDs[sceneIdxWithMarkers], partial)
		require.NoError(t, err)

		stale, err = db.Embedding.FindStale(ctx, "scene", "m1")
		require.NoError(t, err)
		assert.Equal(t, []int{sceneIDs[sceneIdxWithMarkers]}, stale)

		// unknown entity type returns nothing
		stale, err = db.Embedding.FindStale(ctx, "bogus", "m1")
		require.NoError(t, err)
		assert.Empty(t, stale)

		return nil
	})
}

func TestSceneMarkerFindByPerformerIDs(t *testing.T) {
	withRollbackTxn(func(ctx context.Context) error {
		// create a marker and attribute it to a performer
		marker := models.SceneMarker{
			SceneID:      sceneIDs[sceneIdxWithMarkers],
			PrimaryTagID: tagIDs[tagIdxWithPrimaryMarkers],
			EndSeconds:   getMarkerEndSeconds(len(markerIDs) + 1),
		}
		require.NoError(t, db.SceneMarker.Create(ctx, &marker))

		pid := performerIDs[performerIdxWithScene]
		require.NoError(t, db.SceneMarker.UpdatePerformers(ctx, marker.ID, []int{pid}))

		found, err := db.SceneMarker.FindByPerformerIDs(ctx, pid, 10)
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, marker.ID, found[0].ID)

		// a performer with no markers returns nothing
		found, err = db.SceneMarker.FindByPerformerIDs(ctx, pid, 10)
		require.NoError(t, err)
		require.Len(t, found, 1)

		other, err := db.SceneMarker.FindByPerformerIDs(ctx, 999999, 10)
		require.NoError(t, err)
		assert.Empty(t, other)

		return nil
	})
}
