package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ai"
	"github.com/stashapp/stash/pkg/image"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/scene"
	"github.com/stashapp/stash/pkg/tag"
)

const aiCollectionNamePrefix = "[AI] "

const (
	AIOutputTypeGroups             = "GROUPS"
	AIOutputTypeGalleries          = "GALLERIES"
	AIOutputTypeGroupsAndGalleries = "GROUPS_AND_GALLERIES"
)

type AISmartCollectionsInput struct {
	MaxScenes      *int    `json:"maxScenes"`
	MaxImages      *int    `json:"maxImages"`
	MaxCollections *int    `json:"maxCollections"`
	Overwrite      bool    `json:"overwrite"`
	Timeout        *int    `json:"timeout"`
	OutputType     *string `json:"outputType"`
}

type AISmartCollectionsJob struct {
	input    AISmartCollectionsInput
	progress *job.Progress
}

func CreateAISmartCollectionsJob(input AISmartCollectionsInput) *AISmartCollectionsJob {
	return &AISmartCollectionsJob{
		input: input,
	}
}

func (j *AISmartCollectionsJob) Execute(ctx context.Context, progress *job.Progress) error {
	j.progress = progress

	if !instance.Config.GetAIEnabled() {
		return fmt.Errorf("AI is not enabled. Enable it in Settings > AI")
	}

	client := ai.NewClient(instance.Config.GetAIBaseURL(), instance.Config.GetAIModel())

	// Apply custom timeout if specified
	if j.input.Timeout != nil && *j.input.Timeout > 0 {
		client.SetTimeout(time.Duration(*j.input.Timeout) * time.Second)
	}

	r := instance.Repository

	return r.WithTxn(ctx, func(ctx context.Context) error {
		maxScenes := 0
		if j.input.MaxScenes != nil {
			maxScenes = *j.input.MaxScenes
		}
		maxImages := 0
		if j.input.MaxImages != nil {
			maxImages = *j.input.MaxImages
		}

		sceneTagCounts, imageTagCounts, studioCounts, performerCounts, err := j.gatherLibraryStats(ctx, r, maxScenes, maxImages)
		if err != nil {
			return fmt.Errorf("error gathering library stats: %w", err)
		}

		sceneModel := instance.Config.GetAIEmbeddingModel()
		if sceneModel == "" {
			sceneModel = instance.Config.GetAIModel()
		}
		imageModel := instance.Config.GetAIImageEmbeddingModel()
		if imageModel == "" {
			imageModel = sceneModel
		}

		clusters, err := j.gatherEmbeddingClusters(ctx, r, sceneModel, imageModel)
		if err != nil {
			logger.Warnf("error gathering embedding clusters: %v", err)
		}

		collections, err := j.proposeCollections(ctx, client, r, sceneTagCounts, imageTagCounts, studioCounts, performerCounts, clusters)
		if err != nil {
			return fmt.Errorf("error proposing collections: %w", err)
		}

		if err := j.createCollections(ctx, r, collections, clusters); err != nil {
			return fmt.Errorf("error creating collections: %w", err)
		}

		return nil
	})
}

type libraryTag struct {
	Name  string
	Count int
}

type libraryStudio struct {
	Name  string
	Count int
}

type libraryPerformer struct {
	Name  string
	Count int
}

func (j *AISmartCollectionsJob) gatherLibraryStats(ctx context.Context, r models.Repository, maxScenes, maxImages int) (map[int]int, map[int]int, map[int]int, map[int]int, error) {
	sceneTagCounts := make(map[int]int)
	imageTagCounts := make(map[int]int)
	studioCounts := make(map[int]int)
	performerCounts := make(map[int]int)

	pp := 0
	totalSceneCount, err := r.Scene.QueryCount(ctx, nil, &models.FindFilterType{PerPage: &pp})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error counting scenes: %w", err)
	}
	totalImageCount, err := r.Image.QueryCount(ctx, nil, &models.FindFilterType{PerPage: &pp})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error counting images: %w", err)
	}

	sceneLimit := totalSceneCount
	if maxScenes > 0 && maxScenes < sceneLimit {
		sceneLimit = maxScenes
	}
	imageLimit := totalImageCount
	if maxImages > 0 && maxImages < imageLimit {
		imageLimit = maxImages
	}

	scenes, err := scene.Query(ctx, r.Scene, nil, &models.FindFilterType{PerPage: &sceneLimit})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error querying scenes: %w", err)
	}

	for _, s := range scenes {
		if job.IsCancelled(ctx) {
			return sceneTagCounts, imageTagCounts, studioCounts, performerCounts, nil
		}

		if s.StudioID != nil {
			studioCounts[*s.StudioID]++
		}

		tagIDs, err := r.Scene.GetTagIDs(ctx, s.ID)
		if err == nil {
			for _, tid := range tagIDs {
				sceneTagCounts[tid]++
			}
		}

		performerIDs, err := r.Scene.GetPerformerIDs(ctx, s.ID)
		if err == nil {
			for _, pid := range performerIDs {
				performerCounts[pid]++
			}
		}
	}

	images, err := image.Query(ctx, r.Image, nil, &models.FindFilterType{PerPage: &imageLimit})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error querying images: %w", err)
	}

	for _, img := range images {
		if job.IsCancelled(ctx) {
			return sceneTagCounts, imageTagCounts, studioCounts, performerCounts, nil
		}

		tagIDs, err := r.Image.GetTagIDs(ctx, img.ID)
		if err == nil {
			for _, tid := range tagIDs {
				imageTagCounts[tid]++
			}
		}

		performerIDs, err := r.Image.GetPerformerIDs(ctx, img.ID)
		if err == nil {
			for _, pid := range performerIDs {
				performerCounts[pid]++
			}
		}
	}

	return sceneTagCounts, imageTagCounts, studioCounts, performerCounts, nil
}

