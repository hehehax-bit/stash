package sqlite

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCosineSimilarity(t *testing.T) {
	identical := []float32{1, 0, 0}
	assert.Equal(t, 1.0, cosineSimilarity(identical, identical))

	orthogonal := []float32{0, 1, 0}
	assert.Equal(t, 0.0, cosineSimilarity(identical, orthogonal))

	negative := []float32{-1, 0, 0}
	assert.Equal(t, -1.0, cosineSimilarity(identical, negative))

	zero := []float32{0, 0, 0}
	assert.Equal(t, 0.0, cosineSimilarity(identical, zero))

	short := []float32{1}
	assert.Equal(t, 0.0, cosineSimilarity(identical, short))

	assert.Equal(t, 0.0, cosineSimilarity(nil, nil))
	assert.Equal(t, 0.0, cosineSimilarity([]float32{}, []float32{1}))
}

func TestFloat32BytesRoundTrip(t *testing.T) {
	original := []float32{0.1, -0.5, 1.0, 3.14159}
	b := float32sToBytes(original)
	restored := bytesToFloat32s(b)
	assert.Equal(t, original, restored)
}

func TestCosineSimilarityEdgeCases(t *testing.T) {
	assert.Equal(t, 0.0, cosineSimilarity([]float32{1, 2}, []float32{1}))
	assert.Equal(t, 0.0, cosineSimilarity([]float32{0, 0, 0}, []float32{0, 0, 0}))

	a := []float32{3, 4}
	assert.Equal(t, 1.0, cosineSimilarity(a, a))

	b := []float32{1, 1, 1, 1}
	c := []float32{2, 2, 2, 2}
	assert.InDelta(t, 1.0, cosineSimilarity(b, c), 0.0001)
}

func TestCosineSimilarityKnownValues(t *testing.T) {
	a := []float32{1, 1}
	b := []float32{1, 0}
	assert.InDelta(t, 1/math.Sqrt(2), cosineSimilarity(a, b), 0.0001)

	v1 := []float32{1, 2, 3}
	v2 := []float32{4, 5, 6}
	expected := (1*4 + 2*5 + 3*6) / (math.Sqrt(14) * math.Sqrt(77))
	assert.InDelta(t, expected, cosineSimilarity(v1, v2), 0.0001)
}
