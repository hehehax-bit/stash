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

func TestGatherLibraryStats_IncludesImages(t *testing.T) {
	db := mocks.NewDatabase()

	db.Scene.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)
	db.Image.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)

	scene := &models.Scene{ID: 1}
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(mocks.SceneQueryResult([]*models.Scene{scene}, 1), nil)
	db.Scene.On("GetTagIDs", mock.Anything, 1).Return([]int{100}, nil)
	db.Scene.On("GetPerformerIDs", mock.Anything, 1).Return([]int{50}, nil)

	img := &models.Image{ID: 2}
	db.Image.On("Query", mock.Anything, mock.Anything).Return(mocks.ImageQueryResult([]*models.Image{img}, 1), nil)
	db.Image.On("GetTagIDs", mock.Anything, 2).Return([]int{200}, nil)
	db.Image.On("GetPerformerIDs", mock.Anything, 2).Return([]int{51}, nil)

	j := &AISmartCollectionsJob{}
	sceneTagCounts, imageTagCounts, studioCounts, performerCounts, err := j.gatherLibraryStats(context.Background(), db.Repository(), 0, 0)

	require.NoError(t, err)
	assert.Equal(t, 1, sceneTagCounts[100])
	assert.Equal(t, 1, imageTagCounts[200])
	assert.Equal(t, 1, performerCounts[50])
	assert.Equal(t, 1, performerCounts[51])
	assert.Empty(t, studioCounts)
	db.Scene.AssertExpectations(t)
	db.Image.AssertExpectations(t)
}

func TestGatherLibraryStats_RespectsLimits(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(100, nil)
	db.Image.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(100, nil)

	var sceneOptions models.SceneQueryOptions
	db.Scene.On("Query", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			sceneOptions = args.Get(1).(models.SceneQueryOptions)
		}).
		Return(mocks.SceneQueryResult(nil, 0), nil)

	var imageOptions models.ImageQueryOptions
	db.Image.On("Query", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			imageOptions = args.Get(1).(models.ImageQueryOptions)
		}).
		Return(mocks.ImageQueryResult(nil, 0), nil)

	j := &AISmartCollectionsJob{}
	_, _, _, _, err := j.gatherLibraryStats(context.Background(), db.Repository(), 5, 7)

	require.NoError(t, err)
	require.NotNil(t, sceneOptions.FindFilter)
	assert.Equal(t, 5, sceneOptions.FindFilter.GetPageSize())
	require.NotNil(t, imageOptions.FindFilter)
	assert.Equal(t, 7, imageOptions.FindFilter.GetPageSize())
}

func TestCreateGroup_UsesGetAllQuery(t *testing.T) {
	db := mocks.NewDatabase()

	db.Group.On("FindByName", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	db.Group.On("Create", mock.Anything, mock.Anything).Return(nil)
	db.Group.On("UpdatePartial", mock.Anything, mock.Anything, mock.Anything).Return(&models.Group{}, nil)

	var sceneOptions models.SceneQueryOptions
	db.Scene.On("Query", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			sceneOptions = args.Get(1).(models.SceneQueryOptions)
		}).
		Return(mocks.SceneQueryResult(nil, 0), nil)

	j := &AISmartCollectionsJob{}
	err := j.createGroup(context.Background(), db.Repository(), "[AI] Test", "A test group", []int{1, 2}, nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, sceneOptions.FindFilter)
	assert.True(t, sceneOptions.FindFilter.IsGetAll(), "group member query must request all scenes, not limit 0")
	db.Group.AssertExpectations(t)
}

func TestCreateGallery_UsesGetAllQuery(t *testing.T) {
	db := mocks.NewDatabase()

	db.Gallery.On("FindUserGalleryByTitle", mock.Anything, mock.Anything).Return(nil, nil)
	db.Gallery.On("Create", mock.Anything, mock.Anything).Return(nil)
	db.Gallery.On("UpdatePartial", mock.Anything, mock.Anything, mock.Anything).Return(&models.Gallery{}, nil)

	var imageOptions models.ImageQueryOptions
	db.Image.On("Query", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			imageOptions = args.Get(1).(models.ImageQueryOptions)
		}).
		Return(mocks.ImageQueryResult(nil, 0), nil)

	j := &AISmartCollectionsJob{}
	err := j.createGallery(context.Background(), db.Repository(), "[AI] Test", "A test gallery", []int{1, 2}, nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, imageOptions.FindFilter)
	assert.True(t, imageOptions.FindFilter.IsGetAll(), "gallery member query must request all images, not limit 0")
	db.Gallery.AssertExpectations(t)
}