// semanticCluster is a summary of a group of semantically similar entities
// derived from their embeddings, used to feed the AI additional context about
// the library's visual/semantic content.
type semanticCluster struct {
	EntityType string
	Size       int
	Tags       []libraryTag
	Performers []libraryPerformer
	Studios    []libraryStudio
	Titles     []string
	// IDs holds every member entity ID of the cluster, used to place entities
	// into collections the AI associates with this cluster.
	IDs []int
}

// gatherEmbeddingClusters clusters scene and image embeddings and summarizes
// each cluster's dominant tags, performers, studios, and sample titles. This
// gives the AI a view of the library's semantic groupings in addition to the
// raw tag/studio/performer statistics. Clustering is best-effort; failures are
// logged and skipped.
func (j *AISmartCollectionsJob) gatherEmbeddingClusters(ctx context.Context, r models.Repository, sceneModel, imageModel string) ([]semanticCluster, error) {
	var clusters []semanticCluster

	sceneClusters, err := j.clusterEntities(ctx, r, entityTypeScene, sceneModel)
	if err != nil {
		logger.Warnf("error clustering scene embeddings: %v", err)
	} else {
		clusters = append(clusters, sceneClusters...)
	}

	imageClusters, err := j.clusterEntities(ctx, r, entityTypeImage, imageModel)
	if err != nil {
		logger.Warnf("error clustering image embeddings: %v", err)
	} else {
		clusters = append(clusters, imageClusters...)
	}

	return clusters, nil
}

// clusterEntities loads the embeddings for a single entity type, clusters them
// with k-means, and produces a summary per cluster. Returns nil when there is
// not enough data to form meaningful clusters.
func (j *AISmartCollectionsJob) clusterEntities(ctx context.Context, r models.Repository, entityType, model string) ([]semanticCluster, error) {
	if model == "" {
		return nil, nil
	}

	all, err := r.Embedding.FindByEntityType(ctx, entityType, model)
	if err != nil {
		return nil, err
	}
	if len(all) < 10 {
		return nil, nil
	}

	k := clusterCount(len(all))
	if k < 2 {
		return nil, nil
	}

	// Cap the number of entities clustered to bound CPU usage on large
	// libraries, sampling uniformly.
	const maxEntities = 20000
	if len(all) > maxEntities {
		all = limitEmbeddingMap(all, maxEntities)
	}

	// Blend tag/studio/performer metadata into the clustering so semantically
	// related entities group together even when their embeddings differ.
	features, err := j.entityMetadataFeatures(ctx, r, entityType, all)
	if err != nil {
		logger.Warnf("error loading metadata features for %s clustering: %v", entityType, err)
	}

	groups := kMeansHybridEmbeddings(all, k, features, 0.5)

	out := make([]semanticCluster, 0, len(groups))
	for _, ids := range groups {
		if job.IsCancelled(ctx) {
			break
		}
		cluster, err := j.describeCluster(ctx, r, entityType, ids)
		if err != nil {
			logger.Warnf("error describing %s cluster: %v", entityType, err)
			continue
		}
		out = append(out, cluster)
	}

	return out, nil
}

// describeCluster builds a summary of a cluster by sampling up to
// clusterSampleSize entities and aggregating their tags, performers, studios,
// and titles. The full member ID list is retained for collection placement.
func (j *AISmartCollectionsJob) describeCluster(ctx context.Context, r models.Repository, entityType string, ids []int) (semanticCluster, error) {
	c := semanticCluster{
		EntityType: entityType,
		Size:       len(ids),
		IDs:        ids,
	}

	const clusterSampleSize = 50
	sample := ids
	if len(sample) > clusterSampleSize {
		sample = sample[:clusterSampleSize]
	}

	tagCounts := make(map[int]int)
	performerCounts := make(map[int]int)
	studioCounts := make(map[int]int)

	titleFor := func(title string, date *models.Date, details string) string {
		if title == "" {
			return ""
		}
		out := title
		if date != nil && !date.IsZero() {
			out += " (" + date.String() + ")"
		}
		if details != "" {
			d := details
			if len(d) > 80 {
				d = d[:80] + "..."
			}
			out += ": " + d
		}
		return out
	}

	for _, id := range sample {
		switch entityType {
		case entityTypeScene:
			s, err := r.Scene.Find(ctx, id)
			if err != nil || s == nil {
				continue
			}
			if len(c.Titles) < 10 {
				if t := titleFor(s.Title, s.Date, s.Details); t != "" {
					c.Titles = append(c.Titles, t)
				}
			}
			if s.StudioID != nil {
				studioCounts[*s.StudioID]++
			}
			tagIDs, _ := r.Scene.GetTagIDs(ctx, s.ID)
			perfIDs, _ := r.Scene.GetPerformerIDs(ctx, s.ID)
			for _, t := range tagIDs {
				tagCounts[t]++
			}
			for _, p := range perfIDs {
				performerCounts[p]++
			}
		case entityTypeImage:
			img, err := r.Image.Find(ctx, id)
			if err != nil || img == nil {
				continue
			}
			if len(c.Titles) < 10 {
				if t := titleFor(img.Title, img.Date, img.Details); t != "" {
					c.Titles = append(c.Titles, t)
				}
			}
			if img.StudioID != nil {
				studioCounts[*img.StudioID]++
			}
			tagIDs, _ := r.Image.GetTagIDs(ctx, img.ID)
			perfIDs, _ := r.Image.GetPerformerIDs(ctx, img.ID)
			for _, t := range tagIDs {
				tagCounts[t]++
			}
			for _, p := range perfIDs {
				performerCounts[p]++
			}
		}
	}

	c.Tags = resolveTopTags(ctx, r, tagCounts, 5)
	c.Performers = resolveTopPerformers(ctx, r, performerCounts, 5)
	c.Studios = resolveTopStudios(ctx, r, studioCounts, 3)

	return c, nil
}

