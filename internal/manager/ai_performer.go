package manager

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/stashapp/stash/pkg/models"
)

// genericPerformerTokens are words that indicate a descriptive label rather
// than an actual performer name. Names consisting entirely of these tokens
// should not be created as new performers.
var genericPerformerTokens = map[string]struct{}{
	"unknown": {}, "unidentified": {}, "anonymous": {}, "unnamed": {},
	"man": {}, "men": {}, "woman": {}, "women": {}, "girl": {}, "girls": {},
	"guy": {}, "guys": {}, "boy": {}, "boys": {}, "male": {}, "female": {},
	"performer": {}, "performers": {}, "actor": {}, "actress": {},
	"person": {}, "people": {}, "model": {}, "models": {}, "adult": {},
	"star": {}, "figure": {}, "individual": {}, "one": {}, "two": {},
	"some": {}, "a": {}, "an": {}, "the": {}, "and": {}, "with": {}, "in": {},
	"blonde": {}, "brunette": {}, "redhead": {}, "redheaded": {},
	"black": {}, "white": {}, "asian": {}, "caucasian": {}, "hispanic": {},
	"short": {}, "long": {}, "hair": {}, "hairy": {}, "tall": {}, "young": {},
	"old": {},
}

// performerFuzzyThreshold is the minimum name similarity for two performer
// names to be considered the same performer.
const performerFuzzyThreshold = 0.6

// isGenericPerformerName returns true if the name is a generic descriptor
// (e.g. "unknown performer", "woman with short hair") rather than a specific
// performer name.
func isGenericPerformerName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}

	lower := strings.ToLower(name)
	if strings.Contains(lower, "unknown") || strings.Contains(lower, "unidentified") {
		return true
	}

	tokens := performerNameTokens(name)
	if len(tokens) == 0 {
		return true
	}

	for _, t := range tokens {
		if _, ok := genericPerformerTokens[t]; !ok {
			return false
		}
	}
	return true
}

// performerNameTokens returns the lowercase alphanumeric tokens of a name.
func performerNameTokens(name string) []string {
	var tokens []string
	for _, field := range strings.Fields(name) {
		var b strings.Builder
		for _, r := range field {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(unicode.ToLower(r))
			}
		}
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
		}
	}
	return tokens
}

// levenshteinDistance computes the edit distance between two strings.
func levenshteinDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// levenshteinSimilarity returns a value in [0,1] based on edit distance.
func levenshteinSimilarity(a, b string) float64 {
	maxLen := max(len(a), len(b))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(levenshteinDistance(a, b))/float64(maxLen)
}

// performerNameSimilarity returns a value in [0,1] describing how similar two
// performer names are. Single-token names use edit-distance similarity;
// multi-token names use the Dice coefficient over the token sets.
func performerNameSimilarity(a, b string) float64 {
	at := performerNameTokens(a)
	bt := performerNameTokens(b)
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}

	if len(at) == 1 && len(bt) == 1 {
		return levenshteinSimilarity(at[0], bt[0])
	}

	setB := make(map[string]struct{}, len(bt))
	for _, t := range bt {
		setB[t] = struct{}{}
	}

	intersection := 0
	for _, t := range at {
		if _, ok := setB[t]; ok {
			intersection++
		}
	}

	return 2.0 * float64(intersection) / float64(len(at)+len(bt))
}

// performerResolver resolves AI-suggested performer names against the
// existing performer library, reusing matching performers and avoiding
// duplicate or generic creations. Existing performers are loaded lazily and
// cached for the duration of a job so that fuzzy matching only queries the
// library once.
type performerResolver struct {
	r      models.Repository
	all    []*models.Performer
	loaded bool
}

// resolve returns the ID of the existing performer matching the given name, or
// creates a new performer if create is true and the name is specific. Returns
// 0 if the name is generic or no match exists and creation is not permitted.
// fields is applied to the new performer before creation, if provided.
func (pr *performerResolver) resolve(ctx context.Context, name string, create bool, fields func(*models.Performer)) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" || isGenericPerformerName(name) {
		return 0, nil
	}

	existing, err := pr.r.Performer.FindByNames(ctx, []string{name}, true)
	if err != nil {
		return 0, fmt.Errorf("finding performer %q: %w", name, err)
	}
	if len(existing) > 0 {
		return existing[0].ID, nil
	}

	if !pr.loaded {
		pr.all, err = pr.r.Performer.All(ctx)
		if err != nil {
			return 0, fmt.Errorf("loading performers: %w", err)
		}
		pr.loaded = true
	}
	for _, p := range pr.all {
		if performerNameSimilarity(name, p.Name) >= performerFuzzyThreshold {
			return p.ID, nil
		}
	}

	if !create {
		return 0, nil
	}

	newPerformer := models.NewPerformer()
	newPerformer.Name = name
	if fields != nil {
		fields(&newPerformer)
	}
	if err := pr.r.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &newPerformer}); err != nil {
		return 0, fmt.Errorf("creating performer %q: %w", name, err)
	}
	return newPerformer.ID, nil
}
