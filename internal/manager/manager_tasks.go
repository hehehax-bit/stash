package manager

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/file"
	file_image "github.com/stashapp/stash/pkg/file/image"
	"github.com/stashapp/stash/pkg/file/video"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/tag"
)

func useAsVideo(pathname string) bool {
	stash := config.StashConfigs.GetStashFromDirPath(instance.Config.GetStashPaths(), pathname)

	if instance.Config.IsCreateImageClipsFromVideos() && stash != nil && stash.ExcludeVideo {
		return false
	}
	return isVideo(pathname)
}

func useAsImage(pathname string) bool {
	stash := config.StashConfigs.GetStashFromDirPath(instance.Config.GetStashPaths(), pathname)
	if instance.Config.IsCreateImageClipsFromVideos() && stash != nil && stash.ExcludeVideo {
		return isImage(pathname) || isVideo(pathname)
	}
	return isImage(pathname)
}

func isZip(pathname string) bool {
	gExt := config.GetInstance().GetGalleryExtensions()
	return fsutil.MatchExtension(pathname, gExt)
}

func isVideo(pathname string) bool {
	vidExt := config.GetInstance().GetVideoExtensions()
	return fsutil.MatchExtension(pathname, vidExt)
}

func isImage(pathname string) bool {
	imgExt := config.GetInstance().GetImageExtensions()
	return fsutil.MatchExtension(pathname, imgExt)
}

func getScanPaths(inputPaths []string) []*config.StashConfig {
	stashPaths := config.GetInstance().GetStashPaths()

	if len(inputPaths) == 0 {
		return stashPaths
	}

	var ret config.StashConfigs
	for _, p := range inputPaths {
		s := stashPaths.GetStashFromDirPath(p)
		if s == nil {
			logger.Warnf("%s is not in the configured stash paths", p)
			continue
		}

		// make a copy, changing the path
		ss := *s
		ss.Path = p
		ret = append(ret, &ss)
	}

	return ret
}

// Filters the input array for paths that are within the paths managed by stash
func filterStashPaths(inputPaths []string) []string {
	if len(inputPaths) == 0 {
		return inputPaths
	}

	stashPaths := config.GetInstance().GetStashPaths()

	var ret []string
	for _, p := range inputPaths {
		s := stashPaths.GetStashFromDirPath(p)
		if s == nil {
			logger.Warnf("%s is not in the configured stash paths", p)
			continue
		}

		ret = append(ret, p)
	}

	return ret
}

// ScanSubscribe subscribes to a notification that is triggered when a
// scan or clean is complete.
func (s *Manager) ScanSubscribe(ctx context.Context) <-chan bool {
	return s.scanSubs.subscribe(ctx)
}

type ScanMetadataInput struct {
	Paths []string `json:"paths"`

	config.ScanMetadataOptions `mapstructure:",squash"`

	// Filter options for the scan
	Filter *ScanMetaDataFilterInput `json:"filter"`
}

// Filter options for meta data scannning
type ScanMetaDataFilterInput struct {
	// If set, files with a modification time before this time point are ignored by the scan
	MinModTime *time.Time `json:"minModTime"`
}

func (s *Manager) Scan(ctx context.Context, input ScanMetadataInput) (int, error) {
	if err := s.validateFFmpeg(); err != nil {
		return 0, err
	}

	cfg := config.GetInstance()

	scanner := &file.Scanner{
		Repository: file.NewRepository(s.Repository),
		FileDecorators: []file.Decorator{
			&file.FilteredDecorator{
				Decorator: &video.Decorator{
					FFProbe: s.FFProbe,
				},
				Filter: file.FilterFunc(videoFileFilter),
			},
			&file.FilteredDecorator{
				Decorator: &file_image.Decorator{
					FFProbe: s.FFProbe,
				},
				Filter: file.FilterFunc(imageFileFilter),
			},
		},
		FingerprintCalculator: &fingerprintCalculator{s.Config},
		FS:                    &file.OsFS{},
		ZipFileExtensions:     cfg.GetGalleryExtensions(),
		// ScanFilters is set in ScanJob.Execute
		// HandlerRequiredFilters is set in ScanJob.Execute
		// #4425 - isRootPath compares these against the NFC paths stored during scanning
		RootPaths: fsutil.NormalizePaths(cfg.GetStashPaths().Paths()),
		Rescan:    input.Rescan,
	}

	scanJob := ScanJob{
		scanner:       scanner,
		input:         input,
		subscriptions: s.scanSubs,
	}

	return s.JobManager.Add(ctx, "Scanning...", &scanJob), nil
}

func (s *Manager) Import(ctx context.Context) (int, error) {
	config := config.GetInstance()
	metadataPath := config.GetMetadataPath()
	if metadataPath == "" {
		return 0, errors.New("metadata path must be set in config")
	}

	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		task := ImportTask{
			repository:          s.Repository,
			resetter:            s.Database,
			BaseDir:             metadataPath,
			Reset:               true,
			DuplicateBehaviour:  ImportDuplicateEnumFail,
			MissingRefBehaviour: models.ImportMissingRefEnumFail,
			fileNamingAlgorithm: config.GetVideoFileNamingAlgorithm(),
		}
		task.Start(ctx)

		// TODO - return error from task
		return nil
	})

	return s.JobManager.Add(ctx, "Importing...", j), nil
}

func (s *Manager) Export(ctx context.Context) (int, error) {
	config := config.GetInstance()
	metadataPath := config.GetMetadataPath()
	if metadataPath == "" {
		return 0, errors.New("metadata path must be set in config")
	}

	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		var wg sync.WaitGroup
		wg.Add(1)
		task := ExportTask{
			repository:          s.Repository,
			full:                true,
			fileNamingAlgorithm: config.GetVideoFileNamingAlgorithm(),
		}
		task.Start(ctx, &wg)
		// TODO - return error from task
		return nil
	})

	return s.JobManager.Add(ctx, "Exporting...", j), nil
}

func (s *Manager) RunSingleTask(ctx context.Context, t Task) int {
	var wg sync.WaitGroup
	wg.Add(1)

	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		t.Start(ctx)
		defer wg.Done()
		// TODO - return error from task
		return nil
	})

	return s.JobManager.Add(ctx, t.GetDescription(), j)
}

func (s *Manager) AIImageTag(ctx context.Context, input AIImageTagInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIImageTagJob(input)
	return s.JobManager.AddWithType(ctx, "AI Tagging Images...", "ai", j), nil
}

func (s *Manager) AIPerformerTag(ctx context.Context, input AIPerformerTagInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIPerformerTagJob(input)
	return s.JobManager.AddWithType(ctx, "AI Tagging Performer...", "ai", j), nil
}

func (s *Manager) AISceneTag(ctx context.Context, input AISceneTagInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAISceneTagJob(input)
	return s.JobManager.AddWithType(ctx, "AI Tagging Scenes...", "ai", j), nil
}