func resolveTopTags(ctx context.Context, r models.Repository, counts map[int]int, limit int) []libraryTag {
	ids := topByCount(counts, limit)
	if len(ids) == 0 {
		return nil
	}
	tags, err := r.Tag.FindMany(ctx, ids)
	if err != nil {
		return nil
	}
	out := make([]libraryTag, 0, len(tags))
	for _, t := range tags {
		if t == nil {
			continue
		}
		out = append(out, libraryTag{Name: t.Name, Count: counts[t.ID]})
	}
	return out
}

func resolveTopPerformers(ctx context.Context, r models.Repository, counts map[int]int, limit int) []libraryPerformer {
	ids := topByCount(counts, limit)
	if len(ids) == 0 {
		return nil
	}
	performers, err := r.Performer.FindMany(ctx, ids)
	if err != nil {
		return nil
	}
	out := make([]libraryPerformer, 0, len(performers))
	for _, p := range performers {
		if p == nil {
			continue
		}
		out = append(out, libraryPerformer{Name: p.Name, Count: counts[p.ID]})
	}
	return out
}

func resolveTopStudios(ctx context.Context, r models.Repository, counts map[int]int, limit int) []libraryStudio {
	ids := topByCount(counts, limit)
	if len(ids) == 0 {
		return nil
	}
	studios, err := r.Studio.FindMany(ctx, ids)
	if err != nil {
		return nil
	}
	out := make([]libraryStudio, 0, len(studios))
	for _, s := range studios {
		if s == nil {
			continue
		}
		out = append(out, libraryStudio{Name: s.Name, Count: counts[s.ID]})
	}
	return out
}

// entityMetadataFeatures loads the tag, performer, and studio IDs associated
// with each entity as a feature set, used to blend metadata similarity into
// embedding clustering.
func (j *AISmartCollectionsJob) entityMetadataFeatures(ctx context.Context, r models.Repository, entityType string, entities map[int][]float32) (map[int]map[int]bool, error) {
	features := make(map[int]map[int]bool, len(entities))

	for id := range entities {
		var tagIDs, perfIDs []int
		var studioID *int

		switch entityType {
		case entityTypeScene:
			s, err := r.Scene.Find(ctx, id)
			if err != nil || s == nil {
				continue
			}
			studioID = s.StudioID
			tagIDs, _ = r.Scene.GetTagIDs(ctx, s.ID)
			perfIDs, _ = r.Scene.GetPerformerIDs(ctx, s.ID)
		case entityTypeImage:
			img, err := r.Image.Find(ctx, id)
			if err != nil || img == nil {
				continue
			}
			studioID = img.StudioID
			tagIDs, _ = r.Image.GetTagIDs(ctx, img.ID)
			perfIDs, _ = r.Image.GetPerformerIDs(ctx, img.ID)
		default:
			return features, nil
		}

		set := make(map[int]bool, len(tagIDs)+len(perfIDs)+1)
		for _, t := range tagIDs {
			set[t] = true
		}
		for _, p := range perfIDs {
			set[p] = true
		}
		if studioID != nil {
			set[*studioID] = true
		}
		if len(set) > 0 {
			features[id] = set
		}
	}

	return features, nil
}

// clusterCount returns a reasonable number of clusters for n entities, between
// 2 and 8, so that clusters are large enough to be meaningful.
func clusterCount(n int) int {
	if n < 10 {
		return 0
	}
	k := n / 50
	if k < 2 {
		k = 2
	}
	if k > 8 {
		k = 8
	}
	return k
}

// kMeansEmbeddings partitions the given embeddings into k clusters using a
// simple k-means with deterministic, norm-spread seeding. Returns the clusters
// as slices of entity IDs. Vectors of different lengths are ignored.
func kMeansEmbeddings(embeddings map[int][]float32, k int) [][]int {
	return kMeansHybridEmbeddings(embeddings, k, nil, 0.5)
}

