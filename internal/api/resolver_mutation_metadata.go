package api

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/identify"
	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/internal/manager/task"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
)

func (r *mutationResolver) MetadataScan(ctx context.Context, input manager.ScanMetadataInput) (string, error) {
	jobID, err := manager.GetInstance().Scan(ctx, input)

	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataImport(ctx context.Context) (string, error) {
	jobID, err := manager.GetInstance().Import(ctx)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) ImportObjects(ctx context.Context, input manager.ImportObjectsInput) (string, error) {
	t, err := manager.CreateImportTask(config.GetInstance().GetVideoFileNamingAlgorithm(), input)
	if err != nil {
		return "", err
	}

	jobID := manager.GetInstance().RunSingleTask(ctx, t)

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataExport(ctx context.Context) (string, error) {
	jobID, err := manager.GetInstance().Export(ctx)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) ExportObjects(ctx context.Context, input manager.ExportObjectsInput) (*string, error) {
	t := manager.CreateExportTask(config.GetInstance().GetVideoFileNamingAlgorithm(), input)

	var wg sync.WaitGroup
	wg.Add(1)
	t.Start(ctx, &wg)

	if t.DownloadHash != "" {
		baseURL, _ := ctx.Value(BaseURLCtxKey).(string)

		// generate timestamp
		suffix := time.Now().Format("20060102-150405")
		ret := baseURL + "/downloads/" + t.DownloadHash + "/export" + suffix + ".zip"
		return &ret, nil
	}

	return nil, nil
}

func (r *mutationResolver) MetadataGenerate(ctx context.Context, input manager.GenerateMetadataInput) (string, error) {
	jobID, err := manager.GetInstance().Generate(ctx, input)

	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAutoTag(ctx context.Context, input manager.AutoTagMetadataInput) (string, error) {
	jobID := manager.GetInstance().AutoTag(ctx, input)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAIImageTag(ctx context.Context, input manager.AIImageTagInput) (string, error) {
	jobID, err := manager.GetInstance().AIImageTag(ctx, input)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAIPerformerTag(ctx context.Context, input manager.AIPerformerTagInput) (string, error) {
	jobID, err := manager.GetInstance().AIPerformerTag(ctx, input)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAISceneTag(ctx context.Context, input manager.AISceneTagInput) (string, error) {
	jobID, err := manager.GetInstance().AISceneTag(ctx, input)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataDetectLooping(ctx context.Context, input manager.DetectLoopingInput) (string, error) {
	jobID := manager.GetInstance().DetectLooping(ctx, input)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAITagOrganize(ctx context.Context, input manager.AITagOrganizeInput) (string, error) {
	jobID, err := manager.GetInstance().AITagOrganize(ctx, input)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAIPerformerCluster(ctx context.Context, input AIPerformerClusterInput) (string, error) {
	jobID, err := manager.GetInstance().AIPerformerCluster(ctx, manager.AIPerformerClusterInput{
		PerformerIDs:         input.PerformerIds,
		EntityTypes:          input.EntityTypes,
		MaxMediaPerPerformer: input.MaxMediaPerPerformer,
		MinConfidence:        input.MinConfidence,
		Timeout:              input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAIPerformerMergeSuggest(ctx context.Context, input AIPerformerMergeSuggestInput) (string, error) {
	jobID, err := manager.GetInstance().AIPerformerMergeSuggest(ctx, manager.AIPerformerMergeSuggestInput{
		MaxPerformers: input.MaxPerformers,
		MinConfidence: input.MinConfidence,
		Timeout:       input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) AiAuditApplyAll(ctx context.Context) (int, error) {
	return manager.GetInstance().AIAuditApplyAll(ctx)
}

func (r *mutationResolver) AiAuditRejectAll(ctx context.Context) (int, error) {
	return manager.GetInstance().AIAuditRejectAll(ctx)
}

func (r *mutationResolver) AiTranslationApplyAll(ctx context.Context) (int, error) {
	return manager.GetInstance().AITranslationApplyAll(ctx)
}

func (r *mutationResolver) AiTranslationRejectAll(ctx context.Context) (int, error) {
	return manager.GetInstance().AITranslationRejectAll(ctx)
}

func (r *mutationResolver) AiPerformerSuggestionApplyAll(ctx context.Context) (int, error) {
	return manager.GetInstance().AIPerformerSuggestionApplyAll(ctx)
}

func (r *mutationResolver) AiPerformerSuggestionRejectAll(ctx context.Context) (int, error) {
	return manager.GetInstance().AIPerformerSuggestionRejectAll(ctx)
}

func (r *mutationResolver) AiPerformerCandidateApplyAll(ctx context.Context) (int, error) {
	return manager.GetInstance().AIPerformerCandidateApplyAll(ctx)
}

func (r *mutationResolver) AiPerformerCandidateRejectAll(ctx context.Context) (int, error) {
	return manager.GetInstance().AIPerformerCandidateRejectAll(ctx)
}

func (r *mutationResolver) AiMoodGroupCreate(ctx context.Context, mood string) (string, error) {
	groupID, err := manager.GetInstance().AIMoodGroupCreate(ctx, mood)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(groupID), nil
}

func (r *mutationResolver) MetadataAIMoodTag(ctx context.Context, input AIMoodInput) (string, error) {
	overwrite := false
	if input.Overwrite != nil {
		overwrite = *input.Overwrite
	}

	jobID, err := manager.GetInstance().AIMoodTag(ctx, manager.AIMoodInput{
		MaxScenes: input.MaxScenes,
		Timeout:   input.Timeout,
		Overwrite: overwrite,
	})
	if err != nil {
		return "", err
	}
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) AiSavePlan(ctx context.Context, name string, sceneIDs []string) (string, error) {
	ids := make([]int, len(sceneIDs))
	for i, id := range sceneIDs {
		n, err := strconv.Atoi(id)
		if err != nil {
			return "", fmt.Errorf("converting scene id: %w", err)
		}
		ids[i] = n
	}

	plan := &models.AISavedPlan{Name: name, SceneIDs: ids}
	if err := manager.GetInstance().Repository.WithTxn(ctx, func(ctx context.Context) error {
		return manager.GetInstance().Repository.AISavedPlan.Create(ctx, plan)
	}); err != nil {
		return "", err
	}
	return strconv.FormatInt(plan.ID, 10), nil
}

func (r *mutationResolver) AiDeletePlan(ctx context.Context, planID string) (bool, error) {
	id, err := strconv.ParseInt(planID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting plan id: %w", err)
	}

	if err := manager.GetInstance().Repository.WithTxn(ctx, func(ctx context.Context) error {
		return manager.GetInstance().Repository.AISavedPlan.DeleteByID(ctx, id)
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (r *mutationResolver) AiSaveMoment(ctx context.Context, markerID string) (bool, error) {
	id, err := strconv.Atoi(markerID)
	if err != nil {
		return false, fmt.Errorf("converting marker id: %w", err)
	}

	if err := manager.GetInstance().Repository.WithTxn(ctx, func(ctx context.Context) error {
		return manager.GetInstance().Repository.AISavedMoment.Create(ctx, &models.AISavedMoment{MarkerID: id})
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (r *mutationResolver) AiUnsaveMoment(ctx context.Context, markerID string) (bool, error) {
	id, err := strconv.Atoi(markerID)
	if err != nil {
		return false, fmt.Errorf("converting marker id: %w", err)
	}

	if err := manager.GetInstance().Repository.WithTxn(ctx, func(ctx context.Context) error {
		return manager.GetInstance().Repository.AISavedMoment.DeleteByMarkerID(ctx, id)
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (r *mutationResolver) MetadataGenerateGoonReel(ctx context.Context, sceneIDs []string, durationPerScene *int) (string, error) {
	ids := make([]int, len(sceneIDs))
	for i, id := range sceneIDs {
		n, err := strconv.Atoi(id)
		if err != nil {
			return "", fmt.Errorf("converting scene id: %w", err)
		}
		ids[i] = n
	}

	d := 0
	if durationPerScene != nil {
		d = *durationPerScene
	}

	return manager.GetInstance().GenerateGoonReel(ctx, ids, d)
}

func (r *mutationResolver) MetadataGenerateHighlightClip(ctx context.Context, sceneID string, markerID string, duration *int) (string, error) {
	sid, err := strconv.Atoi(sceneID)
	if err != nil {
		return "", fmt.Errorf("converting scene id: %w", err)
	}
	mid, err := strconv.Atoi(markerID)
	if err != nil {
		return "", fmt.Errorf("converting marker id: %w", err)
	}

	d := 0
	if duration != nil {
		d = *duration
	}

	return manager.GetInstance().GenerateHighlightClip(ctx, sid, mid, d)
}

func (r *mutationResolver) AiPerformerSuggestionApply(ctx context.Context, suggestionID string) (bool, error) {
	id, err := strconv.ParseInt(suggestionID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting suggestion id: %w", err)
	}

	if err := manager.GetInstance().AIPerformerSuggestionApply(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) MetadataAIAudit(ctx context.Context, input AIAuditInput) (string, error) {
	jobID, err := manager.GetInstance().AIAudit(ctx, manager.AIAuditInput{
		EntityTypes: input.EntityTypes,
		MaxItems:    input.MaxItems,
		Timeout:     input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAITranslate(ctx context.Context, input AITranslateInput) (string, error) {
	overwrite := false
	if input.Overwrite != nil {
		overwrite = *input.Overwrite
	}

	jobID, err := manager.GetInstance().AITranslate(ctx, manager.AITranslateInput{
		EntityTypes: input.EntityTypes,
		MaxItems:    input.MaxItems,
		Language:    input.Language,
		Timeout:     input.Timeout,
		Overwrite:   overwrite,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAIPerformerDiscovery(ctx context.Context, input AIPerformerDiscoveryInput) (string, error) {
	jobID, err := manager.GetInstance().AIPerformerDiscovery(ctx, manager.AIPerformerDiscoveryInput{
		MaxScenes:  input.MaxScenes,
		MaxImages:  input.MaxImages,
		MinMatches: input.MinMatches,
		Timeout:    input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) AiPerformerCandidateApply(ctx context.Context, candidateID string, targetPerformerID *string) (bool, error) {
	id, err := strconv.ParseInt(candidateID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting candidate id: %w", err)
	}

	var target *int
	if targetPerformerID != nil && *targetPerformerID != "" {
		t, err := strconv.Atoi(*targetPerformerID)
		if err != nil {
			return false, fmt.Errorf("converting target performer id: %w", err)
		}
		target = &t
	}

	if err := manager.GetInstance().AIPerformerCandidateApply(ctx, id, target); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) AiPerformerCandidateReject(ctx context.Context, candidateID string) (bool, error) {
	id, err := strconv.ParseInt(candidateID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting candidate id: %w", err)
	}

	if err := manager.GetInstance().AIPerformerCandidateReject(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) AiTranslationApply(ctx context.Context, translationID string) (bool, error) {
	id, err := strconv.ParseInt(translationID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting translation id: %w", err)
	}

	if err := manager.GetInstance().AITranslationApply(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) AiTranslationReject(ctx context.Context, translationID string) (bool, error) {
	id, err := strconv.ParseInt(translationID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting translation id: %w", err)
	}

	if err := manager.GetInstance().AITranslationReject(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) AiAuditApply(ctx context.Context, auditID string) (bool, error) {
	id, err := strconv.ParseInt(auditID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting audit id: %w", err)
	}

	if err := manager.GetInstance().AIAuditApply(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) AiAuditReject(ctx context.Context, auditID string) (bool, error) {
	id, err := strconv.ParseInt(auditID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting audit id: %w", err)
	}

	if err := manager.GetInstance().AIAuditReject(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) AiPerformerSuggestionReject(ctx context.Context, suggestionID string) (bool, error) {
	id, err := strconv.ParseInt(suggestionID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting suggestion id: %w", err)
	}

	if err := manager.GetInstance().AIPerformerSuggestionReject(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) MetadataAISceneSegment(ctx context.Context, input AISceneSegmentInput) (string, error) {
	overwrite := false
	if input.Overwrite != nil {
		overwrite = *input.Overwrite
	}

	jobID, err := manager.GetInstance().AISceneSegment(ctx, manager.AISceneSegmentInput{
		SceneIDs:  input.SceneIds,
		MaxScenes: input.MaxScenes,
		Overwrite: overwrite,
		Timeout:   input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAISuggestionGenerate(ctx context.Context, input AISuggestionInput) (string, error) {
	jobID, err := manager.GetInstance().AISuggestionGenerate(ctx, manager.AISuggestionInput{
		EntityTypes: input.EntityTypes,
		MaxItems:    input.MaxItems,
		Timeout:     input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) AiSuggestionApply(ctx context.Context, input AISuggestionApplyInput) (bool, error) {
	id, err := strconv.ParseInt(input.SuggestionID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting suggestion id: %w", err)
	}

	if err := manager.GetInstance().AISuggestionApply(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) AiSuggestionReject(ctx context.Context, suggestionID string) (bool, error) {
	id, err := strconv.ParseInt(suggestionID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting suggestion id: %w", err)
	}

	if err := manager.GetInstance().AISuggestionReject(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) MetadataAIMediaQuality(ctx context.Context, input AIMediaQualityInput) (string, error) {
	overwrite := false
	if input.Overwrite != nil {
		overwrite = *input.Overwrite
	}

	jobID, err := manager.GetInstance().AIMediaQuality(ctx, manager.AIMediaQualityInput{
		EntityTypes: input.EntityTypes,
		MaxItems:    input.MaxItems,
		Overwrite:   overwrite,
		Timeout:     input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAIAudioAnalyze(ctx context.Context, input AIAudioAnalyzeInput) (string, error) {
	overwrite := false
	if input.Overwrite != nil {
		overwrite = *input.Overwrite
	}

	jobID, err := manager.GetInstance().AIAudioAnalyze(ctx, manager.AIAudioAnalyzeInput{
		SceneIDs:  input.SceneIds,
		MaxScenes: input.MaxScenes,
		Overwrite: overwrite,
		Timeout:   input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAIPerformerCareer(ctx context.Context, input AIPerformerCareerInput) (string, error) {
	overwrite := false
	if input.Overwrite != nil {
		overwrite = *input.Overwrite
	}

	jobID, err := manager.GetInstance().AIPerformerCareer(ctx, manager.AIPerformerCareerInput{
		PerformerIDs:  input.PerformerIds,
		MaxPerformers: input.MaxPerformers,
		Overwrite:     overwrite,
		Timeout:       input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAISmartCollections(ctx context.Context, input AISmartCollectionsInput) (string, error) {
	overwrite := false
	if input.Overwrite != nil {
		overwrite = *input.Overwrite
	}

	var outputType *string
	if input.OutputType != nil {
		s := string(*input.OutputType)
		outputType = &s
	}

	jobID, err := manager.GetInstance().AISmartCollections(ctx, manager.AISmartCollectionsInput{
		MaxScenes:      input.MaxScenes,
		MaxImages:      input.MaxImages,
		MaxCollections: input.MaxCollections,
		Overwrite:      overwrite,
		Timeout:        input.Timeout,
		OutputType:     outputType,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataAIFileRename(ctx context.Context, input AIFileRenameInput) (string, error) {
	jobID, err := manager.GetInstance().AIFileRenameGenerate(ctx, manager.AIFileRenameInput{
		SceneIDs:  input.SceneIds,
		ImageIDs:  input.ImageIds,
		MaxItems:  input.MaxItems,
		Overwrite: input.Overwrite != nil && *input.Overwrite,
		Timeout:   input.Timeout,
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) AiFileRenameApply(ctx context.Context, input AIFileRenameApplyInput) (bool, error) {
	id, err := strconv.ParseInt(input.RenameID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting rename id: %w", err)
	}

	if err := manager.GetInstance().AIFileRenameApply(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) AiFileRenameReject(ctx context.Context, renameID string) (bool, error) {
	id, err := strconv.ParseInt(renameID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("converting rename id: %w", err)
	}

	if err := manager.GetInstance().AIFileRenameReject(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) MetadataAIEmbedding(ctx context.Context, input AIEmbeddingInput) (string, error) {
	logger.Infof("MetadataAIEmbedding mutation called with input: %+v", input)
	jobID, err := manager.GetInstance().AIEmbedding(ctx, manager.AIEmbeddingInput{
		EntityTypes: input.EntityTypes,
		Overwrite:   input.Overwrite != nil && *input.Overwrite,
		Visual:      input.Visual != nil && *input.Visual,
	})
	if err != nil {
		logger.Errorf("Failed to create embedding job: %v", err)
		return "", err
	}

	logger.Infof("Embedding job created with ID: %d", jobID)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataIdentify(ctx context.Context, input identify.Options) (string, error) {
	t := manager.CreateIdentifyJob(input)
	jobID := manager.GetInstance().JobManager.Add(ctx, "Identifying...", t)

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataClean(ctx context.Context, input manager.CleanMetadataInput) (string, error) {
	jobID := manager.GetInstance().Clean(ctx, input)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MetadataCleanGenerated(ctx context.Context, input task.CleanGeneratedOptions) (string, error) {
	mgr := manager.GetInstance()
	t := &task.CleanGeneratedJob{
		Options:                  input,
		Paths:                    mgr.Paths,
		BlobsStorageType:         mgr.Config.GetBlobsStorage(),
		VideoFileNamingAlgorithm: mgr.Config.GetVideoFileNamingAlgorithm(),
		Repository:               mgr.Repository,
		BlobCleaner:              mgr.Repository.Blob,
	}
	jobID := mgr.JobManager.Add(ctx, "Cleaning generated files...", t)

	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) MigrateHashNaming(ctx context.Context) (string, error) {
	jobID := manager.GetInstance().MigrateHash(ctx)
	return strconv.Itoa(jobID), nil
}

func (r *mutationResolver) BackupDatabase(ctx context.Context, input BackupDatabaseInput) (*string, error) {
	// if download is true, then backup to temporary file and return a link
	download := input.Download != nil && *input.Download
	includeBlobs := input.IncludeBlobs != nil && *input.IncludeBlobs
	mgr := manager.GetInstance()

	backupPath, backupName, err := mgr.BackupDatabase(download, includeBlobs)
	if err != nil {
		logger.Errorf("Error backing up database: %v", err)
		return nil, err
	}

	if download {
		downloadHash, err := mgr.DownloadStore.RegisterFile(backupPath, "", false)
		if err != nil {
			return nil, fmt.Errorf("error registering file for download: %w", err)
		}
		logger.Debugf("Generated backup file %s with hash %s", backupPath, downloadHash)

		baseURL, _ := ctx.Value(BaseURLCtxKey).(string)

		ret := baseURL + "/downloads/" + downloadHash + "/" + backupName
		return &ret, nil
	} else {
		logger.Infof("Successfully backed up database to: %s", backupPath)
	}

	return nil, nil
}

func (r *mutationResolver) AnonymiseDatabase(ctx context.Context, input AnonymiseDatabaseInput) (*string, error) {
	// if download is true, then save to temporary file and return a link
	download := input.Download != nil && *input.Download
	mgr := manager.GetInstance()

	outPath, outName, err := mgr.AnonymiseDatabase(download)
	if err != nil {
		logger.Errorf("Error anonymising database: %v", err)
		return nil, err
	}

	if download {
		downloadHash, err := mgr.DownloadStore.RegisterFile(outPath, "", false)
		if err != nil {
			return nil, fmt.Errorf("error registering file for download: %w", err)
		}
		logger.Debugf("Generated anonymised file %s with hash %s", outPath, downloadHash)

		baseURL, _ := ctx.Value(BaseURLCtxKey).(string)

		ret := baseURL + "/downloads/" + downloadHash + "/" + outName
		return &ret, nil
	} else {
		logger.Infof("Successfully anonymised database to: %s", outPath)
	}

	return nil, nil
}

func (r *mutationResolver) OptimiseDatabase(ctx context.Context) (string, error) {
	jobID := manager.GetInstance().OptimiseDatabase(ctx)
	return strconv.Itoa(jobID), nil
}
