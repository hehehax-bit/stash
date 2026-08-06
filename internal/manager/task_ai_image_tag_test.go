package manager

import (
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetPerformerFields_SetsAllFields(t *testing.T) {
	p := &models.Performer{}

	setPerformerFields(p, aiImagePerformer{
		Name:      "Jane Doe",
		Gender:    "female",
		Ethnicity: "Caucasian",
		HairColor: "Blonde",
		EyeColor:  "Blue",
		Details:   "blonde woman",
	})

	require.NotNil(t, p.Gender)
	assert.Equal(t, models.GenderEnumFemale, *p.Gender)
	assert.Equal(t, "Caucasian", p.Ethnicity)
	assert.Equal(t, "Blonde", p.HairColor)
	assert.Equal(t, "Blue", p.EyeColor)
	assert.Equal(t, "blonde woman", p.Details)
}

func TestSetPerformerFields_SkipsUnknownValues(t *testing.T) {
	p := &models.Performer{}

	setPerformerFields(p, aiImagePerformer{
		Name:      "Jane Doe",
		Gender:    "unknown",
		Ethnicity: "Unknown",
		HairColor: "",
		EyeColor:  "",
		Details:   "",
	})

	assert.Nil(t, p.Gender)
	assert.Empty(t, p.Ethnicity)
	assert.Empty(t, p.HairColor)
	assert.Empty(t, p.EyeColor)
	assert.Empty(t, p.Details)
}

func TestSetPerformerFields_RejectsInvalidGender(t *testing.T) {
	p := &models.Performer{}

	setPerformerFields(p, aiImagePerformer{Gender: "NotAGender"})

	assert.Nil(t, p.Gender)
}