func (s *Manager) DetectLooping(ctx context.Context, input DetectLoopingInput) int {
	j := CreateDetectLoopingJob(input)
	return s.JobManager.AddWithType(ctx, "Detecting Looping Videos...", "ai", j)
}

func (s *Manager) AITagOrganize(ctx context.Context, input AITagOrganizeInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAITagOrganizeJob(input)
	return s.JobManager.AddWithType(ctx, "AI Organizing Tags...", "ai", j), nil
}

func (s *Manager) AIPerformerCluster(ctx context.Context, input AIPerformerClusterInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIPerformerClusterJob(input)
	return s.JobManager.AddWithType(ctx, "AI Clustering Performers...", "ai", j), nil
}

// GenerateHighlightClip enqueues a job that cuts a clip around a scene marker
// and returns the output clip path.
// AIMoodGroupCreate creates a group named after a mood and attaches all
// scenes carrying that mood, reusing the smart collections group pattern.
func (s *Manager) AIMoodGroupCreate(ctx context.Context, mood string) (int, error) {
	r := instance.Repository

	var sceneIDs []int
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		sceneIDs, err = r.AIMood.FindMoods(ctx, mood, 100000)
		return err
	}); err != nil {
		return 0, err
	}
	if len(sceneIDs) == 0 {
		return 0, fmt.Errorf("no scenes carry the mood %q", mood)
	}

	name := mood
	if name != "" {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	name += " scenes"

	var groupID int
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		newGroup := models.NewGroup()
		newGroup.Name = name
		if err := r.Group.Create(ctx, &newGroup); err != nil {
			return fmt.Errorf("creating group: %w", err)
		}
		groupID = newGroup.ID

		groupUpdate := &models.UpdateGroupIDs{
			Groups: []models.GroupsScenes{{GroupID: groupID}},
			Mode:   models.RelationshipUpdateModeAdd,
		}
		for _, sceneID := range sceneIDs {
			partial := models.ScenePartial{GroupIDs: groupUpdate}
			if _, err := r.Scene.UpdatePartial(ctx, sceneID, partial); err != nil {
				return fmt.Errorf("adding scene %d to group: %w", sceneID, err)
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}

	logger.Infof("Created mood group %q with %d scenes", name, len(sceneIDs))
	return groupID, nil
}

func (s *Manager) AIMoodTag(ctx context.Context, input AIMoodInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIMoodJob(input)
	return s.JobManager.AddWithType(ctx, "AI Tagging Moods...", "ai", j), nil
}

// GenerateGoonReel enqueues a job that concatenates best-moment clips of the
// given scenes into a single video and returns the output path.
func (s *Manager) GenerateGoonReel(ctx context.Context, sceneIDs []int, durationPerScene int) (string, error) {
	clipsDir := filepath.Join(instance.Config.GetConfigPath(), "clips")
	outputPath := filepath.Join(clipsDir, fmt.Sprintf("goon_reel_%d.mp4", time.Now().Unix()))

	s.JobManager.AddWithType(ctx, "Generating Goon Reel...", "ai", CreateGenerateGoonReelJob(sceneIDs, durationPerScene))

	return outputPath, nil
}

func (s *Manager) GenerateHighlightClip(ctx context.Context, sceneID, markerID, duration int) (string, error) {
	clipsDir := filepath.Join(instance.Config.GetConfigPath(), "clips")
	outputPath := filepath.Join(clipsDir, fmt.Sprintf("scene_%d_marker_%d.mp4", sceneID, markerID))

	s.JobManager.Add(ctx, "Generating Highlight Clip...", CreateGenerateHighlightClipJob(sceneID, markerID, duration))

	return outputPath, nil
}

func (s *Manager) AIPerformerMergeSuggest(ctx context.Context, input AIPerformerMergeSuggestInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIPerformerMergeSuggestJob(input)
	return s.JobManager.AddWithType(ctx, "AI Suggesting Performer Merges...", "ai", j), nil
}

func (s *Manager) AIPerformerSuggestionApply(ctx context.Context, suggestionID int64) error {
	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	r := instance.Repository

	var suggestion *models.AIPerformerSuggestion
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		suggestion, err = r.AIPerformerSuggestion.FindByID(ctx, suggestionID)
		return err
	}); err != nil {
		return fmt.Errorf("finding suggestion: %w", err)
	}
	if suggestion == nil {
		return fmt.Errorf("suggestion %d not found", suggestionID)
	}
	if suggestion.Status != models.SuggestionStatusPending {
		return fmt.Errorf("suggestion %d is not pending", suggestionID)
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		if err := r.Performer.Merge(ctx, []int{suggestion.SourcePerformerID}, suggestion.TargetPerformerID); err != nil {
			return err
		}
		return r.AIPerformerSuggestion.UpdateStatus(ctx, suggestionID, models.SuggestionStatusAccepted)
	}); err != nil {
		return fmt.Errorf("merging performers: %w", err)
	}

	return nil
}

func (s *Manager) AIPerformerDiscovery(ctx context.Context, input AIPerformerDiscoveryInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIPerformerDiscoveryJob(input)
	return s.JobManager.AddWithType(ctx, "AI Discovering Performers...", "ai", j), nil
}

// AIPerformerCandidateApply creates a performer from the candidate (or attaches
// its members to an existing target performer) and marks the candidate applied.
func (s *Manager) AIPerformerCandidateApply(ctx context.Context, candidateID int64, targetPerformerID *int) error {
	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	r := instance.Repository

	var candidate *models.AIPerformerCandidate
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		candidate, err = r.AIPerformerCandidate.FindByID(ctx, candidateID)
		return err
	}); err != nil {
		return fmt.Errorf("finding candidate: %w", err)
	}
	if candidate == nil {
		return fmt.Errorf("candidate %d not found", candidateID)
	}
	if candidate.Status != models.SuggestionStatusPending {
		return fmt.Errorf("candidate %d is not pending", candidateID)
	}

	performerID := 0
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		if targetPerformerID != nil && *targetPerformerID > 0 {
			performerID = *targetPerformerID
		} else {
			newPerformer := models.NewPerformer()
			newPerformer.Name = candidate.Name
			if err := r.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &newPerformer}); err != nil {
				return fmt.Errorf("creating performer: %w", err)
			}
			performerID = newPerformer.ID
		}

		for _, memberID := range candidate.MemberIDs {
			if err := attachCandidateMember(ctx, r, candidate.EntityType, memberID, performerID); err != nil {
				logger.Warnf("Error attaching member %d to performer %d: %v", memberID, performerID, err)
			}
		}

		return r.AIPerformerCandidate.UpdateStatus(ctx, candidateID, models.SuggestionStatusAccepted)
	}); err != nil {
		return err
	}

	return nil
}

func (s *Manager) AIPerformerCandidateReject(ctx context.Context, candidateID int64) error {
	return instance.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return instance.Repository.AIPerformerCandidate.UpdateStatus(ctx, candidateID, models.SuggestionStatusRejected)
	})
}

