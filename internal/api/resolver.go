package api

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
	"github.com/stashapp/stash/internal/build"
	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/plugin/hook"
	"github.com/stashapp/stash/pkg/scraper"
)

var (
	// ErrNotImplemented is an error which means the given functionality isn't implemented by the API.
	ErrNotImplemented = errors.New("not implemented")

	// ErrNotSupported is returned whenever there's a test, which can be used to guard against the error,
	// but the given parameters aren't supported by the system.
	ErrNotSupported = errors.New("not supported")

	// ErrInput signifies errors where the input isn't valid for some reason. And no more specific error exists.
	ErrInput = errors.New("input error")
)

type hookExecutor interface {
	ExecutePostHooks(ctx context.Context, id int, hookType hook.TriggerEnum, input interface{}, inputFields []string)
}

type Resolver struct {
	repository     models.Repository
	sceneService   manager.SceneService
	imageService   manager.ImageService
	galleryService manager.GalleryService
	groupService   manager.GroupService

	hookExecutor hookExecutor
}

func (r *Resolver) scraperCache() *scraper.Cache {
	return manager.GetInstance().ScraperCache
}

func (r *Resolver) Gallery() GalleryResolver {
	return &galleryResolver{r}
}
func (r *Resolver) GalleryChapter() GalleryChapterResolver {
	return &galleryChapterResolver{r}
}
func (r *Resolver) Mutation() MutationResolver {
	return &mutationResolver{r}
}
func (r *Resolver) Performer() PerformerResolver {
	return &performerResolver{r}
}
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}
func (r *Resolver) Scene() SceneResolver {
	return &sceneResolver{r}
}
func (r *Resolver) Image() ImageResolver {
	return &imageResolver{r}
}
func (r *Resolver) SceneMarker() SceneMarkerResolver {
	return &sceneMarkerResolver{r}
}
func (r *Resolver) Studio() StudioResolver {
	return &studioResolver{r}
}

func (r *Resolver) Group() GroupResolver {
	return &groupResolver{r}
}
func (r *Resolver) Movie() MovieResolver {
	return &movieResolver{&groupResolver{r}}
}

func (r *Resolver) Subscription() SubscriptionResolver {
	return &subscriptionResolver{r}
}
func (r *Resolver) Tag() TagResolver {
	return &tagResolver{r}
}
func (r *Resolver) GalleryFile() GalleryFileResolver {
	return &galleryFileResolver{r}
}
func (r *Resolver) VideoFile() VideoFileResolver {
	return &videoFileResolver{r}
}
func (r *Resolver) ImageFile() ImageFileResolver {
	return &imageFileResolver{r}
}
func (r *Resolver) BasicFile() BasicFileResolver {
	return &basicFileResolver{r}
}
func (r *Resolver) Folder() FolderResolver {
	return &folderResolver{r}
}
func (r *Resolver) SavedFilter() SavedFilterResolver {
	return &savedFilterResolver{r}
}
func (r *Resolver) Plugin() PluginResolver {
	return &pluginResolver{r}
}
func (r *Resolver) ConfigResult() ConfigResultResolver {
	return &configResultResolver{r}
}
func (r *Resolver) AIConfig() AIConfigResolver {
	return &aiConfigResolver{r}
}

type aiConfigResolver struct{ *Resolver }

func (r *aiConfigResolver) ImageEmbeddingBaseURL(ctx context.Context, obj *config.AIConfig) (string, error) {
	return obj.ImageEmbeddingBaseURL, nil
}

func (r *aiConfigResolver) ScheduledTasks(ctx context.Context, obj *config.AIConfig) (*AIScheduledTasksConfig, error) {
	return &AIScheduledTasksConfig{
		EmbeddingRefreshHours: obj.ScheduledTasks.EmbeddingRefreshHours,
		AudioAnalysisHours:    obj.ScheduledTasks.AudioAnalysisHours,
	}, nil
}

func (r *Resolver) AIConfigInput() AIConfigInputResolver {
	return &aiConfigInputResolver{r}
}

