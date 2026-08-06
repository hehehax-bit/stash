package manager

import (
	"context"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTopByCount_SortsByDescendingCount(t *testing.T) {
	counts := map[int]int{
		1: 3,
		2: 5,
		3: 1,
	}

	got := topByCount(counts, 5)

	require.Equal(t, []int{2, 1, 3}, got)
}

func TestTopByCount_TieBreaksByIDAscending(t *testing.T) {
	counts := map[int]int{
		5: 2,
		1: 2,
		3: 2,
	}

	got := topByCount(counts, 5)

	require.Equal(t, []int{1, 3, 5}, got)
}

func TestTopByCount_LimitsToN(t *testing.T) {
	counts := map[int]int{
		1: 10,
		2: 8,
		3: 6,
		4: 4,
	}

	got := topByCount(counts, 2)

	require.Equal(t, []int{1, 2}, got)
}

func TestTopByCount_Empty(t *testing.T) {
	assert.Nil(t, topByCount(nil, 5))
}

func sceneWithYear(id int, year int) *models.Scene {
	s := &models.Scene{ID: id}
	if year > 0 {
		date := models.Date{Time: time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)}
		s.Date = &date
	}
	return s
}

func TestGatherSceneStats_ExtractsYearsFromDayPrecisionDates(t *testing.T) {
	db := mocks.NewDatabase()
	db.Scene.On("FindByPerformerID", mock.Anything, 1).Return([]*models.Scene{
		sceneWithYear(1, 2015),
		sceneWithYear(2, 2020),
		sceneWithYear(3, 2018),
	}, nil)
	db.Scene.On("GetTagIDs", mock.Anything, mock.Anything).Return([]int{}, nil)
	db.Scene.On("GetPerformerIDs", mock.Anything, mock.Anything).Return([]int{}, nil)

	j := &AIPerformerCareerJob{}
	stats, err := j.gatherSceneStats(context.Background(), db.Repository(), 1)

	require.NoError(t, err)
	assert.Equal(t, 3, stats.SceneCount)
	assert.Equal(t, 2015, stats.MinYear)
	assert.Equal(t, 2020, stats.MaxYear)
	assert.Equal(t, []int{2015, 2018, 2020}, stats.ActiveYears)
}

func TestGatherSceneStats_CountsRelationships(t *testing.T) {
	studioID := 10
	db := mocks.NewDatabase()
	scenes := []*models.Scene{
		{ID: 1, StudioID: &studioID},
		{ID: 2, StudioID: &studioID},
		{ID: 3},
	}
	db.Scene.On("FindByPerformerID", mock.Anything, 1).Return(scenes, nil)
	db.Scene.On("GetTagIDs", mock.Anything, mock.Anything).
		Return(func(_ context.Context, id int) []int {
			switch id {
			case 1:
				return []int{100, 200}
			case 2:
				return []int{100}
			default:
				return []int{}
			}
		}, nil)
	db.Scene.On("GetPerformerIDs", mock.Anything, mock.Anything).
		Return(func(_ context.Context, id int) []int {
			switch id {
			case 1:
				return []int{1, 300}
			case 2:
				return []int{1, 400}
			default:
				return []int{1}
			}
		}, nil)

	j := &AIPerformerCareerJob{}
	stats, err := j.gatherSceneStats(context.Background(), db.Repository(), 1)

	require.NoError(t, err)
	assert.Equal(t, 2, stats.StudioCounts[10])
	assert.Equal(t, 2, stats.TagCounts[100])
	assert.Equal(t, 1, stats.TagCounts[200])
	assert.Equal(t, 1, stats.PerformerCounts[300])
	assert.Equal(t, 1, stats.PerformerCounts[400])
	_, selfCounted := stats.PerformerCounts[1]
	assert.False(t, selfCounted, "the performer themselves must not be counted as a co-star")
}

func TestAnalyzeWithAI_ValidJSON(t *testing.T) {
	server := newAIChatServer(t, `{
		"career_start": 2012,
		"career_end": null,
		"active_years": ["2012-2015", "2018-2023"],
		"primary_niches": ["Gonzo", "Anal"],
		"notable_studios": ["Studio A"],
		"career_highlights": "Long-running career.",
		"summary": "A productive performer."
	}`)
	defer server.Close()

	db := mocks.NewDatabase()
	db.Studio.On("Find", mock.Anything, mock.Anything).Return(&models.Studio{ID: 1, Name: "Studio A"}, nil)
	db.Tag.On("Find", mock.Anything, mock.Anything).Return(&models.Tag{ID: 1, Name: "Anal"}, nil)
	db.Performer.On("Find", mock.Anything, mock.Anything).Return(&models.Performer{ID: 2, Name: "Co Star"}, nil)

	j := &AIPerformerCareerJob{}
	p := &models.Performer{ID: 7, Name: "Jane Doe"}
	stats := &careerSceneStats{
		SceneCount:      5,
		MinYear:         2012,
		MaxYear:         2023,
		ActiveYears:     []int{2012, 2015, 2018, 2023},
		StudioCounts:    map[int]int{1: 3},
		TagCounts:       map[int]int{1: 4},
		PerformerCounts: map[int]int{2: 2},
	}

	profile, err := j.analyzeWithAI(context.Background(), ai.NewClient(server.URL, "test-model"), p, stats, db.Repository())

	require.NoError(t, err)
	require.NotNil(t, profile)
	require.NotNil(t, profile.CareerStart)
	assert.Equal(t, 2012, *profile.CareerStart)
	assert.Nil(t, profile.CareerEnd)
	assert.Equal(t, []string{"2012-2015", "2018-2023"}, profile.ActiveYears)
	assert.Equal(t, []string{"Gonzo", "Anal"}, profile.PrimaryNiches)
	assert.Equal(t, "A productive performer.", profile.Summary)
	assert.Equal(t, 7, profile.PerformerID)
}

func TestAnalyzeWithAI_InvalidJSON(t *testing.T) {
	server := newAIChatServer(t, "this is not json")
	defer server.Close()

	db := mocks.NewDatabase()

	j := &AIPerformerCareerJob{}
	stats := &careerSceneStats{}

	_, err := j.analyzeWithAI(context.Background(), ai.NewClient(server.URL, "test-model"), &models.Performer{ID: 1, Name: "Test"}, stats, db.Repository())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no JSON found")
}

func TestAnalyzeWithAI_MissingFields(t *testing.T) {
	server := newAIChatServer(t, `{}`)
	defer server.Close()

	db := mocks.NewDatabase()

	j := &AIPerformerCareerJob{}
	profile, err := j.analyzeWithAI(context.Background(), ai.NewClient(server.URL, "test-model"), &models.Performer{ID: 1, Name: "Test"}, &careerSceneStats{}, db.Repository())

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Nil(t, profile.CareerStart)
	assert.Nil(t, profile.CareerEnd)
}