// kMeansHybridEmbeddings partitions embeddings into k clusters, optionally
// blending metadata similarity into the distance metric. When features is nil,
// plain euclidean k-means on the raw vectors is used. When features is
// provided, the vectors are normalized to unit length (spherical k-means) and
// the distance becomes euclidean(unit vectors) + lambda * jaccardDistance of
// the feature sets, so entities sharing tags/performers/studios cluster
// together even when their embeddings differ.
func kMeansHybridEmbeddings(embeddings map[int][]float32, k int, features map[int]map[int]bool, lambda float64) [][]int {
	type entry struct {
		id   int
		vec  []float32
		norm float64
	}

	useHybrid := features != nil

	entries := make([]entry, 0, len(embeddings))
	for id, v := range embeddings {
		if len(v) == 0 {
			continue
		}
		if useHybrid {
			norm := vecNorm(v)
			if norm == 0 {
				continue
			}
			normalized := make([]float32, len(v))
			for i, f := range v {
				normalized[i] = float32(float64(f) / norm)
			}
			v = normalized
		}
		entries = append(entries, entry{id: id, vec: v, norm: vecNorm(v)})
	}
	if len(entries) < k {
		return nil
	}

	dim := len(entries[0].vec)

	// deterministic seeding: sort by magnitude (ties broken by id) and pick
	// evenly spread seeds
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].norm != entries[j].norm {
			return entries[i].norm < entries[j].norm
		}
		return entries[i].id < entries[j].id
	})

	centroids := make([][]float32, k)
	centroidFeatures := make([]map[int]bool, k)
	for i := 0; i < k; i++ {
		centroids[i] = entries[i*len(entries)/k].vec
		if useHybrid {
			centroidFeatures[i] = features[entries[i*len(entries)/k].id]
		}
	}

	assignments := make([]int, len(entries))
	const maxIterations = 20
	for iter := 0; iter < maxIterations; iter++ {
		changed := false
		for i, e := range entries {
			best := 0
			bestDist := hybridEntityDistance(e.vec, centroids[0], features[e.id], centroidFeatures[0], dim, lambda)
			for c := 1; c < k; c++ {
				d := hybridEntityDistance(e.vec, centroids[c], features[e.id], centroidFeatures[c], dim, lambda)
				if d < bestDist {
					best = c
					bestDist = d
				}
			}
			if assignments[i] != best {
				assignments[i] = best
				changed = true
			}
		}
		if !changed {
			break
		}

		// recompute centroids
		sums := make([][]float64, k)
		counts := make([]int, k)
		featureCounts := make([]map[int]int, k)
		for i := range sums {
			sums[i] = make([]float64, dim)
			if useHybrid {
				featureCounts[i] = make(map[int]int)
			}
		}
		for i, e := range entries {
			c := assignments[i]
			counts[c]++
			for d := 0; d < dim; d++ {
				sums[c][d] += float64(e.vec[d])
			}
			if useHybrid {
				for f := range features[e.id] {
					featureCounts[c][f]++
				}
			}
		}
		for c := 0; c < k; c++ {
			if counts[c] > 0 {
				for d := 0; d < dim; d++ {
					centroids[c][d] = float32(sums[c][d] / float64(counts[c]))
				}
				if useHybrid {
					centroidFeatures[c] = make(map[int]bool, len(featureCounts[c]))
					for f := range featureCounts[c] {
						centroidFeatures[c][f] = true
					}
				}
			}
		}
	}

	groups := make([][]int, k)
	for i, e := range entries {
		groups[assignments[i]] = append(groups[assignments[i]], e.id)
	}

	out := groups[:0]
	for _, g := range groups {
		if len(g) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// hybridEntityDistance computes the distance between an entity and a centroid.
// With metadata features provided, the metadata Jaccard distance is blended in
// with the given weight.
func hybridEntityDistance(vec, centroid []float32, features, centroidFeatures map[int]bool, dim int, lambda float64) float64 {
	d := distanceSquared(vec, centroid, dim)
	if features != nil && centroidFeatures != nil {
		d += lambda * jaccardDistance(features, centroidFeatures)
	}
	return d
}

// jaccardDistance returns 1 - jaccard(a, b). Returns 0 when both sets are
// equal or both empty, and 1 when they are disjoint.
func jaccardDistance(a, b map[int]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}

	inter := 0
	union := len(a)
	for k := range a {
		if b[k] {
			inter++
		}
	}
	for k := range b {
		if !a[k] {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return 1 - float64(inter)/float64(union)
}

func vecNorm(v []float32) float64 {
	var sum float64
	for _, f := range v {
		sum += float64(f) * float64(f)
	}
	return math.Sqrt(sum)
}

func distanceSquared(a, b []float32, dim int) float64 {
	if len(a) < dim || len(b) < dim {
		return math.MaxFloat64
	}
	var sum float64
	for d := 0; d < dim; d++ {
		diff := float64(a[d]) - float64(b[d])
		sum += diff * diff
	}
	return sum
}

// limitEmbeddingMap returns a map containing at most maxEntries entries from
// the input map.
func limitEmbeddingMap(m map[int][]float32, maxEntries int) map[int][]float32 {
	if len(m) <= maxEntries {
		return m
	}
	result := make(map[int][]float32, maxEntries)
	count := 0
	for k, v := range m {
		if count >= maxEntries {
			break
		}
		result[k] = v
		count++
	}
	return result
}

func (j *AISmartCollectionsJob) proposeCollections(ctx context.Context, client *ai.Client, r models.Repository, sceneTagCounts, imageTagCounts, studioCounts, performerCounts map[int]int, clusters []semanticCluster) ([]aiCollectionProposal, error) {
	topSceneTagIDs := topByCount(sceneTagCounts, 40)
	topImageTagIDs := topByCount(imageTagCounts, 40)
	topStudioIDs := topByCount(studioCounts, 10)
	topPerformerIDs := topByCount(performerCounts, 20)

	sceneTags := make([]libraryTag, 0, len(topSceneTagIDs))
	for _, id := range topSceneTagIDs {
		t, err := r.Tag.Find(ctx, id)
		if err != nil || t == nil {
			continue
		}
		sceneTags = append(sceneTags, libraryTag{Name: t.Name, Count: sceneTagCounts[id]})
	}

	imageTags := make([]libraryTag, 0, len(topImageTagIDs))
	for _, id := range topImageTagIDs {
		t, err := r.Tag.Find(ctx, id)
		if err != nil || t == nil {
			continue
		}
		imageTags = append(imageTags, libraryTag{Name: t.Name, Count: imageTagCounts[id]})
	}

	studios := make([]libraryStudio, 0, len(topStudioIDs))
	for _, id := range topStudioIDs {
		s, err := r.Studio.Find(ctx, id)
		if err != nil || s == nil {
			continue
		}
		studios = append(studios, libraryStudio{Name: s.Name, Count: studioCounts[id]})
	}

	performers := make([]libraryPerformer, 0, len(topPerformerIDs))
	for _, id := range topPerformerIDs {
		p, err := r.Performer.Find(ctx, id)
		if err != nil || p == nil {
			continue
		}
		performers = append(performers, libraryPerformer{Name: p.Name, Count: performerCounts[id]})
	}

	outputType := AIOutputTypeGroupsAndGalleries
	if j.input.OutputType != nil {
		outputType = *j.input.OutputType
	}

	// Only require tag data for the failure cases; performers, studios, and
	// clusters are additional context.
	switch outputType {
	case AIOutputTypeGroups:
		if len(sceneTags) == 0 {
			return nil, fmt.Errorf("no tagged scenes found in library")
		}
	case AIOutputTypeGalleries:
		if len(imageTags) == 0 {
			return nil, fmt.Errorf("no tagged images found in library")
		}
	default:
		if len(sceneTags) == 0 && len(imageTags) == 0 {
			return nil, fmt.Errorf("no tagged scenes or images found in library")
		}
	}

	sceneTagLines := make([]string, len(sceneTags))
	for i, t := range sceneTags {
		sceneTagLines[i] = fmt.Sprintf("%s (%d)", t.Name, t.Count)
	}
	imageTagLines := make([]string, len(imageTags))
	for i, t := range imageTags {
		imageTagLines[i] = fmt.Sprintf("%s (%d)", t.Name, t.Count)
	}
	studioLines := make([]string, len(studios))
	for i, s := range studios {
		studioLines[i] = fmt.Sprintf("%s (%d)", s.Name, s.Count)
	}
	performerLines := make([]string, len(performers))
	for i, p := range performers {
		performerLines[i] = fmt.Sprintf("%s (%d)", p.Name, p.Count)
	}
	clusterLines := buildClusterLines(clusters)

	maxCollections := 10
	if j.input.MaxCollections != nil && *j.input.MaxCollections > 0 {
		maxCollections = *j.input.MaxCollections
	}

	systemPrompt, userPrompt := j.buildPrompts(outputType, strings.Join(sceneTagLines, ", "), strings.Join(imageTagLines, ", "), strings.Join(studioLines, ", "), strings.Join(performerLines, ", "), strings.Join(clusterLines, "\n"), maxCollections)

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := client.ChatCompletion(ctx, ai.ChatCompletionRequest{Messages: messages})
	if err != nil {
		return nil, fmt.Errorf("proposal request failed: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content type from AI response")
	}

	cleanJSON := ai.ExtractJSON(content)
	if cleanJSON == "" {
		return nil, fmt.Errorf("parsing AI response: no JSON found")
	}

	var proposals []aiCollectionProposal
	if err := json.Unmarshal([]byte(cleanJSON), &proposals); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}

	return proposals, nil
}

// buildClusterLines renders semantic cluster summaries as human-readable lines
// for inclusion in the AI prompt. Each line is prefixed with its 1-based index
// so the AI can reference the cluster in its proposals.
func buildClusterLines(clusters []semanticCluster) []string {
	lines := make([]string, 0, len(clusters))
	for i, c := range clusters {
		var b strings.Builder
		fmt.Fprintf(&b, "[%d] %s cluster with %d entities", i+1, c.EntityType, c.Size)
		if len(c.Tags) > 0 {
			names := make([]string, len(c.Tags))
			for i, t := range c.Tags {
				names[i] = fmt.Sprintf("%s (%d)", t.Name, t.Count)
			}
			fmt.Fprintf(&b, "; tags: %s", strings.Join(names, ", "))
		}
		if len(c.Performers) > 0 {
			names := make([]string, len(c.Performers))
			for i, p := range c.Performers {
				names[i] = fmt.Sprintf("%s (%d)", p.Name, p.Count)
			}
			fmt.Fprintf(&b, "; performers: %s", strings.Join(names, ", "))
		}
		if len(c.Studios) > 0 {
			names := make([]string, len(c.Studios))
			for i, s := range c.Studios {
				names[i] = fmt.Sprintf("%s (%d)", s.Name, s.Count)
			}
			fmt.Fprintf(&b, "; studios: %s", strings.Join(names, ", "))
		}
		if len(c.Titles) > 0 {
			fmt.Fprintf(&b, "; examples: %s", strings.Join(c.Titles, " | "))
		}
		lines = append(lines, b.String())
	}
	return lines
}

func (j *AISmartCollectionsJob) buildPrompts(outputType string, sceneTagLines, imageTagLines, studioLines, performerLines, clusterLines string, maxCollections int) (string, string) {
	switch outputType {
	case AIOutputTypeGroups:
		return "You are a librarian for an adult media library. You propose useful groups (collections of scenes) from tag statistics, performers, and semantic content clusters.",
			fmt.Sprintf(`The user's adult media library has these most common scene tags:
%s

These most common studios:
%s

These most common performers:
%s

Semantic clusters of scenes (visually or textually similar groups):
%s

Propose up to %d groups that group scenes by common theme, category, niche, studio, or performer. Groups should be distinct, meaningful, and cover a broad range of the library. Each group must have:
- "name": a short, descriptive name (max 4 words)
- "description": a 1 sentence description of what the group contains
- "tags": an array of 2-8 tag names from the lists above that define the group
- optionally "performers": an array of 1-3 performer names from the lists above (omit if the group is not performer-based)
- optionally "cluster": the 1-based number of a semantic cluster from the numbered cluster lists above whose members should be added to this group. Pick the cluster that best matches the group's theme; omit it for groups defined by tags, performers, or studios.

Return ONLY valid JSON as an array of objects, e.g.:
[{"name":"Outdoor Scenes","description":"Scenes filmed outdoors","tags":["Outdoor","Sunny"],"cluster":2}]

Use EXACT tag and performer names from the lists above. Return ONLY valid JSON, no other text.`,
				sceneTagLines, studioLines, performerLines, clusterLines, maxCollections)

	case AIOutputTypeGalleries:
		return "You are a librarian for an adult media library. You propose useful galleries (collections of images) from tag statistics, performers, and semantic content clusters.",
			fmt.Sprintf(`The user's adult media library has these most common image tags:
%s

These most common studios:
%s

These most common performers:
%s

Semantic clusters of images (visually or textually similar groups):
%s

Propose up to %d galleries that group images by common theme, category, niche, studio, or performer. Galleries should be distinct, meaningful, and cover a broad range of the library. Each gallery must have:
- "name": a short, descriptive name (max 4 words)
- "description": a 1 sentence description of what the gallery contains
- "tags": an array of 2-8 tag names from the lists above that define the gallery
- optionally "performers": an array of 1-3 performer names from the lists above (omit if the gallery is not performer-based)
- optionally "cluster": the 1-based number of a semantic cluster from the numbered cluster lists above whose members should be added to this gallery. Pick the cluster that best matches the gallery's theme; omit it for galleries defined by tags, performers, or studios.

Return ONLY valid JSON as an array of objects, e.g.:
[{"name":"Outdoor Images","description":"Images shot outdoors","tags":["Outdoor","Sunny"],"cluster":1}]

Use EXACT tag and performer names from the lists above. Return ONLY valid JSON, no other text.`,
				imageTagLines, studioLines, performerLines, clusterLines, maxCollections)

	default: // GROUPS_AND_GALLERIES
		return "You are a librarian for an adult media library. You propose useful groups (collections of scenes) and galleries (collections of images) from tag statistics, performers, and semantic content clusters.",
			fmt.Sprintf(`The user's adult media library has these most common scene tags:
%s

These most common image tags:
%s

These most common studios:
%s

These most common performers:
%s

Semantic clusters of scenes and images (visually or textually similar groups):
%s

Propose up to %d collections that group scenes and images by common theme, category, niche, studio, or performer. Each collection must have:
- "name": a short, descriptive name (max 4 words)
- "description": a 1 sentence description of what the collection contains
- "tags": an array of 2-8 tag names from the lists above that define the collection
- optionally "performers": an array of 1-3 performer names from the lists above (omit if the collection is not performer-based)
- optionally "cluster": the 1-based number of a semantic cluster from the numbered cluster lists above whose members should be added to this collection. Pick the cluster that best matches the collection's theme; omit it for collections defined by tags, performers, or studios.

Return ONLY valid JSON as an array of objects, e.g.:
[{"name":"Outdoor Content","description":"Scenes and images filmed outdoors","tags":["Outdoor","Sunny"],"cluster":2}]

Use EXACT tag and performer names from the lists above. Return ONLY valid JSON, no other text.`,
				sceneTagLines, imageTagLines, studioLines, performerLines, clusterLines, maxCollections)
	}
}

type aiCollectionProposal struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Performers  []string `json:"performers,omitempty"`
	Studios     []string `json:"studios,omitempty"`
	// Cluster is the 1-based index of a semantic cluster (from the numbered
	// cluster lists in the prompt) whose members should be added to this
	// collection, in addition to any tag/performer/studio criteria.
	Cluster *int `json:"cluster,omitempty"`
}