func attachCandidateMember(ctx context.Context, r models.Repository, entityType string, entityID, performerID int) error {
	update := &models.UpdateIDs{
		IDs:  []int{performerID},
		Mode: models.RelationshipUpdateModeAdd,
	}

	switch entityType {
	case entityTypeScene:
		partial := models.NewScenePartial()
		partial.PerformerIDs = update
		_, err := r.Scene.UpdatePartial(ctx, entityID, partial)
		return err
	case entityTypeImage:
		partial := models.NewImagePartial()
		partial.PerformerIDs = update
		_, err := r.Image.UpdatePartial(ctx, entityID, partial)
		return err
	}
	return nil
}

func (s *Manager) AITranslate(ctx context.Context, input AITranslateInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAITranslateJob(input)
	return s.JobManager.AddWithType(ctx, "AI Translating Library...", "ai", j), nil
}

func (s *Manager) AITranslationApply(ctx context.Context, translationID int64) error {
	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	r := instance.Repository

	var translation *models.AITranslation
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		translation, err = r.AITranslation.FindByID(ctx, translationID)
		return err
	}); err != nil {
		return fmt.Errorf("finding translation: %w", err)
	}
	if translation == nil {
		return fmt.Errorf("translation %d not found", translationID)
	}
	if translation.Status != models.SuggestionStatusPending {
		return fmt.Errorf("translation %d is not pending", translationID)
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		if err := applyTranslationToEntity(ctx, r, translation); err != nil {
			return err
		}
		return r.AITranslation.UpdateStatus(ctx, translationID, models.SuggestionStatusAccepted)
	}); err != nil {
		return err
	}

	return nil
}

func (s *Manager) AITranslationReject(ctx context.Context, translationID int64) error {
	return instance.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return instance.Repository.AITranslation.UpdateStatus(ctx, translationID, models.SuggestionStatusRejected)
	})
}

func applyTranslationToEntity(ctx context.Context, r models.Repository, translation *models.AITranslation) error {
	switch translation.EntityType {
	case entityTypeScene:
		partial := models.NewScenePartial()
		if translation.Field == "title" {
			partial.Title = models.NewOptionalString(translation.TranslatedText)
		} else if translation.Field == "details" {
			partial.Details = models.NewOptionalString(translation.TranslatedText)
		}
		_, err := r.Scene.UpdatePartial(ctx, translation.EntityID, partial)
		return err
	case entityTypeImage:
		partial := models.NewImagePartial()
		if translation.Field == "title" {
			partial.Title = models.NewOptionalString(translation.TranslatedText)
		} else if translation.Field == "details" {
			partial.Details = models.NewOptionalString(translation.TranslatedText)
		}
		_, err := r.Image.UpdatePartial(ctx, translation.EntityID, partial)
		return err
	case entityTypePerformer:
		partial := models.NewPerformerPartial()
		if translation.Field == "name" {
			partial.Name = models.NewOptionalString(translation.TranslatedText)
		} else if translation.Field == "details" {
			partial.Details = models.NewOptionalString(translation.TranslatedText)
		}
		_, err := r.Performer.UpdatePartial(ctx, translation.EntityID, partial)
		return err
	}
	return nil
}

func (s *Manager) AIAudit(ctx context.Context, input AIAuditInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIAuditJob(input)
	return s.JobManager.AddWithType(ctx, "AI Auditing Scenes and Images...", "ai", j), nil
}

func (s *Manager) AIAuditApply(ctx context.Context, auditID int64) error {
	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	r := instance.Repository

	var audit *models.AIAudit
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		audit, err = r.AIAudit.FindByID(ctx, auditID)
		return err
	}); err != nil {
		return fmt.Errorf("finding audit: %w", err)
	}
	if audit == nil {
		return fmt.Errorf("audit %d not found", auditID)
	}
	if audit.Status != models.SuggestionStatusPending {
		return fmt.Errorf("audit %d is not pending", auditID)
	}

	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		switch audit.Field {
		case "title", "details":
			if err := applyAuditTextField(ctx, r, audit); err != nil {
				return err
			}
		case "performers":
			resolver := &performerResolver{r: r}
			performerID, err := resolver.resolve(ctx, audit.AIValue, true, nil)
			if err != nil {
				return fmt.Errorf("resolving performer %q: %w", audit.AIValue, err)
			}
			if performerID > 0 {
				if err := attachPerformerToAuditEntity(ctx, r, audit, performerID); err != nil {
					return err
				}
			}
		}
		return r.AIAudit.UpdateStatus(ctx, auditID, models.SuggestionStatusAccepted)
	}); err != nil {
		return err
	}

	return nil
}

func (s *Manager) AIAuditReject(ctx context.Context, auditID int64) error {
	return instance.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return instance.Repository.AIAudit.UpdateStatus(ctx, auditID, models.SuggestionStatusRejected)
	})
}

func applyAuditTextField(ctx context.Context, r models.Repository, audit *models.AIAudit) error {
	switch audit.EntityType {
	case entityTypeScene:
		partial := models.NewScenePartial()
		if audit.Field == "title" {
			partial.Title = models.NewOptionalString(audit.AIValue)
		} else {
			partial.Details = models.NewOptionalString(audit.AIValue)
		}
		_, err := r.Scene.UpdatePartial(ctx, audit.EntityID, partial)
		return err
	case entityTypeImage:
		partial := models.NewImagePartial()
		if audit.Field == "title" {
			partial.Title = models.NewOptionalString(audit.AIValue)
		} else {
			partial.Details = models.NewOptionalString(audit.AIValue)
		}
		_, err := r.Image.UpdatePartial(ctx, audit.EntityID, partial)
		return err
	}
	return nil
}

func attachPerformerToAuditEntity(ctx context.Context, r models.Repository, audit *models.AIAudit, performerID int) error {
	update := &models.UpdateIDs{
		IDs:  []int{performerID},
		Mode: models.RelationshipUpdateModeAdd,
	}

	switch audit.EntityType {
	case entityTypeScene:
		partial := models.NewScenePartial()
		partial.PerformerIDs = update
		_, err := r.Scene.UpdatePartial(ctx, audit.EntityID, partial)
		return err
	case entityTypeImage:
		partial := models.NewImagePartial()
		partial.PerformerIDs = update
		_, err := r.Image.UpdatePartial(ctx, audit.EntityID, partial)
		return err
	}
	return nil
}

func (s *Manager) AIPerformerSuggestionReject(ctx context.Context, suggestionID int64) error {
	return instance.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return instance.Repository.AIPerformerSuggestion.UpdateStatus(ctx, suggestionID, models.SuggestionStatusRejected)
	})
}

func (s *Manager) AIAuditApplyAll(ctx context.Context) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}
	r := instance.Repository

	var pending []*models.AIAudit
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		pending, err = r.AIAudit.FindByStatus(ctx, models.SuggestionStatusPending)
		return err
	}); err != nil {
		return 0, err
	}

	applied := 0
	for _, a := range pending {
		if err := s.AIAuditApply(ctx, a.ID); err != nil {
			logger.Warnf("Error applying audit %d: %v", a.ID, err)
			continue
		}
		applied++
	}
	logger.Infof("AI audit apply-all: %d applied of %d pending", applied, len(pending))
	return applied, nil
}

