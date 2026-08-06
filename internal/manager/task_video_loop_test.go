package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHammingDistance(t *testing.T) {
	assert.Equal(t, 0, hammingDistance(0, 0))
	assert.Equal(t, 1, hammingDistance(1, 0))
	assert.Equal(t, 16, hammingDistance(0xFF00, 0x00FF))
	assert.Equal(t, 64, hammingDistance(0, ^uint64(0)))
}

var (
	loopH1 = uint64(0xFFFF0000FFFF0000)
	loopH2 = uint64(0x0000FFFF0000FFFF)
	loopH3 = uint64(0xF0F0F0F0F0F0F0F0)
	loopH4 = uint64(0x0F0F0F0F0F0F0F0F)
	loopH5 = uint64(0xAAAAAAAAAAAAAAAA)
	loopH6 = uint64(0x5555555555555555)
	loopH7 = uint64(0x1234567890ABCDEF)
	loopH8 = uint64(0xCAFEBABEDEADBEEF)
)

func TestLoopPeriodFromHashes_InternalPeriod(t *testing.T) {
	// 8 frames where the first 4 repeat exactly: period n/2
	hashes := []uint64{loopH1, loopH2, loopH3, loopH4, loopH1, loopH2, loopH3, loopH4}
	period, ok := loopPeriodFromHashes(hashes, 20)
	assert.True(t, ok)
	assert.Equal(t, 4, period)
}

func TestLoopPeriodFromHashes_QuarterPeriod(t *testing.T) {
	// 8 frames repeating every 2: candidates n/3=2 and n/4=2
	hashes := []uint64{loopH1, loopH2, loopH1, loopH2, loopH1, loopH2, loopH1, loopH2}
	period, ok := loopPeriodFromHashes(hashes, 20)
	assert.True(t, ok)
	// a period-2 sequence also satisfies the n/2 candidate (4), which is checked first
	assert.Equal(t, 4, period)
}

func TestLoopPeriodFromHashes_BoundaryLoop(t *testing.T) {
	// start and end identical, and the content just before the end continues
	// into the content just after the start (wrapped continuity): a genuine loop
	hashes := []uint64{
		0x0000000000000000, // start
		0x0000000000000003,
		0x000000000000000F,
		0x000000000000003F,
		0x00000000000000FF,
		0x00000000000003FF,
		0x0000000000000FFF, // just before the end
		0x0000000000000000, // end == start
	}
	period, ok := loopPeriodFromHashes(hashes, 20)
	assert.True(t, ok)
	assert.Equal(t, 7, period)
}

func TestLoopPeriodFromHashes_FadeToBlackRejected(t *testing.T) {
	// start and end are the same solid (black) frame, but the content in
	// between is unrelated: a fade-to-black video, not a loop. The seam matches
	// but wrapped continuity does not.
	hashes := []uint64{loopH1, loopH2, loopH3, loopH4, loopH5, loopH6, loopH7, loopH1}
	_, ok := loopPeriodFromHashes(hashes, 20)
	assert.False(t, ok)
}

func TestLoopPeriodFromHashes_BlackFramesRejected(t *testing.T) {
	// real-world false positives: the first and last frames are identical
	// solid frames (fades in/out) while the content between them moves.
	// The pattern below mirrors the hashes observed from such videos.
	cases := [][]uint64{
		{
			0x8000000000000000,
			0xc632190806bddbfd,
			0xe7b319649a6c9a68,
			0xa2b9eaf14c3493d2,
			0xb9d2569485799723,
			0xf09201d407e7f0bf,
			0x90d958c6e4e9c7c6,
			0x8000000000000000,
		},
		{
			0xaa55aa55aa55aa55,
			0xde6d7b00a4c5e0da,
			0x86616fa456327a6b,
			0xf6d1dd0293b0a7e0,
			0xe69d185ce1e6a943,
			0xcb0fc1b83666f4a4,
			0xcf4bb42ee124e721,
			0xaa55aa55aa55aa55,
		},
		{
			0x0000000000000000,
			0x805c007f1f6b3f6a,
			0xb386d75966c93192,
			0x95e708f81c8f5be0,
			0x82e07ccb636c6b69,
			0xb0810f6e3c776770,
			0x84f0778733618abb,
			0x0000000000000000,
		},
	}
	for i, hashes := range cases {
		_, ok := loopPeriodFromHashes(hashes, 20)
		assert.False(t, ok, "fade-to-black video %d must not be flagged as looping", i)
	}
}

func TestLoopPeriodFromHashes_NoLoop(t *testing.T) {
	hashes := []uint64{loopH1, loopH2, loopH3, loopH4, loopH5, loopH6, loopH7, loopH8}
	_, ok := loopPeriodFromHashes(hashes, 20)
	assert.False(t, ok)
}

func TestLoopPeriodFromHashes_StaticVideo(t *testing.T) {
	// all frames identical: no motion, must not be flagged as looping
	hashes := []uint64{loopH1, loopH1, loopH1, loopH1, loopH1, loopH1, loopH1, loopH1}
	_, ok := loopPeriodFromHashes(hashes, 20)
	assert.False(t, ok)
}

func TestLoopPeriodFromHashes_TooFewFrames(t *testing.T) {
	_, ok := loopPeriodFromHashes([]uint64{1, 2, 3}, 20)
	assert.False(t, ok)
}

