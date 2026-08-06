package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsGenericPerformerName(t *testing.T) {
	generic := []string{
		"unknown performer",
		"unknown",
		"unknown man",
		"unidentified woman",
		"woman with short hair",
		"blonde woman",
		"black man",
		"a woman",
		"young man",
		"man",
	}
	for _, name := range generic {
		assert.Truef(t, isGenericPerformerName(name), "expected %q to be generic", name)
	}

	specific := []string{
		"Zelda",
		"Princess Zelda",
		"Mia Malkova",
		"Jane Doe",
		"Rae Lil Black",
	}
	for _, name := range specific {
		assert.Falsef(t, isGenericPerformerName(name), "expected %q to be specific", name)
	}
}

func TestPerformerNameSimilarity(t *testing.T) {
	assert.InDelta(t, 1.0, performerNameSimilarity("Zelda", "Zelda"), 0.001)

	// shared token: "Princess Zelda" should strongly match existing "Zelda"
	assert.GreaterOrEqual(t, performerNameSimilarity("Princess Zelda", "Zelda"), 0.6)

	// single-word typo
	assert.GreaterOrEqual(t, performerNameSimilarity("Mia", "Mya"), 0.6)

	// different people sharing a surname should NOT match
	assert.Less(t, performerNameSimilarity("Jane Doe", "John Doe"), 0.6)

	// unrelated names should NOT match
	assert.Less(t, performerNameSimilarity("Zelda", "Mia Malkova"), 0.6)
}
