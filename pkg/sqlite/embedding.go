package sqlite

import (
	"context"
	"encoding/binary"
	"math"
	"sort"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

type embeddingStore struct{}

// NewEmbeddingStore creates a new embedding store.
func NewEmbeddingStore() *embeddingStore {
	return &embeddingStore{}
}

// float32sToBytes converts a slice of float32 to a byte slice using little-endian encoding.
func float32sToBytes(vec []float32) []byte {
	b := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}

// bytesToFloat32s converts a byte slice to a slice of float32 using little-endian decoding.
func bytesToFloat32s(b []byte) []float32 {
	vec := make([]float32, len(b)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return vec
}

// FindByEntity retrieves the embedding for a specific entity by type, ID, and model.
// Returns nil if no embedding exists for the given entity.
func (s *embeddingStore) FindByEntity(ctx context.Context, entityType string, entityID int, model string) ([]float32, error) {
	var blob []byte
	err := dbWrapper.Get(ctx, &blob,
		`SELECT embedding FROM embeddings WHERE entity_type = ? AND entity_id = ? AND model = ?`,
		entityType, entityID, model,
	)
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, nil
	}
	return bytesToFloat32s(blob), nil
}

// FindByEntityType retrieves all embeddings for a given entity type and model.
func (s *embeddingStore) FindByEntityType(ctx context.Context, entityType string, model string) (map[int][]float32, error) {
	rows, err := dbWrapper.Queryx(ctx,
		`SELECT entity_id, embedding FROM embeddings WHERE entity_type = ? AND model = ?`,
		entityType, model,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]float32)
	for rows.Next() {
		var entityID int
		var blob []byte
		if err := rows.Scan(&entityID, &blob); err != nil {
			return nil, err
		}
		result[entityID] = bytesToFloat32s(blob)
	}
	return result, rows.Err()
}

// Set stores or updates an embedding for the given entity.
// Uses upsert to replace existing embeddings for the same entity/model combination.
func (s *embeddingStore) Set(ctx context.Context, entityType string, entityID int, model string, embedding []float32) error {
	blob := float32sToBytes(embedding)
	now := time.Now().Unix()

	_, err := dbWrapper.Exec(ctx,
		`INSERT INTO embeddings (entity_type, entity_id, model, embedding, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(entity_type, entity_id, model) DO UPDATE SET embedding = excluded.embedding, created_at = excluded.created_at`,
		entityType, entityID, model, blob, now,
	)
	return err
}

// HasEmbedding reports whether an embedding exists for the entity under any
// model.
func (s *embeddingStore) HasEmbedding(ctx context.Context, entityType string, entityID int) (bool, error) {
	var count int
	err := dbWrapper.Get(ctx, &count,
		`SELECT COUNT(*) FROM embeddings WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID,
	)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountByEntityType returns the number of distinct entities that have at least
// one embedding, grouped by entity type.
func (s *embeddingStore) CountByEntityType(ctx context.Context) (map[string]int, error) {
	rows, err := dbWrapper.Queryx(ctx,
		`SELECT entity_type, COUNT(DISTINCT entity_id) FROM embeddings GROUP BY entity_type`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var entityType string
		var count int
		if err := rows.Scan(&entityType, &count); err != nil {
			return nil, err
		}
		result[entityType] = count
	}
	return result, rows.Err()
}

// entityTableFor returns the table name for an entity type, used for staleness
// comparisons against the entity's updated_at column.
func entityTableFor(entityType string) string {
	switch entityType {
	case "scene":
		return "scenes"
	case "image":
		return "images"
	case "performer":
		return "performers"
	case "studio":
		return "studios"
	case "tag":
		return "tags"
	case "gallery":
		return "galleries"
	}
	return ""
}

// FindStale returns the entity IDs whose embedding for the given model was
// created before the entity's metadata was last updated.
func (s *embeddingStore) FindStale(ctx context.Context, entityType, model string) ([]int, error) {
	table := entityTableFor(entityType)
	if table == "" {
		return nil, nil
	}

	rows, err := dbWrapper.Queryx(ctx,
		`SELECT e.entity_id FROM embeddings e
		 INNER JOIN `+table+` t ON t.id = e.entity_id
		 WHERE e.entity_type = ? AND e.model = ? AND strftime('%s', t.updated_at) > e.created_at`,
		entityType, model,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteByEntity deletes all embeddings for a specific entity across all models.
func (s *embeddingStore) DeleteByEntity(ctx context.Context, entityType string, entityID int) error {
	_, err := dbWrapper.Exec(ctx,
		`DELETE FROM embeddings WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID,
	)
	return err
}