func (s *Manager) AIAuditRejectAll(ctx context.Context) (int, error) {
	return rejectAll(ctx, rAIAuditRejectAll)
}

func (s *Manager) AITranslationApplyAll(ctx context.Context) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}
	r := instance.Repository

	var pending []*models.AITranslation
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		pending, err = r.AITranslation.FindByStatus(ctx, models.SuggestionStatusPending)
		return err
	}); err != nil {
		return 0, err
	}

	applied := 0
	for _, t := range pending {
		if err := s.AITranslationApply(ctx, t.ID); err != nil {
			logger.Warnf("Error applying translation %d: %v", t.ID, err)
			continue
		}
		applied++
	}
	logger.Infof("AI translation apply-all: %d applied of %d pending", applied, len(pending))
	return applied, nil
}

func (s *Manager) AITranslationRejectAll(ctx context.Context) (int, error) {
	return rejectAll(ctx, rAITranslationRejectAll)
}

func (s *Manager) AIPerformerSuggestionApplyAll(ctx context.Context) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}
	r := instance.Repository

	var pending []*models.AIPerformerSuggestion
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		pending, err = r.AIPerformerSuggestion.FindByStatus(ctx, models.SuggestionStatusPending)
		return err
	}); err != nil {
		return 0, err
	}

	applied := 0
	for _, a := range pending {
		if err := s.AIPerformerSuggestionApply(ctx, a.ID); err != nil {
			logger.Warnf("Error applying merge suggestion %d: %v", a.ID, err)
			continue
		}
		applied++
	}
	logger.Infof("AI merge suggestion apply-all: %d applied of %d pending", applied, len(pending))
	return applied, nil
}

func (s *Manager) AIPerformerSuggestionRejectAll(ctx context.Context) (int, error) {
	return rejectAll(ctx, rAIPerformerSuggestionRejectAll)
}

func (s *Manager) AIPerformerCandidateApplyAll(ctx context.Context) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}
	r := instance.Repository

	var pending []*models.AIPerformerCandidate
	if err := r.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		pending, err = r.AIPerformerCandidate.FindByStatus(ctx, models.SuggestionStatusPending)
		return err
	}); err != nil {
		return 0, err
	}

	applied := 0
	for _, c := range pending {
		if err := s.AIPerformerCandidateApply(ctx, c.ID, nil); err != nil {
			logger.Warnf("Error applying candidate %d: %v", c.ID, err)
			continue
		}
		applied++
	}
	logger.Infof("AI candidate apply-all: %d applied of %d pending", applied, len(pending))
	return applied, nil
}

func (s *Manager) AIPerformerCandidateRejectAll(ctx context.Context) (int, error) {
	return rejectAll(ctx, rAIPerformerCandidateRejectAll)
}

type rejectAllFunc func(context.Context, models.Repository) (int64, error)

func rejectAll(ctx context.Context, fn rejectAllFunc) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}
	r := instance.Repository

	var count int64
	if err := r.WithTxn(ctx, func(ctx context.Context) error {
		var err error
		count, err = fn(ctx, r)
		return err
	}); err != nil {
		return 0, err
	}
	return int(count), nil
}

func rAIAuditRejectAll(ctx context.Context, r models.Repository) (int64, error) {
	return r.AIAudit.UpdateStatusByStatus(ctx, models.SuggestionStatusPending, models.SuggestionStatusRejected)
}

func rAITranslationRejectAll(ctx context.Context, r models.Repository) (int64, error) {
	return r.AITranslation.UpdateStatusByStatus(ctx, models.SuggestionStatusPending, models.SuggestionStatusRejected)
}

func rAIPerformerSuggestionRejectAll(ctx context.Context, r models.Repository) (int64, error) {
	return r.AIPerformerSuggestion.UpdateStatusByStatus(ctx, models.SuggestionStatusPending, models.SuggestionStatusRejected)
}

func rAIPerformerCandidateRejectAll(ctx context.Context, r models.Repository) (int64, error) {
	return r.AIPerformerCandidate.UpdateStatusByStatus(ctx, models.SuggestionStatusPending, models.SuggestionStatusRejected)
}

func (s *Manager) AISceneSegment(ctx context.Context, input AISceneSegmentInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAISceneSegmentJob(input)
	return s.JobManager.AddWithType(ctx, "AI Segmenting Scenes...", "ai", j), nil
}

func (s *Manager) AISuggestionGenerate(ctx context.Context, input AISuggestionInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAISuggestionJob(input)
	return s.JobManager.AddWithType(ctx, "AI Generating Suggestions...", "ai", j), nil
}

func (s *Manager) AIMediaQuality(ctx context.Context, input AIMediaQualityInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIMediaQualityJob(input)
	return s.JobManager.AddWithType(ctx, "AI Assessing Media Quality...", "ai", j), nil
}

func (s *Manager) AIAudioAnalyze(ctx context.Context, input AIAudioAnalyzeInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIAudioAnalyzeJob(input)
	return s.JobManager.AddWithType(ctx, "AI Analyzing Scene Audio...", "ai", j), nil
}

func (s *Manager) AIPerformerCareer(ctx context.Context, input AIPerformerCareerInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIPerformerCareerJob(input)
	return s.JobManager.AddWithType(ctx, "AI Analyzing Performer Careers...", "ai", j), nil
}

func (s *Manager) AISmartCollections(ctx context.Context, input AISmartCollectionsInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAISmartCollectionsJob(input)
	return s.JobManager.AddWithType(ctx, "AI Generating Smart Collections...", "ai", j), nil
}

func (s *Manager) AIFileRenameGenerate(ctx context.Context, input AIFileRenameInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	j := CreateAIFileRenameJob(input)
	return s.JobManager.AddWithType(ctx, "AI Suggesting Filenames...", "ai", j), nil
}