type aiConfigInputResolver struct{ *Resolver }

func (r *aiConfigInputResolver) ImageEmbeddingBaseURL(ctx context.Context, obj *config.AIConfigInput, data *string) error {
	obj.ImageEmbeddingBaseURL = data
	return nil
}

func (r *aiConfigInputResolver) ScheduledTasks(ctx context.Context, obj *config.AIConfigInput, data *AIScheduledTasksConfigInput) error {
	if data == nil {
		obj.ScheduledTasks = nil
		return nil
	}
	cfg := config.AIScheduledTasksConfig{}
	if data.EmbeddingRefreshHours != nil {
		cfg.EmbeddingRefreshHours = *data.EmbeddingRefreshHours
	}
	if data.AudioAnalysisHours != nil {
		cfg.AudioAnalysisHours = *data.AudioAnalysisHours
	}
	obj.ScheduledTasks = &cfg
	return nil
}

func (r *Resolver) AIChatMessage() AIChatMessageResolver {
	return &aiChatMessageResolver{r}
}
func (r *Resolver) AIChatSession() AIChatSessionResolver {
	return &aiChatSessionResolver{r}
}
func (r *Resolver) AISuggestion() AISuggestionResolver {
	return &aiSuggestionResolver{r}
}

type aiSuggestionResolver struct{ *Resolver }

func (r *aiSuggestionResolver) ID(ctx context.Context, obj *AISuggestion) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiSuggestionResolver) Scene(ctx context.Context, obj *AISuggestion) (*models.Scene, error) {
	return obj.Scene, nil
}

func (r *aiSuggestionResolver) Image(ctx context.Context, obj *AISuggestion) (*models.Image, error) {
	return obj.Image, nil
}

func (r *Resolver) AIPerformerMergeSuggestion() AIPerformerMergeSuggestionResolver {
	return &aiPerformerMergeSuggestionResolver{r}
}

type aiPerformerMergeSuggestionResolver struct{ *Resolver }

func (r *aiPerformerMergeSuggestionResolver) ID(ctx context.Context, obj *AIPerformerMergeSuggestion) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiPerformerMergeSuggestionResolver) Source(ctx context.Context, obj *AIPerformerMergeSuggestion) (*models.Performer, error) {
	return obj.Source, nil
}

func (r *aiPerformerMergeSuggestionResolver) Target(ctx context.Context, obj *AIPerformerMergeSuggestion) (*models.Performer, error) {
	return obj.Target, nil
}

func (r *Resolver) AITranslation() AITranslationResolver {
	return &aiTranslationResolver{r}
}

type aiTranslationResolver struct{ *Resolver }

func (r *aiTranslationResolver) ID(ctx context.Context, obj *AITranslation) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiTranslationResolver) EntityID(ctx context.Context, obj *AITranslation) (string, error) {
	return obj.EntityID, nil
}

func (r *aiTranslationResolver) Scene(ctx context.Context, obj *AITranslation) (*models.Scene, error) {
	return obj.Scene, nil
}

func (r *aiTranslationResolver) Image(ctx context.Context, obj *AITranslation) (*models.Image, error) {
	return obj.Image, nil
}

func (r *aiTranslationResolver) Performer(ctx context.Context, obj *AITranslation) (*models.Performer, error) {
	return obj.Performer, nil
}

func (r *Resolver) AIPerformerCandidate() AIPerformerCandidateResolver {
	return &aiPerformerCandidateResolver{r}
}

type aiPerformerCandidateResolver struct{ *Resolver }

func (r *aiPerformerCandidateResolver) ID(ctx context.Context, obj *AIPerformerCandidate) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiPerformerCandidateResolver) EntityID(ctx context.Context, obj *AIPerformerCandidate) (string, error) {
	return obj.EntityID, nil
}

func (r *aiPerformerCandidateResolver) MemberIDs(ctx context.Context, obj *AIPerformerCandidate) ([]string, error) {
	return obj.MemberIDs, nil
}