// DeleteByEntityType deletes all embeddings for a given entity type.
func (s *embeddingStore) DeleteByEntityType(ctx context.Context, entityType string) error {
	_, err := dbWrapper.Exec(ctx,
		`DELETE FROM embeddings WHERE entity_type = ?`,
		entityType,
	)
	return err
}

// DeleteByModel deletes all embeddings for a specific model.
func (s *embeddingStore) DeleteByModel(ctx context.Context, model string) error {
	_, err := dbWrapper.Exec(ctx,
		`DELETE FROM embeddings WHERE model = ?`,
		model,
	)
	return err
}

// cosineSimilarity computes the cosine similarity between two float32 vectors.
// Returns a value between -1 and 1, where 1 means identical direction, 0 means orthogonal,
// and -1 means opposite direction. Returns 0 for mismatched lengths or zero vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// FindNearDuplicates groups entities of the given type whose embeddings are
// mutually similar, using cosine similarity and a minimum score threshold.
// Returns groups of entity IDs. Each entity is assigned to at most one group.
// Groups with fewer than minGroupSize members are dropped.
// If maxEntities > 0, only the first maxEntities embeddings are processed
// to prevent O(n^2) performance issues on large libraries.
func (s *embeddingStore) FindNearDuplicates(ctx context.Context, entityType string, model string, threshold float64, minGroupSize int, maxEntities int) ([]models.DuplicateGroup, error) {
	if minGroupSize < 2 {
		minGroupSize = 2
	}
	if maxEntities <= 0 {
		maxEntities = 10000
	}

	all, err := s.FindByEntityType(ctx, entityType, model)
	if err != nil {
		return nil, err
	}

	// Limit the number of entities to process
	if len(all) > maxEntities {
		all = limitMap(all, maxEntities)
	}

	// Greedy clustering: for each unassigned entity, collect all other
	// unassigned entities within the threshold into a group.
	assigned := make(map[int]bool, len(all))
	var groups []models.DuplicateGroup

	ids := make([]int, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}

	for _, id := range ids {
		if assigned[id] {
			continue
		}

		group := []int{id}
		for _, other := range ids {
			if other == id || assigned[other] {
				continue
			}
			if cosineSimilarity(all[id], all[other]) >= threshold {
				group = append(group, other)
			}
		}

		if len(group) >= minGroupSize {
			for _, g := range group {
				assigned[g] = true
			}
			groups = append(groups, models.DuplicateGroup{EntityIDs: group})
		}
	}

	return groups, nil
}

// limitMap returns a new map containing at most maxEntries entries from the input map.
func limitMap(m map[int][]float32, maxEntries int) map[int][]float32 {
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

// SearchSimilar finds the most similar entities to the given query vector.
// Returns up to limit results sorted by similarity score (highest first).
func (s *embeddingStore) SearchSimilar(ctx context.Context, entityType string, model string, query []float32, limit int) ([]models.SimilarityResult, error) {
	all, err := s.FindByEntityType(ctx, entityType, model)
	if err != nil {
		return nil, err
	}

	type scored struct {
		id    int
		score float64
	}

	var results []scored
	for id, vec := range all {
		score := cosineSimilarity(query, vec)
		results = append(results, scored{id: id, score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if limit > len(results) {
		limit = len(results)
	}

	out := make([]models.SimilarityResult, limit)
	for i := 0; i < limit; i++ {
		out[i] = models.SimilarityResult{EntityID: results[i].id, Score: results[i].score}
	}
	return out, nil
}