func TestClusterCount(t *testing.T) {
	assert.Equal(t, 0, clusterCount(9))
	assert.Equal(t, 2, clusterCount(10))
	assert.Equal(t, 2, clusterCount(100))
	assert.Equal(t, 4, clusterCount(200))
	assert.Equal(t, 8, clusterCount(10000))
}

func TestKMeansEmbeddings_SeparatesClusters(t *testing.T) {
	embeddings := make(map[int][]float32)
	// three vectors near the origin
	for i := 1; i <= 3; i++ {
		embeddings[i] = []float32{0.0, 0.0}
	}
	// three vectors near (1,1)
	for i := 4; i <= 6; i++ {
		embeddings[i] = []float32{1.0, 1.0}
	}

	groups := kMeansEmbeddings(embeddings, 2)
	require.Len(t, groups, 2)

	var sizes []int
	for _, g := range groups {
		sizes = append(sizes, len(g))
	}
	assert.ElementsMatch(t, []int{3, 3}, sizes)
}

func TestKMeansEmbeddings_ReturnsNilForTooFew(t *testing.T) {
	embeddings := map[int][]float32{
		1: {0.0},
		2: {0.5},
	}
	assert.Nil(t, kMeansEmbeddings(embeddings, 5))
}

func TestBuildClusterLines(t *testing.T) {
	clusters := []semanticCluster{
		{
			EntityType: "scene",
			Size:       42,
			Tags:       []libraryTag{{Name: "Solo", Count: 35}, {Name: "Outdoor", Count: 28}},
			Performers: []libraryPerformer{{Name: "Alice", Count: 12}},
			Studios:    []libraryStudio{{Name: "StudioX", Count: 9}},
			Titles:     []string{"Sunny Day", "Beach Walk"},
		},
		{
			EntityType: "image",
			Size:       10,
			Tags:       []libraryTag{{Name: "Closeup", Count: 8}},
		},
	}

	lines := buildClusterLines(clusters)
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "scene cluster with 42 entities")
	assert.Contains(t, lines[0], "Solo (35)")
	assert.Contains(t, lines[0], "Alice (12)")
	assert.Contains(t, lines[0], "StudioX (9)")
	assert.Contains(t, lines[0], "Beach Walk")
	assert.Contains(t, lines[1], "image cluster with 10 entities")
	assert.Contains(t, lines[1], "Closeup (8)")
}

func TestSceneIDsForCollection_UnionsCriteria(t *testing.T) {
	db := mocks.NewDatabase()

	// tag query matches scenes 1,2; performer query matches 2,3; studio query matches 4
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(
		mocks.SceneQueryResult([]*models.Scene{{ID: 1}, {ID: 2}}, 2), nil,
	).Once()
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(
		mocks.SceneQueryResult([]*models.Scene{{ID: 2}, {ID: 3}}, 2), nil,
	).Once()
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(
		mocks.SceneQueryResult([]*models.Scene{{ID: 4}}, 1), nil,
	).Once()

	ids := sceneIDsForCollection(context.Background(), db.Repository(), []int{10}, []int{20}, []int{30})
	assert.ElementsMatch(t, []int{1, 2, 3, 4}, ids)
	db.Scene.AssertExpectations(t)
}

func TestBuildPrompts_IncludesPerformersAndClusters(t *testing.T) {
	j := &AISmartCollectionsJob{}
	_, userPrompt := j.buildPrompts(
		AIOutputTypeGroups,
		"Solo (35)",
		"",
		"StudioX (9)",
		"Alice (12)",
		"- scene cluster with 42 entities; tags: Solo (35)",
		10,
	)

	assert.Contains(t, userPrompt, "Alice (12)")
	assert.Contains(t, userPrompt, "scene cluster with 42 entities")
	assert.Contains(t, userPrompt, "performers")
}