func (r *aiPerformerCandidateResolver) Scene(ctx context.Context, obj *AIPerformerCandidate) (*models.Scene, error) {
	return obj.Scene, nil
}

func (r *aiPerformerCandidateResolver) Image(ctx context.Context, obj *AIPerformerCandidate) (*models.Image, error) {
	return obj.Image, nil
}

func (r *Resolver) AIAudit() AIAuditResolver {
	return &aiAuditResolver{r}
}

type aiAuditResolver struct{ *Resolver }

func (r *aiAuditResolver) ID(ctx context.Context, obj *AIAudit) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiAuditResolver) EntityID(ctx context.Context, obj *AIAudit) (string, error) {
	return obj.EntityID, nil
}

func (r *aiAuditResolver) Scene(ctx context.Context, obj *AIAudit) (*models.Scene, error) {
	return obj.Scene, nil
}

func (r *aiAuditResolver) Image(ctx context.Context, obj *AIAudit) (*models.Image, error) {
	return obj.Image, nil
}

func (r *Resolver) AIMediaQuality() AIMediaQualityResolver {
	return &aiMediaQualityResolver{r}
}

type aiMediaQualityResolver struct{ *Resolver }

func (r *aiMediaQualityResolver) ID(ctx context.Context, obj *AIMediaQuality) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiMediaQualityResolver) Scene(ctx context.Context, obj *AIMediaQuality) (*models.Scene, error) {
	return obj.Scene, nil
}

func (r *aiMediaQualityResolver) Image(ctx context.Context, obj *AIMediaQuality) (*models.Image, error) {
	return obj.Image, nil
}

func (r *Resolver) AISceneAudio() AISceneAudioResolver {
	return &aiSceneAudioResolver{r}
}

type aiSceneAudioResolver struct{ *Resolver }

func (r *aiSceneAudioResolver) ID(ctx context.Context, obj *AISceneAudio) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiSceneAudioResolver) SceneID(ctx context.Context, obj *AISceneAudio) (string, error) {
	return fmt.Sprintf("%d", obj.SceneID), nil
}

func (r *aiSceneAudioResolver) Scene(ctx context.Context, obj *AISceneAudio) (*models.Scene, error) {
	return obj.Scene, nil
}

func (r *Resolver) AIPerformerCareer() AIPerformerCareerResolver {
	return &aiPerformerCareerResolver{r}
}

type aiPerformerCareerResolver struct{ *Resolver }

func (r *aiPerformerCareerResolver) ID(ctx context.Context, obj *AIPerformerCareer) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiPerformerCareerResolver) PerformerID(ctx context.Context, obj *AIPerformerCareer) (string, error) {
	return fmt.Sprintf("%d", obj.PerformerID), nil
}

func (r *aiPerformerCareerResolver) Performer(ctx context.Context, obj *AIPerformerCareer) (*models.Performer, error) {
	return obj.Performer, nil
}

func (r *Resolver) AIFileRename() AIFileRenameResolver {
	return &aiFileRenameResolver{r}
}

type aiFileRenameResolver struct{ *Resolver }

func (r *aiFileRenameResolver) ID(ctx context.Context, obj *AIFileRename) (string, error) {
	return fmt.Sprintf("%d", obj.ID), nil
}

func (r *aiFileRenameResolver) EntityID(ctx context.Context, obj *AIFileRename) (string, error) {
	return fmt.Sprintf("%d", obj.EntityID), nil
}

func (r *aiFileRenameResolver) Scene(ctx context.Context, obj *AIFileRename) (*models.Scene, error) {
	return obj.Scene, nil
}

func (r *aiFileRenameResolver) Image(ctx context.Context, obj *AIFileRename) (*models.Image, error) {
	return obj.Image, nil
}

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }

type galleryResolver struct{ *Resolver }
type galleryChapterResolver struct{ *Resolver }
type performerResolver struct{ *Resolver }
type sceneResolver struct{ *Resolver }
type sceneMarkerResolver struct{ *Resolver }
type imageResolver struct{ *Resolver }
type studioResolver struct{ *Resolver }

