package sqlite

import (
	"context"
	"fmt"
)

// hasEmbeddingCriterionHandler filters entities by the presence of an embedding
// in the embeddings table. entityType matches the embeddings.entity_type value
// and idCol is the fully-qualified id column of the queried table. The filter
// value is "true" or "false".
func hasEmbeddingCriterionHandler(hasEmbedding *string, entityType string, idCol string) criterionHandlerFunc {
	return func(ctx context.Context, f *filterBuilder) {
		if hasEmbedding != nil {
			exists := fmt.Sprintf("EXISTS (SELECT 1 FROM embeddings WHERE entity_type = '%s' AND entity_id = %s)", entityType, idCol)
			if *hasEmbedding == "true" {
				f.addWhere(exists)
			} else {
				f.addWhere("NOT " + exists)
			}
		}
	}
}