func (j *AISmartCollectionsJob) createCollections(ctx context.Context, r models.Repository, proposals []aiCollectionProposal, clusters []semanticCluster) error {
	outputType := AIOutputTypeGroupsAndGalleries
	if j.input.OutputType != nil {
		outputType = *j.input.OutputType
	}

	// Load existing collections based on output type
	var existingGroups []*models.Group
	var existingGalleries []*models.Gallery
	var existingGroupNames, existingGalleryNames map[string]bool
	var err error

	if outputType == AIOutputTypeGroups || outputType == AIOutputTypeGroupsAndGalleries {
		existingGroups, _, err = r.Group.Query(ctx, nil, &models.FindFilterType{PerPage: newInt(0)})
		if err != nil {
			return fmt.Errorf("error loading existing groups: %w", err)
		}
		existingGroupNames = make(map[string]bool, len(existingGroups))
		for _, g := range existingGroups {
			existingGroupNames[g.Name] = true
		}
	}

	if outputType == AIOutputTypeGalleries || outputType == AIOutputTypeGroupsAndGalleries {
		existingGalleries, _, err = r.Gallery.Query(ctx, nil, &models.FindFilterType{PerPage: newInt(0)})
		if err != nil {
			return fmt.Errorf("error loading existing galleries: %w", err)
		}
		existingGalleryNames = make(map[string]bool, len(existingGalleries))
		for _, g := range existingGalleries {
			existingGalleryNames[g.Title] = true
		}
	}

	for _, p := range proposals {
		if job.IsCancelled(ctx) {
			return nil
		}

		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		fullName := aiCollectionNamePrefix + name

		// Find or create tags
		tagIDs := make([]int, 0, len(p.Tags))
		for _, tagName := range p.Tags {
			tagName = strings.TrimSpace(tagName)
			if tagName == "" {
				continue
			}
			t, err := tag.ByName(ctx, r.Tag, tagName)
			if err != nil {
				logger.Errorf("Error finding tag %q: %v", tagName, err)
				continue
			}
			if t == nil {
				newTag := models.NewTag()
				newTag.Name = tagName
				if err := r.Tag.Create(ctx, &models.CreateTagInput{Tag: &newTag}); err != nil {
					logger.Errorf("Error creating tag %q: %v", tagName, err)
					continue
				}
				t = &newTag
			}
			tagIDs = append(tagIDs, t.ID)
		}

		performerIDs := resolvePerformerIDs(ctx, r, p.Performers)
		studioIDs := resolveStudioIDs(ctx, r, p.Studios)
		clusterSceneIDs, clusterImageIDs := clusterIDsForCollection(clusters, p.Cluster)

		if len(tagIDs) == 0 && len(performerIDs) == 0 && len(studioIDs) == 0 && len(clusterSceneIDs) == 0 && len(clusterImageIDs) == 0 {
			continue
		}

		// Handle based on output type
		switch outputType {
		case AIOutputTypeGroups:
			if err := j.createGroup(ctx, r, fullName, p.Description, tagIDs, performerIDs, studioIDs, clusterSceneIDs); err != nil {
				logger.Errorf("Error creating group %q: %v", fullName, err)
			}

		case AIOutputTypeGalleries:
			if err := j.createGallery(ctx, r, fullName, p.Description, tagIDs, performerIDs, studioIDs, clusterImageIDs); err != nil {
				logger.Errorf("Error creating gallery %q: %v", fullName, err)
			}

		case AIOutputTypeGroupsAndGalleries:
			if err := j.createGroup(ctx, r, fullName, p.Description, tagIDs, performerIDs, studioIDs, clusterSceneIDs); err != nil {
				logger.Errorf("Error creating group %q: %v", fullName, err)
			}
			if err := j.createGallery(ctx, r, fullName, p.Description, tagIDs, performerIDs, studioIDs, clusterImageIDs); err != nil {
				logger.Errorf("Error creating gallery %q: %v", fullName, err)
			}

		default: // GROUPS_AND_GALLERIES
			if err := j.createGroup(ctx, r, fullName, p.Description, tagIDs, performerIDs, studioIDs, clusterSceneIDs); err != nil {
				logger.Errorf("Error creating group %q: %v", fullName, err)
			}
			if err := j.createGallery(ctx, r, fullName, p.Description, tagIDs, performerIDs, studioIDs, clusterImageIDs); err != nil {
				logger.Errorf("Error creating gallery %q: %v", fullName, err)
			}
		}
	}

	return nil
}