// AIFileRenameApply renames the file on disk and updates the database, then
// marks the rename suggestion as applied. Returns an error if the suggestion
// is not pending.
func (s *Manager) AIFileRenameApply(ctx context.Context, renameID int64) error {
	var rename *models.AIFileRename
	if err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		rename, err = s.Repository.AIFileRename.FindByID(ctx, renameID)
		return err
	}); err != nil {
		return err
	}

	if rename == nil {
		return fmt.Errorf("rename suggestion not found: %d", renameID)
	}

	if rename.Status != models.FileRenameStatusPending {
		return fmt.Errorf("rename suggestion %d is not pending (status %q)", renameID, rename.Status)
	}

	return s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		file, err := s.entityPrimaryFile(ctx, rename)
		if err != nil {
			return err
		}

		oldPath := file.Base().Path
		oldName := filepath.Base(oldPath)
		newName := rename.SuggestedName + filepath.Ext(oldName)
		newPath := filepath.Join(filepath.Dir(oldPath), newName)

		if newPath == oldPath {
			return fmt.Errorf("suggested name %q is the same as the current name", rename.SuggestedName)
		}
		exists, err := fsutil.FileExists(newPath)
		if err != nil {
			return fmt.Errorf("checking destination %q: %w", newPath, err)
		}
		if exists {
			return fmt.Errorf("destination already exists: %q", newPath)
		}
		if err := fsutil.SafeMove(oldPath, newPath); err != nil {
			return fmt.Errorf("renaming %q to %q: %w", oldPath, newPath, err)
		}

		base := file.Base()
		base.Basename = newName
		base.Path = newPath
		if err := s.Repository.File.Update(ctx, file); err != nil {
			// try to roll back the file move
			_ = fsutil.SafeMove(newPath, oldPath)
			return fmt.Errorf("updating file record: %w", err)
		}

		return s.Repository.AIFileRename.UpdateStatus(ctx, renameID, models.FileRenameStatusApplied)
	})
}

// AIFileRenameReject marks the given rename suggestion as rejected.
func (s *Manager) AIFileRenameReject(ctx context.Context, renameID int64) error {
	var rename *models.AIFileRename
	if err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		rename, err = s.Repository.AIFileRename.FindByID(ctx, renameID)
		return err
	}); err != nil {
		return err
	}

	if rename == nil {
		return fmt.Errorf("rename suggestion not found: %d", renameID)
	}

	if rename.Status != models.FileRenameStatusPending {
		return fmt.Errorf("rename suggestion %d is not pending (status %q)", renameID, rename.Status)
	}

	return s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return s.Repository.AIFileRename.UpdateStatus(ctx, renameID, models.FileRenameStatusRejected)
	})
}

func (s *Manager) entityPrimaryFile(ctx context.Context, rename *models.AIFileRename) (models.File, error) {
	switch rename.EntityType {
	case entityTypeScene:
		sc, err := s.Repository.Scene.Find(ctx, rename.EntityID)
		if err != nil {
			return nil, err
		}
		if sc == nil {
			return nil, fmt.Errorf("scene not found: %d", rename.EntityID)
		}
		if err := sc.LoadPrimaryFile(ctx, s.Repository.File); err != nil {
			return nil, err
		}
		f := sc.Files.Primary()
		if f == nil {
			return nil, fmt.Errorf("scene %d has no file", rename.EntityID)
		}
		return f, nil
	case entityTypeImage:
		img, err := s.Repository.Image.Find(ctx, rename.EntityID)
		if err != nil {
			return nil, err
		}
		if img == nil {
			return nil, fmt.Errorf("image not found: %d", rename.EntityID)
		}
		if err := img.LoadPrimaryFile(ctx, s.Repository.File); err != nil {
			return nil, err
		}
		f := img.Files.Primary()
		if f == nil {
			return nil, fmt.Errorf("image %d has no file", rename.EntityID)
		}
		return f, nil
	default:
		return nil, fmt.Errorf("unsupported entity type %q", rename.EntityType)
	}
}

// AISuggestionApply applies the given suggestion to its target entity and marks
// it accepted. Returns an error if the suggestion is not pending.
func (s *Manager) AISuggestionApply(ctx context.Context, suggestionID int64) error {
	var suggestion *models.AISuggestion
	if err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		suggestion, err = s.Repository.AISuggestion.FindByID(ctx, suggestionID)
		return err
	}); err != nil {
		return err
	}

	if suggestion == nil {
		return fmt.Errorf("suggestion not found: %d", suggestionID)
	}

	if suggestion.Status != models.SuggestionStatusPending {
		return fmt.Errorf("suggestion %d is not pending (status %q)", suggestionID, suggestion.Status)
	}

	switch suggestion.EntityType {
	case entityTypeScene:
		return s.applySceneSuggestion(ctx, suggestion)
	case entityTypeImage:
		return s.applyImageSuggestion(ctx, suggestion)
	default:
		return fmt.Errorf("unsupported entity type %q", suggestion.EntityType)
	}
}

// AISuggestionReject marks the given suggestion as rejected.
func (s *Manager) AISuggestionReject(ctx context.Context, suggestionID int64) error {
	var suggestion *models.AISuggestion
	if err := s.Repository.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		suggestion, err = s.Repository.AISuggestion.FindByID(ctx, suggestionID)
		return err
	}); err != nil {
		return err
	}

	if suggestion == nil {
		return fmt.Errorf("suggestion not found: %d", suggestionID)
	}

	if suggestion.Status != models.SuggestionStatusPending {
		return fmt.Errorf("suggestion %d is not pending (status %q)", suggestionID, suggestion.Status)
	}

	return s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		return s.Repository.AISuggestion.UpdateStatus(ctx, suggestionID, models.SuggestionStatusRejected)
	})
}

func (s *Manager) applySceneSuggestion(ctx context.Context, suggestion *models.AISuggestion) error {
	return s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		// Resolve performer IDs
		resolver := &performerResolver{r: s.Repository}
		var performerIDs []int
		for _, name := range suggestion.Performers {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			performerID, err := resolver.resolve(ctx, name, true, nil)
			if err != nil {
				return fmt.Errorf("resolving performer %q: %w", name, err)
			}
			if performerID > 0 {
				performerIDs = append(performerIDs, performerID)
			}
		}

		// Resolve tag IDs
		tagIDs, err := s.resolveTagIDs(ctx, suggestion.Tags)
		if err != nil {
			return err
		}

		partial := models.NewScenePartial()
		if suggestion.Title != "" {
			partial.Title = models.NewOptionalString(suggestion.Title)
		}
		if suggestion.Details != "" {
			partial.Details = models.NewOptionalString(suggestion.Details)
		}
		if len(performerIDs) > 0 {
			partial.PerformerIDs = &models.UpdateIDs{
				IDs:  performerIDs,
				Mode: models.RelationshipUpdateModeAdd,
			}
		}
		if len(tagIDs) > 0 {
			partial.TagIDs = &models.UpdateIDs{
				IDs:  tagIDs,
				Mode: models.RelationshipUpdateModeAdd,
			}
		}

		if _, err := s.Repository.Scene.UpdatePartial(ctx, suggestion.EntityID, partial); err != nil {
			return err
		}

		return s.Repository.AISuggestion.UpdateStatus(ctx, suggestion.ID, models.SuggestionStatusAccepted)
	})
}

