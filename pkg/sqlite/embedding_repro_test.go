//go:build integration
// +build integration

package sqlite_test

import (
	"context"
	"os"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	"github.com/stashapp/stash/pkg/txn"
	"github.com/stretchr/testify/require"
)

// TestEmbeddingStoreWithDatabase verifies that the embedding store (which uses
// dbWrapper directly, not explicit transactions) can be used through the
// database context provided by WithDatabase. This is the access pattern used by
// the AI embedding job and regressions here previously failed with
// "not in transaction".
func TestEmbeddingStoreWithDatabase(t *testing.T) {
	f, err := os.CreateTemp("", "*.sqlite")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())

	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(f.Name()))
	defer db.Close()

	ctx := context.Background()

	vec := make([]float32, 4)
	for i := range vec {
		vec[i] = float32(i) * 0.25
	}
	vec2 := make([]float32, 4)
	copy(vec2, vec)
	vec2[0] = 1.0

	// insert two scene embeddings
	require.NoError(t, txn.WithTxn(ctx, db, func(ctx context.Context) error {
		if err := db.Embedding.Set(ctx, "scene", 1, "m1", vec); err != nil {
			return err
		}
		return db.Embedding.Set(ctx, "scene", 2, "m1", vec2)
	}))

	// read back through the database context, mirroring how the AI embedding
	// job accesses files and embeddings outside of explicit transactions
	err = txn.WithDatabase(ctx, db, func(ctx context.Context) error {
		all, err := db.Embedding.FindByEntityType(ctx, "scene", "m1")
		require.NoError(t, err)
		require.Len(t, all, 2)

		sim, err := db.Embedding.SearchSimilar(ctx, "scene", "m1", vec, 5)
		require.NoError(t, err)
		require.Len(t, sim, 2)
		require.Equal(t, 1, sim[0].EntityID)

		dups, err := db.Embedding.FindNearDuplicates(ctx, "scene", "m1", 0.9, 2, 100)
		require.NoError(t, err)
		require.Len(t, dups, 0)
		return nil
	})
	require.NoError(t, err)

	// and verify that outside a database context the store still fails with the
	// "not in transaction" error (previously the embedding job hit this)
	_, err = db.Embedding.FindByEntityType(ctx, "scene", "m1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in transaction")
}

// TestLoadPrimaryFileWithDatabase verifies that loading an image's primary file
// works through the WithDatabase context used by the visual AI embedding job
// (loadImageFile). This previously failed with "not in transaction"/SQL errors
// when the visual embedding job ran, producing "0 embeddings generated".
func TestLoadPrimaryFileWithDatabase(t *testing.T) {
	f, err := os.CreateTemp("", "*.sqlite")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())

	db := sqlite.NewDatabase()
	require.NoError(t, db.Open(f.Name()))
	defer db.Close()

	ctx := context.Background()

	var imageID int
	var fileID models.FileID

	// create a folder, image file and image within a single transaction
	require.NoError(t, txn.WithTxn(ctx, db, func(ctx context.Context) error {
		folder := models.Folder{}
		if err := db.Folder.Create(ctx, &folder); err != nil {
			return err
		}

		file := &models.ImageFile{
			BaseFile: &models.BaseFile{
				Basename:       "test.jpg",
				ParentFolderID: folder.ID,
				Size:           10,
			},
		}
		if err := db.File.Create(ctx, file); err != nil {
			return err
		}
		fileID = file.ID

		image := &models.Image{}
		if err := db.Image.Create(ctx, &models.CreateImageInput{
			Image:   image,
			FileIDs: []models.FileID{file.ID},
		}); err != nil {
			return err
		}
		imageID = image.ID
		return nil
	}))
	require.NotZero(t, imageID)

	// now load the primary file through the WithDatabase context, mirroring the
	// visual embedding job's access pattern
	err = txn.WithDatabase(ctx, db, func(ctx context.Context) error {
		img, err := db.Image.Find(ctx, imageID)
		require.NoError(t, err)
		require.NotNil(t, img)

		require.NoError(t, img.LoadPrimaryFile(ctx, db.File))
		require.NotNil(t, img.Files.Primary())
		require.Equal(t, fileID, img.Files.Primary().Base().ID)
		return nil
	})
	require.NoError(t, err)
}