// movie is group under the hood
type groupResolver struct{ *Resolver }
type movieResolver struct{ *groupResolver }

type tagResolver struct{ *Resolver }
type galleryFileResolver struct{ *Resolver }
type videoFileResolver struct{ *Resolver }
type imageFileResolver struct{ *Resolver }
type basicFileResolver struct{ *Resolver }
type folderResolver struct{ *Resolver }
type savedFilterResolver struct{ *Resolver }
type pluginResolver struct{ *Resolver }
type configResultResolver struct{ *Resolver }

func (r *Resolver) withTxn(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.repository.WithTxn(ctx, fn)
}

func (r *Resolver) withReadTxn(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.repository.WithReadTxn(ctx, fn)
}

// idOnly returns true if the query is only asking for the id field.
// This can be used to optimize certain queries where we don't need to load the full object if we're only getting the id.
func (r *Resolver) idOnly(ctx context.Context) bool {
	fields := graphql.CollectAllFields(ctx)
	return len(fields) == 1 && fields[0] == "id"
}

func (r *queryResolver) MarkerWall(ctx context.Context, q *string) (ret []*models.SceneMarker, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.SceneMarker.Wall(ctx, q)
		return err
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (r *queryResolver) SceneWall(ctx context.Context, q *string) (ret []*models.Scene, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.Scene.Wall(ctx, q)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *queryResolver) MarkerStrings(ctx context.Context, q *string, sort *string) (ret []*models.MarkerStringsResultType, err error) {
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		ret, err = r.repository.SceneMarker.GetMarkerStrings(ctx, q, sort)
		return err
	}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *queryResolver) Stats(ctx context.Context) (*StatsResultType, error) {
	var ret StatsResultType
	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		repo := r.repository
		sceneQB := repo.Scene
		imageQB := repo.Image
		galleryQB := repo.Gallery
		studioQB := repo.Studio
		performerQB := repo.Performer
		movieQB := repo.Group
		tagQB := repo.Tag

		// embrace the error

		scenesCount, err := sceneQB.Count(ctx)
		if err != nil {
			return err
		}

		scenesSize, err := sceneQB.Size(ctx)
		if err != nil {
			return err
		}

		scenesDuration, err := sceneQB.Duration(ctx)
		if err != nil {
			return err
		}

		imageCount, err := imageQB.Count(ctx)
		if err != nil {
			return err
		}

		imageSize, err := imageQB.Size(ctx)
		if err != nil {
			return err
		}

		galleryCount, err := galleryQB.Count(ctx)
		if err != nil {
			return err
		}

		performersCount, err := performerQB.Count(ctx)
		if err != nil {
			return err
		}

		studiosCount, err := studioQB.Count(ctx)
		if err != nil {
			return err
		}

		groupsCount, err := movieQB.Count(ctx)
		if err != nil {
			return err
		}

		tagsCount, err := tagQB.Count(ctx)
		if err != nil {
			return err
		}

		scenesTotalOCount, err := sceneQB.GetAllOCount(ctx)
		if err != nil {
			return err
		}
		imagesTotalOCount, err := imageQB.OCount(ctx)
		if err != nil {
			return err
		}
		totalOCount := scenesTotalOCount + imagesTotalOCount

		totalPlayDuration, err := sceneQB.PlayDuration(ctx)
		if err != nil {
			return err
		}

		totalPlayCount, err := sceneQB.CountAllViews(ctx)
		if err != nil {
			return err
		}

		uniqueScenePlayCount, err := sceneQB.CountUniqueViews(ctx)
		if err != nil {
			return err
		}

		ret = StatsResultType{
			SceneCount:        scenesCount,
			ScenesSize:        scenesSize,
			ScenesDuration:    scenesDuration,
			ImageCount:        imageCount,
			ImagesSize:        imageSize,
			GalleryCount:      galleryCount,
			PerformerCount:    performersCount,
			StudioCount:       studiosCount,
			GroupCount:        groupsCount,
			MovieCount:        groupsCount,
			TagCount:          tagsCount,
			TotalOCount:       totalOCount,
			TotalPlayDuration: totalPlayDuration,
			TotalPlayCount:    totalPlayCount,
			ScenesPlayed:      uniqueScenePlayCount,
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &ret, nil
}

func (r *queryResolver) Version(ctx context.Context) (*Version, error) {
	version, hash, buildtime := build.Version()

	return &Version{
		Version:   &version,
		Hash:      hash,
		BuildTime: buildtime,
	}, nil
}

func (r *queryResolver) Latestversion(ctx context.Context) (*LatestVersion, error) {
	latestRelease, err := GetLatestRelease(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.Errorf("Error while retrieving latest version: %v", err)
		}
		return nil, err
	}
	logger.Infof("Retrieved latest version: %s (%s)", latestRelease.Version, latestRelease.ShortHash)

	return &LatestVersion{
		Version:     latestRelease.Version,
		Shorthash:   latestRelease.ShortHash,
		ReleaseDate: latestRelease.Date,
		URL:         latestRelease.Url,
	}, nil
}