func (s *Manager) applyImageSuggestion(ctx context.Context, suggestion *models.AISuggestion) error {
	return s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		// Resolve performer IDs
		resolver := &performerResolver{r: s.Repository}
		var performerIDs []int
		for _, name := range suggestion.Performers {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			performerID, err := resolver.resolve(ctx, name, true, nil)
			if err != nil {
				return fmt.Errorf("resolving performer %q: %w", name, err)
			}
			if performerID > 0 {
				performerIDs = append(performerIDs, performerID)
			}
		}

		tagIDs, err := s.resolveTagIDs(ctx, suggestion.Tags)
		if err != nil {
			return err
		}

		partial := models.NewImagePartial()
		if suggestion.Title != "" {
			partial.Title = models.NewOptionalString(suggestion.Title)
		}
		if suggestion.Details != "" {
			partial.Details = models.NewOptionalString(suggestion.Details)
		}
		if len(performerIDs) > 0 {
			partial.PerformerIDs = &models.UpdateIDs{
				IDs:  performerIDs,
				Mode: models.RelationshipUpdateModeAdd,
			}
		}
		if len(tagIDs) > 0 {
			partial.TagIDs = &models.UpdateIDs{
				IDs:  tagIDs,
				Mode: models.RelationshipUpdateModeAdd,
			}
		}

		if _, err := s.Repository.Image.UpdatePartial(ctx, suggestion.EntityID, partial); err != nil {
			return err
		}

		return s.Repository.AISuggestion.UpdateStatus(ctx, suggestion.ID, models.SuggestionStatusAccepted)
	})
}

func (s *Manager) resolveTagIDs(ctx context.Context, names []string) ([]int, error) {
	var tagIDs []int
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		t, err := tag.ByName(ctx, s.Repository.Tag, name)
		if err != nil {
			return nil, fmt.Errorf("finding tag %q: %w", name, err)
		}
		if t != nil {
			tagIDs = append(tagIDs, t.ID)
			continue
		}
		newTag := models.NewTag()
		newTag.Name = name
		if err := s.Repository.Tag.Create(ctx, &models.CreateTagInput{Tag: &newTag}); err != nil {
			return nil, fmt.Errorf("creating tag %q: %w", name, err)
		}
		tagIDs = append(tagIDs, newTag.ID)
	}
	return tagIDs, nil
}

func (s *Manager) AIEmbedding(ctx context.Context, input AIEmbeddingInput) (int, error) {
	if !instance.Config.GetAIEnabled() {
		return 0, fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	logger.Infof("Creating embedding job with input: %+v", input)
	j := CreateAIEmbeddingJob(input)
	jobID := s.JobManager.AddWithType(ctx, "AI Generating Embeddings...", "ai", j)
	logger.Infof("Created embedding job with ID: %d", jobID)
	return jobID, nil
}

func (s *Manager) Generate(ctx context.Context, input GenerateMetadataInput) (int, error) {
	if err := s.validateFFmpeg(); err != nil {
		return 0, err
	}
	if err := instance.Paths.Generated.EnsureTmpDir(); err != nil {
		logger.Warnf("could not generate temporary directory: %v", err)
	}

	j := &GenerateJob{
		repository: s.Repository,
		input:      input,
	}

	return s.JobManager.Add(ctx, "Generating...", j), nil
}

func (s *Manager) GenerateDefaultScreenshot(ctx context.Context, sceneId string) int {
	return s.generateScreenshot(ctx, sceneId, nil)
}

func (s *Manager) GenerateScreenshot(ctx context.Context, sceneId string, at float64) int {
	return s.generateScreenshot(ctx, sceneId, &at)
}

// generate default screenshot if at is nil
func (s *Manager) generateScreenshot(ctx context.Context, sceneId string, at *float64) int {
	if err := instance.Paths.Generated.EnsureTmpDir(); err != nil {
		logger.Warnf("failure generating screenshot: %v", err)
	}

	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		sceneIdInt, err := strconv.Atoi(sceneId)
		if err != nil {
			return fmt.Errorf("error parsing scene id %s: %w", sceneId, err)
		}

		var scene *models.Scene
		if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
			scene, err = s.Repository.Scene.Find(ctx, sceneIdInt)
			if err != nil {
				return err
			}
			if scene == nil {
				return fmt.Errorf("scene with id %s not found", sceneId)
			}

			return scene.LoadPrimaryFile(ctx, s.Repository.File)
		}); err != nil {
			return fmt.Errorf("error finding scene for screenshot generation: %w", err)
		}

		task := GenerateCoverTask{
			repository:   s.Repository,
			Scene:        *scene,
			ScreenshotAt: at,
			Overwrite:    true,
		}

		task.Start(ctx)

		logger.Infof("Generate screenshot finished")

		// TODO - return error from task
		return nil
	})

	return s.JobManager.Add(ctx, fmt.Sprintf("Generating screenshot for scene id %s", sceneId), j)
}

type AutoTagMetadataInput struct {
	// Paths to tag, null for all files
	Paths []string `json:"paths"`
	// IDs of performers to tag files with, or "*" for all
	Performers []string `json:"performers"`
	// IDs of studios to tag files with, or "*" for all
	Studios []string `json:"studios"`
	// IDs of tags to tag files with, or "*" for all
	Tags []string `json:"tags"`
}

func (s *Manager) AutoTag(ctx context.Context, input AutoTagMetadataInput) int {
	j := autoTagJob{
		repository: s.Repository,
		input:      input,
	}

	return s.JobManager.Add(ctx, "Auto-tagging...", &j)
}

type CleanMetadataInput struct {
	Paths []string `json:"paths"`
	// Do a dry run. Don't delete any files
	DryRun bool `json:"dryRun"`

	IgnoreZipFileContents bool `json:"ignoreZipFileContents"`
}

func (s *Manager) Clean(ctx context.Context, input CleanMetadataInput) int {
	cleaner := &file.Cleaner{
		FS:         &file.OsFS{},
		Repository: file.NewRepository(s.Repository),
		Handlers: []file.CleanHandler{
			&cleanHandler{},
		},
		TrashPath: s.Config.GetDeleteTrashPath(),
	}

	j := cleanJob{
		cleaner:      cleaner,
		repository:   s.Repository,
		sceneService: s.SceneService,
		imageService: s.ImageService,
		input:        input,
		scanSubs:     s.scanSubs,
	}

	return s.JobManager.Add(ctx, "Cleaning...", &j)
}

func (s *Manager) OptimiseDatabase(ctx context.Context) int {
	j := OptimiseDatabaseJob{
		Optimiser: s.Database,
	}

	return s.JobManager.Add(ctx, "Optimising database...", &j)
}

func (s *Manager) MigrateHash(ctx context.Context) int {
	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		fileNamingAlgo := config.GetInstance().GetVideoFileNamingAlgorithm()
		logger.Infof("Migrating generated files for %s naming hash", fileNamingAlgo.String())

		var scenes []*models.Scene
		if err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
			var err error
			scenes, err = s.Repository.Scene.All(ctx)
			return err
		}); err != nil {
			return fmt.Errorf("failed to fetch list of scenes for migration: %w", err)
		}

		var wg sync.WaitGroup
		total := len(scenes)
		progress.SetTotal(total)

		for _, scene := range scenes {
			progress.Increment()
			if job.IsCancelled(ctx) {
				logger.Info("Stopping due to user request")
				return nil
			}

			if scene == nil {
				logger.Errorf("nil scene, skipping migrate")
				continue
			}

			wg.Add(1)

			task := MigrateHashTask{Scene: scene, fileNamingAlgorithm: fileNamingAlgo}
			go func() {
				task.Start()
				wg.Done()
			}()

			wg.Wait()
		}

		logger.Info("Finished migrating")
		return nil
	})

	return s.JobManager.Add(ctx, "Migrating scene hashes...", j)
}