func TestJaccardDistance(t *testing.T) {
	a := map[int]bool{1: true, 2: true}
	b := map[int]bool{2: true, 3: true}
	// intersection {2}, union {1,2,3} -> 1 - 1/3
	assert.InDelta(t, 2.0/3.0, jaccardDistance(a, b), 0.0001)

	assert.Equal(t, 0.0, jaccardDistance(a, a))
	assert.Equal(t, 0.0, jaccardDistance(nil, nil))
	assert.Equal(t, 1.0, jaccardDistance(map[int]bool{1: true}, map[int]bool{2: true}))
}

func TestHybridEntityDistance(t *testing.T) {
	vecA := []float32{1, 0}
	vecB := []float32{0, 1}
	featA := map[int]bool{1: true, 2: true}
	featB := map[int]bool{1: true, 2: true}

	// same features: only the embedding distance contributes (2 for unit vectors)
	d := hybridEntityDistance(vecA, vecB, featA, featB, 2, 0.5)
	assert.InDelta(t, 2.0, d, 0.0001)

	// disjoint features add lambda * 1
	disjoint := map[int]bool{9: true}
	d = hybridEntityDistance(vecA, vecB, featA, disjoint, 2, 0.5)
	assert.InDelta(t, 2.5, d, 0.0001)

	// identical vectors with disjoint features: distance is lambda
	d = hybridEntityDistance(vecA, vecA, featA, disjoint, 2, 0.5)
	assert.InDelta(t, 0.5, d, 0.0001)
}

func TestKMeansHybridEmbeddings_MetadataPullsTogether(t *testing.T) {
	// All vectors are identical, so embeddings alone cannot separate them;
	// only the metadata feature sets can drive the grouping.
	embeddings := map[int][]float32{
		1: {0.5, 0.5},
		2: {0.5, 0.5},
		3: {0.5, 0.5},
		4: {0.5, 0.5},
	}
	features := map[int]map[int]bool{
		1: {10: true},
		2: {20: true},
		3: {20: true},
		4: {20: true},
	}

	groups := kMeansHybridEmbeddings(embeddings, 2, features, 0.5)
	require.Len(t, groups, 2)

	var groupWithOne []int
	for _, g := range groups {
		if containsInt(g, 1) {
			groupWithOne = g
		}
	}
	assert.Len(t, groupWithOne, 1, "entity 1 has unique metadata and must form its own cluster")

	// entities 2-4 share metadata and must be grouped together
	var groupB []int
	for _, g := range groups {
		if containsInt(g, 2) {
			groupB = g
		}
	}
	assert.ElementsMatch(t, []int{2, 3, 4}, groupB)
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestClusterIDsForCollection(t *testing.T) {
	clusters := []semanticCluster{
		{EntityType: "scene", IDs: []int{1, 2, 3}},
		{EntityType: "image", IDs: []int{4, 5}},
	}

	sceneIDs, imageIDs := clusterIDsForCollection(clusters, nil)
	assert.Nil(t, sceneIDs)
	assert.Nil(t, imageIDs)

	sceneIDs, imageIDs = clusterIDsForCollection(clusters, newInt(1))
	assert.ElementsMatch(t, []int{1, 2, 3}, sceneIDs)
	assert.Nil(t, imageIDs)

	sceneIDs, imageIDs = clusterIDsForCollection(clusters, newInt(2))
	assert.Nil(t, sceneIDs)
	assert.ElementsMatch(t, []int{4, 5}, imageIDs)

	sceneIDs, imageIDs = clusterIDsForCollection(clusters, newInt(0))
	assert.Nil(t, sceneIDs)
	assert.Nil(t, imageIDs)

	sceneIDs, imageIDs = clusterIDsForCollection(clusters, newInt(3))
	assert.Nil(t, sceneIDs)
	assert.Nil(t, imageIDs)
}

func TestBuildClusterLines_Numbered(t *testing.T) {
	clusters := []semanticCluster{
		{EntityType: "scene", Size: 42, Titles: []string{"Sunny Day (2020-06-01): filmed at the beach"}},
		{EntityType: "image", Size: 10},
	}

	lines := buildClusterLines(clusters)
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "[1] scene cluster with 42 entities")
	assert.Contains(t, lines[0], "Sunny Day (2020-06-01)")
	assert.Contains(t, lines[1], "[2] image cluster with 10 entities")
}