// clusterIDsForCollection resolves the member IDs of the 1-based semantic
// cluster referenced by a proposal. Scene clusters contribute scene IDs,
// image clusters contribute image IDs. Returns nil for missing or out-of-range
// indices.
func clusterIDsForCollection(clusters []semanticCluster, clusterIdx *int) ([]int, []int) {
	if clusterIdx == nil {
		return nil, nil
	}
	idx := *clusterIdx - 1
	if idx < 0 || idx >= len(clusters) {
		return nil, nil
	}
	c := clusters[idx]
	switch c.EntityType {
	case entityTypeScene:
		return c.IDs, nil
	case entityTypeImage:
		return nil, c.IDs
	}
	return nil, nil
}

// resolvePerformerIDs looks up performers by exact (case-insensitive) name and
// returns their IDs. Unmatched names are skipped.
func resolvePerformerIDs(ctx context.Context, r models.Repository, names []string) []int {
	var ids []int
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		performers, err := r.Performer.FindByNames(ctx, []string{n}, true)
		if err != nil || len(performers) == 0 || performers[0] == nil {
			continue
		}
		ids = append(ids, performers[0].ID)
	}
	return ids
}

// resolveStudioIDs looks up studios by exact (case-insensitive) name and
// returns their IDs. Unmatched names are skipped.
func resolveStudioIDs(ctx context.Context, r models.Repository, names []string) []int {
	var ids []int
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		s, err := r.Studio.FindByName(ctx, n, true)
		if err != nil || s == nil {
			continue
		}
		ids = append(ids, s.ID)
	}
	return ids
}