// batchTagType indicates which batch tagging mode to use
type batchTagType int

const (
	batchTagByIds batchTagType = iota
	batchTagByNamesOrStashIds
	batchTagAll
)

// getBatchTagType determines the batch tag mode based on the input
func (input StashBoxBatchTagInput) getBatchTagType(hasPerformerFields bool) batchTagType {
	switch {
	case len(input.Ids) > 0:
		return batchTagByIds
	case hasPerformerFields && len(input.PerformerIds) > 0:
		return batchTagByIds
	case len(input.StashIDs) > 0 || len(input.Names) > 0:
		return batchTagByNamesOrStashIds
	case hasPerformerFields && len(input.PerformerNames) > 0:
		return batchTagByNamesOrStashIds
	default:
		return batchTagAll
	}
}

// Accepts either ids, or a combination of names and stash_ids.
// If none are set, then all existing items will be tagged.
type StashBoxBatchTagInput struct {
	// Stash endpoint to use for the tagging
	//
	// Deprecated: use StashBoxEndpoint
	Endpoint         *int    `json:"endpoint"`
	StashBoxEndpoint *string `json:"stash_box_endpoint"`
	// Fields to exclude when executing the tagging
	ExcludeFields []string `json:"exclude_fields"`
	// Collection fields to merge (add to existing) instead of overwriting when executing the tagging
	MergeFields []string `json:"merge_fields"`
	// Refresh items already tagged by StashBox if true. Only tag items with no StashBox tagging if false
	Refresh bool `json:"refresh"`
	// If batch adding studios or tags, should their parent entities also be created?
	CreateParent bool `json:"createParent"`
	// IDs in stash of the items to update.
	// If set, names and stash_ids fields will be ignored.
	Ids []string `json:"ids"`
	// Names of the items in the stash-box instance to search for and create
	Names []string `json:"names"`
	// Stash IDs of the items in the stash-box instance to search for and create
	StashIDs []string `json:"stash_ids"`
	// IDs in stash of the performers to update
	//
	// Deprecated: use Ids
	PerformerIds []string `json:"performer_ids"`
	// Names of the performers in the stash-box instance to search for and create
	//
	// Deprecated: use Names
	PerformerNames []string `json:"performer_names"`
}

func (s *Manager) batchTagPerformersByIds(ctx context.Context, input StashBoxBatchTagInput, box *models.StashBox) ([]Task, error) {
	var tasks []Task

	err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		performerQuery := s.Repository.Performer

		ids := input.Ids
		if len(ids) == 0 {
			ids = input.PerformerIds //nolint:staticcheck
		}

		for _, performerID := range ids {
			if id, err := strconv.Atoi(performerID); err == nil {
				performer, err := performerQuery.Find(ctx, id)
				if err != nil {
					return err
				}

				if err := performer.LoadStashIDs(ctx, performerQuery); err != nil {
					return fmt.Errorf("loading performer stash ids: %w", err)
				}

				hasStashID := performer.StashIDs.ForEndpoint(box.Endpoint) != nil
				if (input.Refresh && hasStashID) || (!input.Refresh && !hasStashID) {
					tasks = append(tasks, &stashBoxBatchPerformerTagTask{
						performer:      performer,
						box:            box,
						excludedFields: input.ExcludeFields,
						mergeFields:    input.MergeFields,
					})
				}
			}
		}
		return nil
	})

	return tasks, err
}

func (s *Manager) batchTagPerformersByNamesOrStashIds(input StashBoxBatchTagInput, box *models.StashBox) []Task {
	var tasks []Task

	for i := range input.StashIDs {
		stashID := input.StashIDs[i]
		if len(stashID) > 0 {
			tasks = append(tasks, &stashBoxBatchPerformerTagTask{
				stashID:        &stashID,
				box:            box,
				excludedFields: input.ExcludeFields,
				mergeFields:    input.MergeFields,
			})
		}
	}

	names := input.Names
	if len(names) == 0 {
		names = input.PerformerNames //nolint:staticcheck
	}

	for i := range names {
		name := names[i]
		if len(name) > 0 {
			tasks = append(tasks, &stashBoxBatchPerformerTagTask{
				name:           &name,
				box:            box,
				excludedFields: input.ExcludeFields,
				mergeFields:    input.MergeFields,
			})
		}
	}

	return tasks
}

func (s *Manager) batchTagAllPerformers(ctx context.Context, input StashBoxBatchTagInput, box *models.StashBox) ([]Task, error) {
	var tasks []Task

	err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		performerQuery := s.Repository.Performer
		var performers []*models.Performer
		var err error

		performers, err = performerQuery.FindByStashIDStatus(ctx, input.Refresh, box.Endpoint)

		if err != nil {
			return fmt.Errorf("error querying performers: %v", err)
		}

		for _, performer := range performers {
			if err := performer.LoadStashIDs(ctx, performerQuery); err != nil {
				return fmt.Errorf("error loading stash ids for performer %s: %v", performer.Name, err)
			}

			tasks = append(tasks, &stashBoxBatchPerformerTagTask{
				performer:      performer,
				box:            box,
				excludedFields: input.ExcludeFields,
				mergeFields:    input.MergeFields,
			})
		}
		return nil
	})

	return tasks, err
}

func (s *Manager) StashBoxBatchPerformerTag(ctx context.Context, box *models.StashBox, input StashBoxBatchTagInput) int {
	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		logger.Infof("Initiating stash-box batch performer tag")

		var tasks []Task
		var err error

		switch input.getBatchTagType(true) {
		case batchTagByIds:
			tasks, err = s.batchTagPerformersByIds(ctx, input, box)
		case batchTagByNamesOrStashIds:
			tasks = s.batchTagPerformersByNamesOrStashIds(input, box)
		case batchTagAll:
			tasks, err = s.batchTagAllPerformers(ctx, input, box)
		}

		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			return nil
		}

		progress.SetTotal(len(tasks))

		logger.Infof("Starting stash-box batch operation for %d performers", len(tasks))

		for _, task := range tasks {
			progress.ExecuteTask(task.GetDescription(), func() {
				task.Start(ctx)
			})

			progress.Increment()
		}

		return nil
	})

	return s.JobManager.Add(ctx, "Batch stash-box performer tag...", j)
}