func TestDetectLoopingJob_ResolvesOrCreatesTag(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Looping", true).Return(&models.Tag{ID: 42, Name: "Looping"}, nil)
	db.Scene.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(models.NewSceneQueryResult(db.Scene), nil)
	db.Scene.On("FindMany", mock.Anything, mock.Anything).Return([]*models.Scene{}, nil)
	initClusterInstance(db)

	j := CreateDetectLoopingJob(DetectLoopingInput{})
	err := j.Execute(context.Background(), &job.Progress{})

	require.NoError(t, err)
	db.Tag.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}

func TestDetectLoopingJob_CreatesMissingTag(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Looping", true).Return(nil, nil)
	db.Tag.On("Create", mock.Anything, mock.MatchedBy(func(input *models.CreateTagInput) bool {
		return input.Tag != nil && input.Tag.Name == "Looping"
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.CreateTagInput).Tag.ID = 42
	}).Return(nil)
	db.Scene.On("QueryCount", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(models.NewSceneQueryResult(db.Scene), nil)
	db.Scene.On("FindMany", mock.Anything, mock.Anything).Return([]*models.Scene{}, nil)
	initClusterInstance(db)

	j := CreateDetectLoopingJob(DetectLoopingInput{})
	err := j.Execute(context.Background(), &job.Progress{})

	require.NoError(t, err)
	db.Tag.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}

func TestDetectLoopingJob_TagLookupError(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Looping", true).Return(nil, errors.New("db error"))
	initClusterInstance(db)

	j := CreateDetectLoopingJob(DetectLoopingInput{})
	err := j.Execute(context.Background(), &job.Progress{})

	require.Error(t, err)
}

func TestDetectLoopingJob_SceneIDsOnly(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Looping", true).Return(&models.Tag{ID: 42, Name: "Looping"}, nil)
	db.Scene.On("FindMany", mock.Anything, []int{7, 9}).Return([]*models.Scene{{ID: 7}, {ID: 9}}, nil)
	initClusterInstance(db)

	j := CreateDetectLoopingJob(DetectLoopingInput{SceneIDs: []int{7, 9}})
	err := j.Execute(context.Background(), &job.Progress{})

	// the scenes have no files, so every scene fails; the job must surface that
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed for all 2 scenes")
	db.Scene.AssertNotCalled(t, "QueryCount", mock.Anything, mock.Anything, mock.Anything)
	db.Scene.AssertNotCalled(t, "Query", mock.Anything, mock.Anything)
	db.Tag.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}

func TestDetectLoopingJob_AllScenesFailed(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Looping", true).Return(&models.Tag{ID: 42, Name: "Looping"}, nil)
	db.Scene.On("FindMany", mock.Anything, []int{7}).Return([]*models.Scene{{ID: 7}}, nil)
	initClusterInstance(db)

	j := CreateDetectLoopingJob(DetectLoopingInput{SceneIDs: []int{7}})
	err := j.Execute(context.Background(), &job.Progress{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "scene has no file")
}

func TestLoopPeriodFromHashes_SeamlessSlowLoop(t *testing.T) {
	// a slow, visually near-static loop: all pairwise distances are small but
	// non-zero, and the end frame matches the first frame (the seam)
	hashes := []uint64{
		0x0000000000000000,
		0x0000000000000003,
		0x000000000000000F,
		0x000000000000003F,
		0x00000000000000FF,
		0x00000000000003FF,
		0x0000000000000FFF,
		0x0000000000000000,
	}
	period, ok := loopPeriodFromHashes(hashes, 20)
	assert.True(t, ok, "a seamless slow loop must be detected")
	assert.Equal(t, 7, period)
}

func TestLoopPeriodFromHashes_NearStaticLoop(t *testing.T) {
	// a near-static loop (as found in real libraries): every sampled frame is
	// within a few bits of every other, and the seam matches. This must be
	// flagged as looping — only fully static stills are excluded.
	hashes := []uint64{
		0x0000000000000000,
		0x0000000000000001,
		0x0000000000000000,
		0x0000000000000002,
		0x0000000000000000,
		0x0000000000000001,
		0x0000000000000000,
		0x0000000000000001,
	}
	period, ok := loopPeriodFromHashes(hashes, 20)
	assert.True(t, ok, "a near-static video with a matching seam must be detected")
	assert.Equal(t, 7, period)
}

func TestDetectLoopingJob_SkipsTaggedScenes(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "Looping", true).Return(&models.Tag{ID: 42, Name: "Looping"}, nil)
	db.Scene.On("QueryCount", mock.Anything, mock.MatchedBy(func(f *models.SceneFilterType) bool {
		return f != nil && f.Tags != nil && f.Tags.Modifier == models.CriterionModifierExcludes
	}), mock.Anything).Return(0, nil)
	db.Scene.On("Query", mock.Anything, mock.Anything).Return(models.NewSceneQueryResult(db.Scene), nil)
	db.Scene.On("FindMany", mock.Anything, mock.Anything).Return([]*models.Scene{}, nil)
	initClusterInstance(db)

	j := CreateDetectLoopingJob(DetectLoopingInput{})
	err := j.Execute(context.Background(), &job.Progress{})

	require.NoError(t, err)
	db.Tag.AssertExpectations(t)
	db.Scene.AssertExpectations(t)
}