func (j *AISmartCollectionsJob) createGroup(ctx context.Context, r models.Repository, name, description string, tagIDs, performerIDs, studioIDs, clusterSceneIDs []int) error {
	// Check if group already exists
	existingGroup, err := r.Group.FindByName(ctx, name, true)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}

	var group *models.Group
	if existingGroup != nil {
		if !j.input.Overwrite {
			return nil // skip if not overwriting
		}
		group = existingGroup
		group.Name = name
		if description != "" {
			group.Synopsis = description
		}
		if err := r.Group.Update(ctx, group); err != nil {
			return err
		}
	} else {
		newGroup := models.NewGroup()
		group = &newGroup
		group.Name = name
		if description != "" {
			group.Synopsis = description
		}
		if err := r.Group.Create(ctx, group); err != nil {
			return err
		}
	}

	// Add tags to group using UpdatePartial
	if len(tagIDs) > 0 {
		tagUpdate := &models.UpdateIDs{
			IDs:  tagIDs,
			Mode: models.RelationshipUpdateModeAdd,
		}
		partial := models.GroupPartial{
			TagIDs: tagUpdate,
		}
		if _, err := r.Group.UpdatePartial(ctx, group.ID, partial); err != nil {
			logger.Errorf("Error adding tags to group %q: %v", name, err)
		}
	}

	// Query scenes matching the tags, performers, or studios and add to group
	sceneIDs := sceneIDsForCollection(ctx, r, tagIDs, performerIDs, studioIDs)
	sceneIDs = uniqueInts(append(sceneIDs, clusterSceneIDs...))

	if len(sceneIDs) > 0 {
		// Add each scene to the group using UpdatePartial with GroupIDs in Add mode
		groupUpdate := &models.UpdateGroupIDs{
			Groups: []models.GroupsScenes{{GroupID: group.ID}},
			Mode:   models.RelationshipUpdateModeAdd,
		}
		for _, sceneID := range sceneIDs {
			partial := models.ScenePartial{
				GroupIDs: groupUpdate,
			}
			if _, err := r.Scene.UpdatePartial(ctx, sceneID, partial); err != nil {
				logger.Errorf("Error adding scene %d to group %q: %v", sceneID, name, err)
			}
		}
	}

	return nil
}