func (r *mutationResolver) ExecSQL(ctx context.Context, sql string, args []interface{}) (*SQLExecResult, error) {
	var rowsAffected *int64
	var lastInsertID *int64

	db := manager.GetInstance().Database
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		var err error
		rowsAffected, lastInsertID, err = db.ExecSQL(ctx, sql, args)
		return err
	}); err != nil {
		return nil, err
	}

	return &SQLExecResult{
		RowsAffected: rowsAffected,
		LastInsertID: lastInsertID,
	}, nil
}

func (r *mutationResolver) QuerySQL(ctx context.Context, sql string, args []interface{}) (*SQLQueryResult, error) {
	var cols []string
	var rows [][]interface{}

	db := manager.GetInstance().Database
	if err := r.withTxn(ctx, func(ctx context.Context) error {
		var err error
		cols, rows, err = db.QuerySQL(ctx, sql, args)
		return err
	}); err != nil {
		return nil, err
	}

	return &SQLQueryResult{
		Columns: cols,
		Rows:    rows,
	}, nil
}

// Get scene marker tags which show up under the video.
func (r *queryResolver) SceneMarkerTags(ctx context.Context, scene_id string) ([]*SceneMarkerTag, error) {
	sceneID, err := strconv.Atoi(scene_id)
	if err != nil {
		return nil, err
	}

	var keys []int
	tags := make(map[int]*SceneMarkerTag)

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		sceneMarkers, err := r.repository.SceneMarker.FindBySceneID(ctx, sceneID)
		if err != nil {
			return err
		}

		tqb := r.repository.Tag
		for _, sceneMarker := range sceneMarkers {
			markerPrimaryTag, err := tqb.Find(ctx, sceneMarker.PrimaryTagID)
			if err != nil {
				return err
			}

			if markerPrimaryTag == nil {
				return fmt.Errorf("tag with id %d not found", sceneMarker.PrimaryTagID)
			}

			_, hasKey := tags[markerPrimaryTag.ID]
			if !hasKey {
				sceneMarkerTag := &SceneMarkerTag{Tag: markerPrimaryTag}
				tags[markerPrimaryTag.ID] = sceneMarkerTag
				keys = append(keys, markerPrimaryTag.ID)
			}
			tags[markerPrimaryTag.ID].SceneMarkers = append(tags[markerPrimaryTag.ID].SceneMarkers, sceneMarker)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Sort so that primary tags that show up earlier in the video are first.
	sort.Slice(keys, func(i, j int) bool {
		a := tags[keys[i]]
		b := tags[keys[j]]
		return a.SceneMarkers[0].Seconds < b.SceneMarkers[0].Seconds
	})

	var result []*SceneMarkerTag
	for _, key := range keys {
		result = append(result, tags[key])
	}

	return result, nil
}

func firstError(errs []error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}

	return nil
}