func (s *Manager) batchTagStudiosByIds(ctx context.Context, input StashBoxBatchTagInput, box *models.StashBox) ([]Task, error) {
	var tasks []Task

	err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		studioQuery := s.Repository.Studio

		for _, studioID := range input.Ids {
			if id, err := strconv.Atoi(studioID); err == nil {
				studio, err := studioQuery.Find(ctx, id)
				if err != nil {
					return err
				}

				if err := studio.LoadStashIDs(ctx, studioQuery); err != nil {
					return fmt.Errorf("loading studio stash ids: %w", err)
				}

				hasStashID := studio.StashIDs.ForEndpoint(box.Endpoint) != nil
				if (input.Refresh && hasStashID) || (!input.Refresh && !hasStashID) {
					tasks = append(tasks, &stashBoxBatchStudioTagTask{
						studio:         studio,
						createParent:   input.CreateParent,
						box:            box,
						excludedFields: input.ExcludeFields,
					})
				}
			}
		}
		return nil
	})

	return tasks, err
}

func (s *Manager) batchTagStudiosByNamesOrStashIds(input StashBoxBatchTagInput, box *models.StashBox) []Task {
	var tasks []Task

	for i := range input.StashIDs {
		stashID := input.StashIDs[i]
		if len(stashID) > 0 {
			tasks = append(tasks, &stashBoxBatchStudioTagTask{
				stashID:        &stashID,
				createParent:   input.CreateParent,
				box:            box,
				excludedFields: input.ExcludeFields,
			})
		}
	}

	for i := range input.Names {
		name := input.Names[i]
		if len(name) > 0 {
			tasks = append(tasks, &stashBoxBatchStudioTagTask{
				name:           &name,
				createParent:   input.CreateParent,
				box:            box,
				excludedFields: input.ExcludeFields,
			})
		}
	}

	return tasks
}

func (s *Manager) batchTagAllStudios(ctx context.Context, input StashBoxBatchTagInput, box *models.StashBox) ([]Task, error) {
	var tasks []Task

	err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		studioQuery := s.Repository.Studio
		var studios []*models.Studio
		var err error

		studios, err = studioQuery.FindByStashIDStatus(ctx, input.Refresh, box.Endpoint)

		if err != nil {
			return fmt.Errorf("error querying studios: %v", err)
		}

		for _, studio := range studios {
			tasks = append(tasks, &stashBoxBatchStudioTagTask{
				studio:         studio,
				createParent:   input.CreateParent,
				box:            box,
				excludedFields: input.ExcludeFields,
			})
		}
		return nil
	})

	return tasks, err
}

func (s *Manager) StashBoxBatchStudioTag(ctx context.Context, box *models.StashBox, input StashBoxBatchTagInput) int {
	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		logger.Infof("Initiating stash-box batch studio tag")

		var tasks []Task
		var err error

		switch input.getBatchTagType(false) {
		case batchTagByIds:
			tasks, err = s.batchTagStudiosByIds(ctx, input, box)
		case batchTagByNamesOrStashIds:
			tasks = s.batchTagStudiosByNamesOrStashIds(input, box)
		case batchTagAll:
			tasks, err = s.batchTagAllStudios(ctx, input, box)
		}

		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			return nil
		}

		progress.SetTotal(len(tasks))

		logger.Infof("Starting stash-box batch operation for %d studios", len(tasks))

		for _, task := range tasks {
			progress.ExecuteTask(task.GetDescription(), func() {
				task.Start(ctx)
			})

			progress.Increment()
		}

		return nil
	})

	return s.JobManager.Add(ctx, "Batch stash-box studio tag...", j)
}

func (s *Manager) batchTagTagsByIds(ctx context.Context, input StashBoxBatchTagInput, box *models.StashBox) ([]Task, error) {
	var tasks []Task

	err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		tagQuery := s.Repository.Tag

		for _, tagID := range input.Ids {
			if id, err := strconv.Atoi(tagID); err == nil {
				t, err := tagQuery.Find(ctx, id)
				if err != nil {
					return err
				}

				if err := t.LoadStashIDs(ctx, tagQuery); err != nil {
					return fmt.Errorf("loading tag stash ids: %w", err)
				}

				hasStashID := t.StashIDs.ForEndpoint(box.Endpoint) != nil
				if (input.Refresh && hasStashID) || (!input.Refresh && !hasStashID) {
					tasks = append(tasks, &stashBoxBatchTagTagTask{
						tag:            t,
						createParent:   input.CreateParent,
						box:            box,
						excludedFields: input.ExcludeFields,
					})
				}
			}
		}
		return nil
	})

	return tasks, err
}

func (s *Manager) batchTagTagsByNamesOrStashIds(input StashBoxBatchTagInput, box *models.StashBox) []Task {
	var tasks []Task

	for i := range input.StashIDs {
		stashID := input.StashIDs[i]
		if len(stashID) > 0 {
			tasks = append(tasks, &stashBoxBatchTagTagTask{
				stashID:        &stashID,
				createParent:   input.CreateParent,
				box:            box,
				excludedFields: input.ExcludeFields,
			})
		}
	}

	for i := range input.Names {
		name := input.Names[i]
		if len(name) > 0 {
			tasks = append(tasks, &stashBoxBatchTagTagTask{
				name:           &name,
				createParent:   input.CreateParent,
				box:            box,
				excludedFields: input.ExcludeFields,
			})
		}
	}

	return tasks
}

func (s *Manager) batchTagAllTags(ctx context.Context, input StashBoxBatchTagInput, box *models.StashBox) ([]Task, error) {
	var tasks []Task

	err := s.Repository.WithTxn(ctx, func(ctx context.Context) error {
		tagQuery := s.Repository.Tag
		var tags []*models.Tag
		var err error

		tags, err = tagQuery.FindByStashIDStatus(ctx, input.Refresh, box.Endpoint)

		if err != nil {
			return fmt.Errorf("error querying tags: %v", err)
		}

		for _, t := range tags {
			tasks = append(tasks, &stashBoxBatchTagTagTask{
				tag:            t,
				createParent:   input.CreateParent,
				box:            box,
				excludedFields: input.ExcludeFields,
			})
		}
		return nil
	})

	return tasks, err
}

func (s *Manager) StashBoxBatchTagTag(ctx context.Context, box *models.StashBox, input StashBoxBatchTagInput) int {
	j := job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		logger.Infof("Initiating stash-box batch tag tag")

		var tasks []Task
		var err error

		switch input.getBatchTagType(false) {
		case batchTagByIds:
			tasks, err = s.batchTagTagsByIds(ctx, input, box)
		case batchTagByNamesOrStashIds:
			tasks = s.batchTagTagsByNamesOrStashIds(input, box)
		case batchTagAll:
			tasks, err = s.batchTagAllTags(ctx, input, box)
		}

		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			return nil
		}

		progress.SetTotal(len(tasks))

		logger.Infof("Starting stash-box batch operation for %d tags", len(tasks))

		for _, task := range tasks {
			progress.ExecuteTask(task.GetDescription(), func() {
				task.Start(ctx)
			})

			progress.Increment()
		}

		return nil
	})

	return s.JobManager.Add(ctx, "Batch stash-box tag tag...", j)
}