func (j *AISmartCollectionsJob) createGallery(ctx context.Context, r models.Repository, name, description string, tagIDs, performerIDs, studioIDs, clusterImageIDs []int) error {
	// Check if gallery already exists
	existingGalleries, err := r.Gallery.FindUserGalleryByTitle(ctx, name)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}

	var gallery *models.Gallery
	if len(existingGalleries) > 0 {
		if !j.input.Overwrite {
			return nil // skip if not overwriting
		}
		gallery = existingGalleries[0]
		gallery.Title = name
		if description != "" {
			gallery.Details = description
		}
		updateInput := &models.UpdateGalleryInput{
			Gallery: gallery,
		}
		if err := r.Gallery.Update(ctx, updateInput); err != nil {
			return err
		}
	} else {
		newGallery := models.NewGallery()
		gallery = &newGallery
		gallery.Title = name
		if description != "" {
			gallery.Details = description
		}
		createInput := &models.CreateGalleryInput{
			Gallery: gallery,
		}
		if err := r.Gallery.Create(ctx, createInput); err != nil {
			return err
		}
	}

	// Add tags to gallery using UpdatePartial
	if len(tagIDs) > 0 {
		tagUpdate := &models.UpdateIDs{
			IDs:  tagIDs,
			Mode: models.RelationshipUpdateModeAdd,
		}
		partial := models.GalleryPartial{
			TagIDs: tagUpdate,
		}
		if _, err := r.Gallery.UpdatePartial(ctx, gallery.ID, partial); err != nil {
			logger.Errorf("Error adding tags to gallery %q: %v", name, err)
		}
	}

	// Query images matching the tags, performers, or studios and add to gallery
	imageIDs := imageIDsForCollection(ctx, r, tagIDs, performerIDs, studioIDs)
	imageIDs = uniqueInts(append(imageIDs, clusterImageIDs...))

	if len(imageIDs) > 0 {
		// Add images to gallery
		if err := r.Gallery.AddImages(ctx, gallery.ID, imageIDs...); err != nil {
			logger.Errorf("Error adding images to gallery %q: %v", name, err)
		}
	}

	return nil
}

// sceneIDsForCollection queries scenes matching any of the given tags,
// performers, or studios and returns the union of their IDs. The criteria are
// combined with OR semantics so collections defined by performers or studios
// populate correctly.
func sceneIDsForCollection(ctx context.Context, r models.Repository, tagIDs, performerIDs, studioIDs []int) []int {
	var ids []int

	if len(tagIDs) > 0 {
		ids = append(ids, querySceneIDs(ctx, r, &models.SceneFilterType{
			Tags: &models.HierarchicalMultiCriterionInput{
				Value:    intStrs(tagIDs),
				Modifier: "INCLUDES",
				Depth:    newInt(0),
			},
		})...)
	}

	if len(performerIDs) > 0 {
		ids = append(ids, querySceneIDs(ctx, r, &models.SceneFilterType{
			Performers: &models.MultiCriterionInput{
				Value:    intStrs(performerIDs),
				Modifier: "INCLUDES",
			},
		})...)
	}

	if len(studioIDs) > 0 {
		ids = append(ids, querySceneIDs(ctx, r, &models.SceneFilterType{
			Studios: &models.HierarchicalMultiCriterionInput{
				Value:    intStrs(studioIDs),
				Modifier: "INCLUDES",
				Depth:    newInt(0),
			},
		})...)
	}

	return uniqueInts(ids)
}

// imageIDsForCollection queries images matching any of the given tags,
// performers, or studios and returns the union of their IDs.
func imageIDsForCollection(ctx context.Context, r models.Repository, tagIDs, performerIDs, studioIDs []int) []int {
	var ids []int

	if len(tagIDs) > 0 {
		ids = append(ids, queryImageIDs(ctx, r, &models.ImageFilterType{
			Tags: &models.HierarchicalMultiCriterionInput{
				Value:    intStrs(tagIDs),
				Modifier: "INCLUDES",
				Depth:    newInt(0),
			},
		})...)
	}

	if len(performerIDs) > 0 {
		ids = append(ids, queryImageIDs(ctx, r, &models.ImageFilterType{
			Performers: &models.MultiCriterionInput{
				Value:    intStrs(performerIDs),
				Modifier: "INCLUDES",
			},
		})...)
	}

	if len(studioIDs) > 0 {
		ids = append(ids, queryImageIDs(ctx, r, &models.ImageFilterType{
			Studios: &models.HierarchicalMultiCriterionInput{
				Value:    intStrs(studioIDs),
				Modifier: "INCLUDES",
				Depth:    newInt(0),
			},
		})...)
	}

	return uniqueInts(ids)
}

func querySceneIDs(ctx context.Context, r models.Repository, filter *models.SceneFilterType) []int {
	queryOptions := models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: &models.FindFilterType{PerPage: newInt(models.PerPageAll)},
			Count:      false,
		},
		SceneFilter: filter,
	}
	result, err := r.Scene.Query(ctx, queryOptions)
	if err != nil {
		logger.Errorf("Error querying scenes: %v", err)
		return nil
	}
	scenes, err := result.Resolve(ctx)
	if err != nil {
		logger.Errorf("Error resolving scenes: %v", err)
		return nil
	}
	ids := make([]int, 0, len(scenes))
	for _, s := range scenes {
		if s != nil {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

func queryImageIDs(ctx context.Context, r models.Repository, filter *models.ImageFilterType) []int {
	queryOptions := models.ImageQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: &models.FindFilterType{PerPage: newInt(models.PerPageAll)},
			Count:      false,
		},
		ImageFilter: filter,
	}
	result, err := r.Image.Query(ctx, queryOptions)
	if err != nil {
		logger.Errorf("Error querying images: %v", err)
		return nil
	}
	images, err := result.Resolve(ctx)
	if err != nil {
		logger.Errorf("Error resolving images: %v", err)
		return nil
	}
	ids := make([]int, 0, len(images))
	for _, img := range images {
		if img != nil {
			ids = append(ids, img.ID)
		}
	}
	return ids
}

func intStrs(ids []int) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("%d", id)
	}
	return out
}

func uniqueInts(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func newInt(i int) *int {
	return &i
}
