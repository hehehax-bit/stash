package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/ffmpeg"
	"github.com/stashapp/stash/pkg/ffmpeg/transcoder"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sliceutil"
	"github.com/stashapp/stash/pkg/sliceutil/stringslice"
	"github.com/stashapp/stash/pkg/tag"
)

type ToolFunc func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error)

type Tool struct {
	Name        string
	Description string
	Parameters  interface{}
	Execute     ToolFunc
}

var searchScenesParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Search query for scene title, details, tags, or performers",
		},
		"min_o_counter": map[string]interface{}{
			"type":        "integer",
			"description": "Minimum o-counter (orgasm count) for filtering",
		},
		"min_play_count": map[string]interface{}{
			"type":        "integer",
			"description": "Minimum play count for filtering",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"query"},
}

var getSceneParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"id": map[string]interface{}{
			"type":        "integer",
			"description": "Scene ID",
		},
	},
	"required": []string{"id"},
}

var searchPerformersParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Search query for performer name or aliases",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"query"},
}

var searchImagesParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Search query for image title, details, tags, or performers",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"query"},
}

var searchGalleriesParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Search query for gallery title, details, tags, or performers",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"query"},
}

var countByTagParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"tag_name": map[string]interface{}{
			"type":        "string",
			"description": "Tag name to search for",
		},
	},
	"required": []string{"tag_name"},
}

var getStatsParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"reason": map[string]interface{}{
			"type":        "string",
			"description": "Why you need stats",
		},
	},
}

func searchScenes(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Query        string `json:"query"`
		MinOCounter  *int   `json:"min_o_counter"`
		MinPlayCount *int   `json:"min_play_count"`
		Limit        int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	q := params.Query
	filter := &models.FindFilterType{
		Q:       &q,
		PerPage: &params.Limit,
		Sort:    strPtr("title"),
	}

	sceneFilter := &models.SceneFilterType{}
	if params.MinOCounter != nil {
		sceneFilter.OCounter = &models.IntCriterionInput{
			Value:    *params.MinOCounter,
			Modifier: models.CriterionModifierGreaterThan,
		}
	}
	if params.MinPlayCount != nil {
		sceneFilter.PlayCount = &models.IntCriterionInput{
			Value:    *params.MinPlayCount,
			Modifier: models.CriterionModifierGreaterThan,
		}
	}

	result, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: filter,
			Count:      false,
		},
		SceneFilter: sceneFilter,
	})
	if err != nil {
		return "", fmt.Errorf("querying scenes: %w", err)
	}

	scenes, err := result.Resolve(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving scenes: %w", err)
	}

	if len(scenes) == 0 {
		return "No scenes found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d scenes:\n\n", len(scenes))
	for _, s := range scenes {
		_ = s.LoadTagIDs(ctx, repo.Scene)
		_ = s.LoadPerformerIDs(ctx, repo.Scene)
		_ = s.LoadPrimaryFile(ctx, repo.File)

		fmt.Fprintf(&b, "- [Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
		if s.Rating != nil {
			fmt.Fprintf(&b, " | Rating: %d/100", *s.Rating)
		}
		if s.Date != nil {
			fmt.Fprintf(&b, " | Date: %s", s.Date.String())
		}
		if len(s.TagIDs.List()) > 0 {
			tags, _ := repo.Tag.FindMany(ctx, s.TagIDs.List())
			var tagNames []string
			for _, t := range tags {
				tagNames = append(tagNames, t.Name)
			}
			fmt.Fprintf(&b, " | Tags: %s", strings.Join(tagNames, ", "))
		}
		if len(s.PerformerIDs.List()) > 0 {
			performers, _ := repo.Performer.FindMany(ctx, s.PerformerIDs.List())
			var perfNames []string
			for _, p := range performers {
				perfNames = append(perfNames, p.Name)
			}
			fmt.Fprintf(&b, " | Performers: %s", strings.Join(perfNames, ", "))
		}
		f := s.Files.Primary()
		if f != nil {
			fmt.Fprintf(&b, " | Duration: %.1fs", f.Duration)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func getScene(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	s, err := repo.Scene.Find(ctx, params.ID)
	if err != nil {
		return "", fmt.Errorf("finding scene: %w", err)
	}
	if s == nil {
		return fmt.Sprintf("Scene with ID %d not found.", params.ID), nil
	}

	_ = s.LoadTagIDs(ctx, repo.Scene)
	_ = s.LoadPerformerIDs(ctx, repo.Scene)
	_ = s.LoadGalleryIDs(ctx, repo.Scene)
	_ = s.LoadPrimaryFile(ctx, repo.File)
	_ = s.LoadURLs(ctx, repo.Scene)
	_ = s.LoadStashIDs(ctx, repo.Scene)

	var b strings.Builder
	fmt.Fprintf(&b, "[Scene #%d - %s](/scenes/%d)\n", s.ID, s.Title, s.ID)
	if s.Details != "" {
		fmt.Fprintf(&b, "  Details: %s\n", s.Details)
	}
	if s.Director != "" {
		fmt.Fprintf(&b, "  Director: %s\n", s.Director)
	}
	if s.Code != "" {
		fmt.Fprintf(&b, "  Code: %s\n", s.Code)
	}
	if s.Rating != nil {
		fmt.Fprintf(&b, "  Rating: %d/100\n", *s.Rating)
	}
	if s.Date != nil {
		fmt.Fprintf(&b, "  Date: %s\n", s.Date.String())
	}
	fmt.Fprintf(&b, "  Organized: %v\n", s.Organized)
	oCount, _ := repo.Scene.GetOCount(ctx, s.ID)
	fmt.Fprintf(&b, "  OCounter: %d\n", oCount)
	viewCount, _ := repo.Scene.CountViews(ctx, s.ID)
	fmt.Fprintf(&b, "  PlayCount: %d\n", viewCount)

	if s.StudioID != nil {
		studio, _ := repo.Studio.Find(ctx, *s.StudioID)
		if studio != nil {
			fmt.Fprintf(&b, "  Studio: %s\n", studio.Name)
		}
	}

	if len(s.TagIDs.List()) > 0 {
		tags, _ := repo.Tag.FindMany(ctx, s.TagIDs.List())
		var tagNames []string
		for _, t := range tags {
			tagNames = append(tagNames, t.Name)
		}
		fmt.Fprintf(&b, "  Tags: %s\n", strings.Join(tagNames, ", "))
	}

	if len(s.PerformerIDs.List()) > 0 {
		performers, _ := repo.Performer.FindMany(ctx, s.PerformerIDs.List())
		var perfNames []string
		for _, p := range performers {
			perfNames = append(perfNames, p.Name)
		}
		fmt.Fprintf(&b, "  Performers: %s\n", strings.Join(perfNames, ", "))
	}

	f := s.Files.Primary()
	if f != nil {
		fmt.Fprintf(&b, "  Duration: %.1fs\n", f.Duration)
		fmt.Fprintf(&b, "  Resolution: %dx%d\n", f.Width, f.Height)
		fmt.Fprintf(&b, "  Size: %.1f MB\n", float64(f.Size)/1024/1024)
	}

	if len(s.URLs.List()) > 0 {
		fmt.Fprintf(&b, "  URLs: %s\n", strings.Join(s.URLs.List(), ", "))
	}

	return b.String(), nil
}

func searchPerformers(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	q := params.Query
	filter := &models.FindFilterType{
		Q:       &q,
		PerPage: &params.Limit,
		Sort:    strPtr("name"),
	}

	performers, _, err := repo.Performer.Query(ctx, nil, filter)
	if err != nil {
		return "", fmt.Errorf("querying performers: %w", err)
	}

	if len(performers) == 0 {
		return "No performers found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d performers:\n\n", len(performers))
	for _, p := range performers {
		fmt.Fprintf(&b, "- [Performer #%d - %s](/performers/%d)", p.ID, p.Name, p.ID)
		if len(p.Aliases.List()) > 0 {
			fmt.Fprintf(&b, " | Aliases: %s", strings.Join(p.Aliases.List(), ", "))
		}
		if p.Gender != nil {
			fmt.Fprintf(&b, " | Gender: %s", p.Gender.String())
		}
		if p.Rating != nil {
			fmt.Fprintf(&b, " | Rating: %d/100", *p.Rating)
		}
		if p.Favorite {
			fmt.Fprintf(&b, " | Favorite")
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func searchImages(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	q := params.Query
	filter := &models.FindFilterType{
		Q:       &q,
		PerPage: &params.Limit,
		Sort:    strPtr("title"),
	}

	result, err := repo.Image.Query(ctx, models.ImageQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: filter,
			Count:      false,
		},
	})
	if err != nil {
		return "", fmt.Errorf("querying images: %w", err)
	}

	images, err := result.Resolve(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving images: %w", err)
	}

	if len(images) == 0 {
		return "No images found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d images:\n\n", len(images))
	for _, img := range images {
		_ = img.LoadTagIDs(ctx, repo.Image)
		_ = img.LoadPerformerIDs(ctx, repo.Image)

		fmt.Fprintf(&b, "- [Image #%d - %s](/images/%d)", img.ID, img.Title, img.ID)
		if img.Rating != nil {
			fmt.Fprintf(&b, " | Rating: %d/100", *img.Rating)
		}
		if len(img.TagIDs.List()) > 0 {
			tags, _ := repo.Tag.FindMany(ctx, img.TagIDs.List())
			var tagNames []string
			for _, t := range tags {
				tagNames = append(tagNames, t.Name)
			}
			fmt.Fprintf(&b, " | Tags: %s", strings.Join(tagNames, ", "))
		}
		if len(img.PerformerIDs.List()) > 0 {
			performers, _ := repo.Performer.FindMany(ctx, img.PerformerIDs.List())
			var perfNames []string
			for _, p := range performers {
				perfNames = append(perfNames, p.Name)
			}
			fmt.Fprintf(&b, " | Performers: %s", strings.Join(perfNames, ", "))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func searchGalleries(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	q := params.Query
	filter := &models.FindFilterType{
		Q:       &q,
		PerPage: &params.Limit,
		Sort:    strPtr("title"),
	}

	galleries, _, err := repo.Gallery.Query(ctx, nil, filter)
	if err != nil {
		return "", fmt.Errorf("querying galleries: %w", err)
	}

	if len(galleries) == 0 {
		return "No galleries found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d galleries:\n\n", len(galleries))
	for _, g := range galleries {
		_ = g.LoadTagIDs(ctx, repo.Gallery)
		_ = g.LoadPerformerIDs(ctx, repo.Gallery)

		fmt.Fprintf(&b, "- [Gallery #%d - %s](/galleries/%d)", g.ID, g.Title, g.ID)
		if g.Rating != nil {
			fmt.Fprintf(&b, " | Rating: %d/100", *g.Rating)
		}
		if len(g.TagIDs.List()) > 0 {
			tags, _ := repo.Tag.FindMany(ctx, g.TagIDs.List())
			var tagNames []string
			for _, t := range tags {
				tagNames = append(tagNames, t.Name)
			}
			fmt.Fprintf(&b, " | Tags: %s", strings.Join(tagNames, ", "))
		}
		if len(g.PerformerIDs.List()) > 0 {
			performers, _ := repo.Performer.FindMany(ctx, g.PerformerIDs.List())
			var perfNames []string
			for _, p := range performers {
				perfNames = append(perfNames, p.Name)
			}
			fmt.Fprintf(&b, " | Performers: %s", strings.Join(perfNames, ", "))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func countByTag(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	tagFilter := &models.TagFilterType{
		Name: &models.StringCriterionInput{
			Value:    params.TagName,
			Modifier: models.CriterionModifierEquals,
		},
	}

	tags, _, err := repo.Tag.Query(ctx, tagFilter, &models.FindFilterType{PerPage: intPtr(1)})
	if err != nil {
		return "", fmt.Errorf("finding tag: %w", err)
	}

	if len(tags) == 0 {
		return fmt.Sprintf("No tag found matching \"%s\".", params.TagName), nil
	}

	tag := tags[0]

	sceneFilter := &models.SceneFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{fmt.Sprintf("%d", tag.ID)},
			Modifier: models.CriterionModifierIncludes,
		},
	}

	sceneResult, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: &models.FindFilterType{PerPage: intPtr(0)},
			Count:      true,
		},
		SceneFilter: sceneFilter,
	})
	if err != nil {
		return "", fmt.Errorf("counting scenes: %w", err)
	}

	imageFilter := &models.ImageFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{fmt.Sprintf("%d", tag.ID)},
			Modifier: models.CriterionModifierIncludes,
		},
	}

	imageResult, err := repo.Image.Query(ctx, models.ImageQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: &models.FindFilterType{PerPage: intPtr(0)},
			Count:      true,
		},
		ImageFilter: imageFilter,
	})
	if err != nil {
		return "", fmt.Errorf("counting images: %w", err)
	}

	return fmt.Sprintf("Tag \"%s\": %d scenes, %d images", tag.Name, sceneResult.Count, imageResult.Count), nil
}

func getStats(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	sceneCount, _ := repo.Scene.Count(ctx)
	imageCount, _ := repo.Image.Count(ctx)
	galleryCount, _ := repo.Gallery.Count(ctx)
	performerCount, _ := repo.Performer.Count(ctx)
	studioCount, _ := repo.Studio.Count(ctx)
	tagCount, _ := repo.Tag.Count(ctx)
	sceneDuration, _ := repo.Scene.Duration(ctx)

	var b strings.Builder
	fmt.Fprintf(&b, "Library Statistics:\n")
	fmt.Fprintf(&b, "  Scenes: %d (%.1f hours total)\n", sceneCount, sceneDuration/3600)
	fmt.Fprintf(&b, "  Images: %d\n", imageCount)
	fmt.Fprintf(&b, "  Galleries: %d\n", galleryCount)
	fmt.Fprintf(&b, "  Performers: %d\n", performerCount)
	fmt.Fprintf(&b, "  Studios: %d\n", studioCount)
	fmt.Fprintf(&b, "  Tags: %d\n", tagCount)

	return b.String(), nil
}

var searchSimilarScenesParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"scene_id": map[string]interface{}{
			"type":        "integer",
			"description": "ID of the scene to find similar scenes for",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"scene_id"},
}

var searchSemanticParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Natural language description of what to find",
		},
		"type": map[string]interface{}{
			"type":        "string",
			"description": "Entity type to search: 'scene', 'performer', 'image', 'gallery', 'studio', or 'tag'",
			"enum":        []string{"scene", "performer", "image", "gallery", "studio", "tag"},
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"query", "type"},
}

var searchSimilarImagesParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"image_id": map[string]interface{}{
			"type":        "integer",
			"description": "ID of the image to find similar images for",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"image_id"},
}

var batchSceneUpdateParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Text search query to filter scenes by title, details, or path",
		},
		"tag_ids": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "integer"},
			"description": "Only affect scenes with ALL of these tag IDs",
		},
		"studio_id": map[string]interface{}{
			"type":        "integer",
			"description": "Only affect scenes from this studio ID",
		},
		"performer_ids": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "integer"},
			"description": "Only affect scenes with ALL of these performer IDs",
		},
		"rating_min": map[string]interface{}{
			"type":        "integer",
			"description": "Minimum rating (1-100) filter",
		},
		"rating_max": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum rating (1-100) filter",
		},
		"organized": map[string]interface{}{
			"type":        "boolean",
			"description": "Only affect organized (true) or unorganized (false) scenes",
		},
		"add_tag_ids": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "integer"},
			"description": "Tag IDs to add to matching scenes",
		},
		"remove_tag_ids": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "integer"},
			"description": "Tag IDs to remove from matching scenes",
		},
		"set_rating": map[string]interface{}{
			"type":        "integer",
			"description": "Set rating (1-100) for matching scenes. Set to 0 to clear rating.",
		},
		"set_organized": map[string]interface{}{
			"type":        "boolean",
			"description": "Set organized flag for matching scenes",
		},
		"set_studio_id": map[string]interface{}{
			"type":        "integer",
			"description": "Set studio ID for matching scenes. Set to 0 to clear studio.",
		},
		"add_performer_ids": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "integer"},
			"description": "Performer IDs to add to matching scenes",
		},
		"remove_performer_ids": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "integer"},
			"description": "Performer IDs to remove from matching scenes",
		},
		"confirmed": map[string]interface{}{
			"type":        "boolean",
			"description": "Set to true to confirm and execute the batch update. First call without confirmed to see how many scenes would be affected.",
		},
	},
}

var rememberParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"key": map[string]interface{}{
			"type":        "string",
			"description": "Memory key, e.g. 'preferred_genre', 'disliked_performer_5'",
		},
		"value": map[string]interface{}{
			"type":        "string",
			"description": "Memory value, e.g. 'retro', 'I prefer shorter scenes'",
		},
	},
	"required": []string{"key", "value"},
}

var forgetParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"key": map[string]interface{}{
			"type":        "string",
			"description": "Memory key to forget",
		},
	},
	"required": []string{"key"},
}

var emptyParam = map[string]interface{}{
	"type":       "object",
	"properties": map[string]interface{}{},
}

var recommendParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"type": map[string]interface{}{
			"type":        "string",
			"description": "Type of content to recommend",
			"enum":        []string{"scenes", "images"},
		},
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Natural language description of what the user is looking for",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Number of recommendations to return (1-20, default 5)",
			"minimum":     1,
			"maximum":     20,
		},
	},
	"required": []string{"type", "query"},
}

var queryScenesParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"search": map[string]interface{}{
			"type":        "string",
			"description": "Text search across scene title, details, and file path",
		},
		"tags": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Tag names to filter by",
		},
		"tags_mode": map[string]interface{}{
			"type":        "string",
			"description": "How to match tags: AND (scene must have all tags) or OR (scene must have any tag)",
			"enum":        []string{"AND", "OR"},
		},
		"performers": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Performer names to filter by",
		},
		"studio": map[string]interface{}{
			"type":        "string",
			"description": "Studio name to filter by",
		},
		"rating_min": map[string]interface{}{
			"type":        "integer",
			"description": "Minimum rating (0-100)",
			"minimum":     0,
			"maximum":     100,
		},
		"rating_max": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum rating (0-100)",
			"minimum":     0,
			"maximum":     100,
		},
		"date_from": map[string]interface{}{
			"type":        "string",
			"description": "Earliest date (inclusive, format YYYY-MM-DD)",
		},
		"date_to": map[string]interface{}{
			"type":        "string",
			"description": "Latest date (inclusive, format YYYY-MM-DD)",
		},
		"organized": map[string]interface{}{
			"type":        "boolean",
			"description": "Filter by organized status",
		},
		"has_markers": map[string]interface{}{
			"type":        "boolean",
			"description": "Filter scenes that have markers/segments",
		},
		"sort": map[string]interface{}{
			"type":        "string",
			"description": "Sort field: title, date, rating, duration, created_at, updated_at, random, o_counter, play_count, filesize, resolution",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum results to return (1-100, default 20)",
			"minimum":     1,
			"maximum":     100,
		},
	},
	"required": []string{},
}

func batchUpdateScenes(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Query              string `json:"query"`
		TagIDs             []int  `json:"tag_ids"`
		StudioID           *int   `json:"studio_id"`
		PerformerIDs       []int  `json:"performer_ids"`
		RatingMin          *int   `json:"rating_min"`
		RatingMax          *int   `json:"rating_max"`
		Organized          *bool  `json:"organized"`
		AddTagIDs          []int  `json:"add_tag_ids"`
		RemoveTagIDs       []int  `json:"remove_tag_ids"`
		SetRating          *int   `json:"set_rating"`
		SetOrganized       *bool  `json:"set_organized"`
		SetStudioID        *int   `json:"set_studio_id"`
		AddPerformerIDs    []int  `json:"add_performer_ids"`
		RemovePerformerIDs []int  `json:"remove_performer_ids"`
		Confirmed          bool   `json:"confirmed"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	if params.Query == "" && len(params.TagIDs) == 0 && params.StudioID == nil && len(params.PerformerIDs) == 0 && params.RatingMin == nil && params.RatingMax == nil && params.Organized == nil {
		return "Please provide at least one filter criterion (query, tag_ids, studio_id, performer_ids, rating_min/max, or organized) to avoid accidentally modifying all scenes.", nil
	}

	hasActions := len(params.AddTagIDs) > 0 || len(params.RemoveTagIDs) > 0 || params.SetRating != nil || params.SetOrganized != nil || params.SetStudioID != nil || len(params.AddPerformerIDs) > 0 || len(params.RemovePerformerIDs) > 0
	if !hasActions {
		return "No actions specified. Provide at least one action (add_tag_ids, remove_tag_ids, set_rating, set_organized, set_studio_id, add_performer_ids, remove_performer_ids).", nil
	}

	sceneFilter := &models.SceneFilterType{}
	if params.Organized != nil {
		sceneFilter.Organized = params.Organized
	}
	switch {
	case params.RatingMin != nil && params.RatingMax != nil:
		v2 := *params.RatingMax
		sceneFilter.Rating100 = &models.IntCriterionInput{
			Value:    *params.RatingMin,
			Value2:   &v2,
			Modifier: models.CriterionModifierBetween,
		}
	case params.RatingMin != nil:
		sceneFilter.Rating100 = &models.IntCriterionInput{
			Value:    *params.RatingMin,
			Modifier: models.CriterionModifierGreaterThan,
		}
	case params.RatingMax != nil:
		sceneFilter.Rating100 = &models.IntCriterionInput{
			Value:    *params.RatingMax,
			Modifier: models.CriterionModifierLessThan,
		}
	}
	if params.StudioID != nil {
		sceneFilter.Studios = &models.HierarchicalMultiCriterionInput{
			Value:    []string{fmt.Sprintf("%d", *params.StudioID)},
			Modifier: models.CriterionModifierIncludes,
		}
	}
	if len(params.TagIDs) > 0 {
		tagStrs := make([]string, len(params.TagIDs))
		for i, id := range params.TagIDs {
			tagStrs[i] = fmt.Sprintf("%d", id)
		}
		sceneFilter.Tags = &models.HierarchicalMultiCriterionInput{
			Value:    tagStrs,
			Modifier: models.CriterionModifierIncludesAll,
		}
	}
	if len(params.PerformerIDs) > 0 {
		perfStrs := make([]string, len(params.PerformerIDs))
		for i, id := range params.PerformerIDs {
			perfStrs[i] = fmt.Sprintf("%d", id)
		}
		sceneFilter.Performers = &models.MultiCriterionInput{
			Value:    perfStrs,
			Modifier: models.CriterionModifierIncludes,
		}
	}

	findFilter := &models.FindFilterType{}
	var q *string
	if params.Query != "" {
		q = &params.Query
	}
	findFilter.Q = q

	const maxBatch = 1000
	findFilter.PerPage = intPtr(maxBatch)

	result, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: findFilter,
			Count:      true,
		},
		SceneFilter: sceneFilter,
	})
	if err != nil {
		return "", fmt.Errorf("querying scenes: %w", err)
	}

	if result.Count > maxBatch {
		return fmt.Sprintf("Batch update would affect %d scenes, which exceeds the maximum batch size of %d. Please narrow your filters.", result.Count, maxBatch), nil
	}

	if result.Count == 0 {
		return "No scenes match the given filters.", nil
	}

	if !params.Confirmed {
		return fmt.Sprintf("This batch update would affect **%d scenes**. Call again with `\"confirmed\": true` to execute.\n\nFilters:\n- Query: %s\n- Tags: %v\n- Studio: %v\n- Performers: %v\n- Rating: %v-%v\n- Organized: %v\n\nActions:\n- Add tags: %v\n- Remove tags: %v\n- Set rating: %v\n- Set organized: %v\n- Set studio: %v\n- Add performers: %v\n- Remove performers: %v",
			result.Count,
			params.Query,
			params.TagIDs,
			params.StudioID,
			params.PerformerIDs,
			params.RatingMin, params.RatingMax,
			params.Organized,
			params.AddTagIDs, params.RemoveTagIDs,
			params.SetRating, params.SetOrganized, params.SetStudioID,
			params.AddPerformerIDs, params.RemovePerformerIDs), nil
	}

	scenes, err := result.Resolve(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving scenes: %w", err)
	}

	updateCount := 0
	if err := repo.WithTxn(ctx, func(ctx context.Context) error {
		for _, s := range scenes {
			partial := models.NewScenePartial()

			if len(params.AddTagIDs) > 0 || len(params.RemoveTagIDs) > 0 {
				_ = s.LoadTagIDs(ctx, repo.Scene)

				var mode models.RelationshipUpdateMode
				var ids []int
				if len(params.AddTagIDs) > 0 {
					mode = models.RelationshipUpdateModeAdd
					ids = params.AddTagIDs
				} else {
					mode = models.RelationshipUpdateModeRemove
					ids = params.RemoveTagIDs
				}

				if len(ids) > 0 {
					partial.TagIDs = &models.UpdateIDs{IDs: ids, Mode: mode}
				}
			}

			if params.SetRating != nil {
				if *params.SetRating > 0 {
					partial.Rating = models.NewOptionalInt(*params.SetRating)
				} else {
					partial.Rating = models.NewOptionalInt(0)
				}
			}

			if params.SetOrganized != nil {
				partial.Organized = models.NewOptionalBool(*params.SetOrganized)
			}

			if params.SetStudioID != nil {
				if *params.SetStudioID > 0 {
					partial.StudioID = models.NewOptionalInt(*params.SetStudioID)
				} else {
					partial.StudioID = models.NewOptionalInt(0)
				}
			}

			if len(params.AddPerformerIDs) > 0 || len(params.RemovePerformerIDs) > 0 {
				_ = s.LoadPerformerIDs(ctx, repo.Scene)

				var mode models.RelationshipUpdateMode
				var ids []int
				if len(params.AddPerformerIDs) > 0 {
					mode = models.RelationshipUpdateModeAdd
					ids = params.AddPerformerIDs
				} else {
					mode = models.RelationshipUpdateModeRemove
					ids = params.RemovePerformerIDs
				}

				if len(ids) > 0 {
					partial.PerformerIDs = &models.UpdateIDs{IDs: ids, Mode: mode}
				}
			}

			if _, err := repo.Scene.UpdatePartial(ctx, s.ID, partial); err != nil {
				return fmt.Errorf("updating scene %d: %w", s.ID, err)
			}
			updateCount++
		}
		return nil
	}); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully updated **%d scenes** out of %d matching.", updateCount, result.Count), nil
}

func remember(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if err := repo.Memory.Set(ctx, params.Key, params.Value); err != nil {
		return "", fmt.Errorf("saving memory: %w", err)
	}
	return fmt.Sprintf("Saved memory: \"%s\" = \"%s\"", params.Key, params.Value), nil
}

func forget(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if err := repo.Memory.Delete(ctx, params.Key); err != nil {
		return "", fmt.Errorf("deleting memory: %w", err)
	}
	return fmt.Sprintf("Forgot memory: \"%s\"", params.Key), nil
}

func listMemories(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	memories, err := repo.Memory.FindAll(ctx)
	if err != nil {
		return "", fmt.Errorf("listing memories: %w", err)
	}
	if len(memories) == 0 {
		return "No memories saved yet. Use `remember` to save preferences.", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Saved memories (%d):\n\n", len(memories))
	for _, m := range memories {
		fmt.Fprintf(&b, "- **%s**: %s\n", m.Key, m.Value)
	}
	return b.String(), nil
}

func searchSimilarScenes(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		SceneID int `json:"scene_id"`
		Limit   int `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	model := cfg.EmbeddingModel
	if model == "" {
		model = cfg.LLMModel
	}

	vec, err := repo.Embedding.FindByEntity(ctx, "scene", params.SceneID, model)
	if err != nil {
		return "", fmt.Errorf("finding embedding: %w", err)
	}
	if vec == nil {
		return "No embedding found for this scene. Run the embedding job first.", nil
	}

	results, err := repo.Embedding.SearchSimilar(ctx, "scene", model, vec, params.Limit)
	if err != nil {
		return "", fmt.Errorf("searching similar scenes: %w", err)
	}

	if len(results) == 0 {
		return "No similar scenes found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Similar scenes to scene #%d:\n\n", params.SceneID)
	for _, r := range results {
		s, _ := repo.Scene.Find(ctx, r.EntityID)
		if s == nil {
			continue
		}
		fmt.Fprintf(&b, "- [Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
		fmt.Fprintf(&b, " | Similarity: %.1f%%", r.Score*100)
		if r.EntityID == params.SceneID {
			fmt.Fprintf(&b, " (source)")
		}
		_ = s.LoadPrimaryFile(ctx, repo.File)
		if f := s.Files.Primary(); f != nil {
			fmt.Fprintf(&b, " | Duration: %.1fs", f.Duration)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func recommendScene(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		Vibe  string `json:"vibe"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	params.Vibe = strings.TrimSpace(params.Vibe)
	if params.Vibe == "" {
		return "", fmt.Errorf("vibe is required — describe what you are in the mood for")
	}
	if params.Limit <= 0 || params.Limit > 5 {
		params.Limit = 3
	}

	model := cfg.EmbeddingModel
	if model == "" {
		model = cfg.LLMModel
	}
	if cfg.LLMBaseURL == "" || model == "" {
		return "AI is not configured. Configure the AI base URL and embedding model first.", nil
	}

	client := NewClient(cfg.LLMBaseURL, model)
	vec, err := client.Embedding(ctx, params.Vibe)
	if err != nil {
		return "", fmt.Errorf("embedding the vibe: %w", err)
	}

	results, err := repo.Embedding.SearchSimilar(ctx, "scene", model, vec, params.Limit*3)
	if err != nil {
		return "", fmt.Errorf("searching scenes: %w", err)
	}
	if len(results) == 0 {
		return "No matching scenes found. Run the embedding job first.", nil
	}

	type candidate struct {
		scene *models.Scene
		score float64
		moans bool
	}
	var candidates []candidate
	for _, r := range results {
		s, _ := repo.Scene.Find(ctx, r.EntityID)
		if s == nil {
			continue
		}
		audio, _ := repo.AISceneAudio.FindBySceneID(ctx, s.ID)
		moans := audio != nil && audio.Moans
		score := r.Score
		if moans {
			score += 0.15
		}
		candidates = append(candidates, candidate{scene: s, score: score, moans: moans})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if len(candidates) > params.Limit {
		candidates = candidates[:params.Limit]
	}
	if len(candidates) == 0 {
		return "No matching scenes found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Recommended scenes for %q:\n\n", params.Vibe)
	for _, c := range candidates {
		s := c.scene
		_ = s.LoadPrimaryFile(ctx, repo.File)
		fmt.Fprintf(&b, "- [Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
		if c.moans {
			fmt.Fprintf(&b, " \U0001F525 (moans detected)")
		}
		if f := s.Files.Primary(); f != nil {
			fmt.Fprintf(&b, " | Duration: %.1fs", f.Duration)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nExplain to the user why you picked these and which one you recommend most.")
	return b.String(), nil
}

func searchSemantic(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		Query string `json:"query"`
		Type  string `json:"type"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	model := cfg.EmbeddingModel
	if model == "" {
		model = cfg.LLMModel
	}

	client := NewClient(cfg.LLMBaseURL, model)
	vec, err := client.Embedding(ctx, params.Query)
	if err != nil {
		return "", fmt.Errorf("embedding query: %w", err)
	}

	results, err := repo.Embedding.SearchSimilar(ctx, params.Type, model, vec, params.Limit)
	if err != nil {
		return "", fmt.Errorf("searching: %w", err)
	}

	if len(results) == 0 {
		return "No results found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d results for \"%s\":\n\n", len(results), params.Query)
	for _, r := range results {
		fmt.Fprintf(&b, "- ")
		switch params.Type {
		case "scene":
			s, _ := repo.Scene.Find(ctx, r.EntityID)
			if s != nil {
				fmt.Fprintf(&b, "[Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
			} else {
				fmt.Fprintf(&b, "Scene #%d", r.EntityID)
			}
		case "performer":
			p, _ := repo.Performer.Find(ctx, r.EntityID)
			if p != nil {
				fmt.Fprintf(&b, "[Performer #%d - %s](/performers/%d)", p.ID, p.Name, p.ID)
			} else {
				fmt.Fprintf(&b, "Performer #%d", r.EntityID)
			}
		case "image":
			img, _ := repo.Image.Find(ctx, r.EntityID)
			if img != nil {
				fmt.Fprintf(&b, "[Image #%d - %s](/images/%d)", img.ID, img.Title, img.ID)
			} else {
				fmt.Fprintf(&b, "Image #%d", r.EntityID)
			}
		case "gallery":
			g, _ := repo.Gallery.Find(ctx, r.EntityID)
			if g != nil {
				fmt.Fprintf(&b, "[Gallery #%d - %s](/galleries/%d)", g.ID, g.Title, g.ID)
			} else {
				fmt.Fprintf(&b, "Gallery #%d", r.EntityID)
			}
		case "studio":
			st, _ := repo.Studio.Find(ctx, r.EntityID)
			if st != nil {
				fmt.Fprintf(&b, "[Studio #%d - %s](/studios/%d)", st.ID, st.Name, st.ID)
			} else {
				fmt.Fprintf(&b, "Studio #%d", r.EntityID)
			}
		case "tag":
			t, _ := repo.Tag.Find(ctx, r.EntityID)
			if t != nil {
				fmt.Fprintf(&b, "[Tag #%d - %s](/tags/%d)", t.ID, t.Name, t.ID)
			} else {
				fmt.Fprintf(&b, "Tag #%d", r.EntityID)
			}
		}
		fmt.Fprintf(&b, " | Similarity: %.1f%%\n", r.Score*100)
	}
	return b.String(), nil
}

func searchSimilarImages(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		ImageID int `json:"image_id"`
		Limit   int `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	model := cfg.EmbeddingModel
	if model == "" {
		model = cfg.LLMModel
	}

	vec, err := repo.Embedding.FindByEntity(ctx, "image", params.ImageID, model)
	if err != nil {
		return "", fmt.Errorf("finding embedding: %w", err)
	}
	if vec == nil {
		return "No embedding found for this image. Run the embedding job first.", nil
	}

	results, err := repo.Embedding.SearchSimilar(ctx, "image", model, vec, params.Limit)
	if err != nil {
		return "", fmt.Errorf("searching similar images: %w", err)
	}

	if len(results) == 0 {
		return "No similar images found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Similar images to image #%d:\n\n", params.ImageID)
	for _, r := range results {
		img, _ := repo.Image.Find(ctx, r.EntityID)
		if img == nil {
			continue
		}
		fmt.Fprintf(&b, "- [Image #%d - %s](/images/%d)", img.ID, img.Title, img.ID)
		fmt.Fprintf(&b, " | Similarity: %.1f%%", r.Score*100)
		if r.EntityID == params.ImageID {
			fmt.Fprintf(&b, " (source)")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func recommendContent(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		Query string `json:"query"`
		Type  string `json:"type"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 20 {
		params.Limit = 5
	}

	entityType := "scene"
	if params.Type == "images" {
		entityType = "image"
	}

	model := cfg.EmbeddingModel
	if model == "" {
		model = cfg.LLMModel
	}

	client := NewClient(cfg.LLMBaseURL, model)
	vec, err := client.Embedding(ctx, params.Query)
	if err != nil {
		return "", fmt.Errorf("embedding query: %w", err)
	}

	results, err := repo.Embedding.SearchSimilar(ctx, entityType, model, vec, params.Limit)
	if err != nil {
		return "", fmt.Errorf("searching recommendations: %w", err)
	}

	if len(results) == 0 {
		return "No recommendations found for this query. Try a different description.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Recommended %s\n\n", params.Type)
	fmt.Fprintf(&b, "Based on your interest in: \"%s\"\n\n", params.Query)
	for i, r := range results {
		fmt.Fprintf(&b, "%d. ", i+1)
		switch entityType {
		case "scene":
			s, _ := repo.Scene.Find(ctx, r.EntityID)
			if s != nil {
				fmt.Fprintf(&b, "[Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
				if s.Rating != nil {
					fmt.Fprintf(&b, " (Rating: %d/100)", *s.Rating)
				}
			} else {
				fmt.Fprintf(&b, "Scene #%d", r.EntityID)
			}
		case "image":
			img, _ := repo.Image.Find(ctx, r.EntityID)
			if img != nil {
				fmt.Fprintf(&b, "[Image #%d - %s](/images/%d)", img.ID, img.Title, img.ID)
				if img.Rating != nil {
					fmt.Fprintf(&b, " (Rating: %d/100)", *img.Rating)
				}
			} else {
				fmt.Fprintf(&b, "Image #%d", r.EntityID)
			}
		}
		fmt.Fprintf(&b, " | Relevance: %.1f%%\n", r.Score*100)
	}
	b.WriteString("\n_Recommendations are based on semantic similarity to your query._")
	return b.String(), nil
}

func queryScenes(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Search     string   `json:"search"`
		Tags       []string `json:"tags"`
		TagsMode   string   `json:"tags_mode"`
		Performers []string `json:"performers"`
		Studio     string   `json:"studio"`
		RatingMin  *int     `json:"rating_min"`
		RatingMax  *int     `json:"rating_max"`
		DateFrom   string   `json:"date_from"`
		DateTo     string   `json:"date_to"`
		Organized  *bool    `json:"organized"`
		HasMarkers *bool    `json:"has_markers"`
		Sort       string   `json:"sort"`
		Limit      int      `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}

	filter := &models.FindFilterType{}
	if params.Search != "" {
		filter.Q = &params.Search
	}
	if params.Sort != "" {
		filter.Sort = &params.Sort
	}
	perPage := params.Limit
	filter.PerPage = &perPage

	sceneFilter := &models.SceneFilterType{}

	if params.RatingMin != nil {
		if params.RatingMax != nil {
			value2 := *params.RatingMax
			sceneFilter.Rating100 = &models.IntCriterionInput{
				Value:    *params.RatingMin,
				Value2:   &value2,
				Modifier: models.CriterionModifierBetween,
			}
		} else {
			sceneFilter.Rating100 = &models.IntCriterionInput{
				Value:    *params.RatingMin,
				Modifier: models.CriterionModifierGreaterThan,
			}
		}
	}

	if params.DateFrom != "" && params.DateTo != "" {
		dateTo := params.DateTo
		sceneFilter.Date = &models.DateCriterionInput{
			Value:    params.DateFrom,
			Value2:   &dateTo,
			Modifier: models.CriterionModifierBetween,
		}
	}

	if params.Organized != nil {
		sceneFilter.Organized = params.Organized
	}

	if params.HasMarkers != nil && *params.HasMarkers {
		v := "1"
		sceneFilter.HasMarkers = &v
	}

	if len(params.Tags) > 0 {
		tags, err := repo.Tag.FindByNames(ctx, params.Tags, true)
		if err != nil {
			return "", fmt.Errorf("looking up tags: %w", err)
		}
		if len(tags) > 0 {
			tagIDs := make([]string, len(tags))
			for i, t := range tags {
				tagIDs[i] = fmt.Sprintf("%d", t.ID)
			}
			modifier := models.CriterionModifierIncludes
			if params.TagsMode == "AND" {
				modifier = models.CriterionModifierIncludesAll
			}
			sceneFilter.Tags = &models.HierarchicalMultiCriterionInput{
				Value:    tagIDs,
				Modifier: modifier,
			}
		}
	}

	if len(params.Performers) > 0 {
		performers, err := repo.Performer.FindByNames(ctx, params.Performers, true)
		if err != nil {
			return "", fmt.Errorf("looking up performers: %w", err)
		}
		if len(performers) > 0 {
			performerIDs := make([]string, len(performers))
			for i, p := range performers {
				performerIDs[i] = fmt.Sprintf("%d", p.ID)
			}
			sceneFilter.Performers = &models.MultiCriterionInput{
				Value:    performerIDs,
				Modifier: models.CriterionModifierIncludes,
			}
		}
	}

	if params.Studio != "" {
		studio, err := repo.Studio.FindByName(ctx, params.Studio, true)
		if err != nil {
			return "", fmt.Errorf("looking up studio: %w", err)
		}
		if studio != nil {
			studioIDs := []string{fmt.Sprintf("%d", studio.ID)}
			sceneFilter.Studios = &models.HierarchicalMultiCriterionInput{
				Value:    studioIDs,
				Modifier: models.CriterionModifierIncludes,
			}
		}
	}

	result, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: filter,
			Count:      true,
		},
		SceneFilter: sceneFilter,
	})
	if err != nil {
		return "", fmt.Errorf("querying scenes: %w", err)
	}

	scenes, err := result.Resolve(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving scenes: %w", err)
	}

	if len(scenes) == 0 {
		return fmt.Sprintf("No scenes found matching the criteria (out of %d total matching).", result.Count), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d matching scenes. Showing top %d:\n\n", result.Count, len(scenes))
	for i, s := range scenes {
		studioName := ""
		if s.StudioID != nil {
			st, _ := repo.Studio.Find(ctx, *s.StudioID)
			if st != nil {
				studioName = st.Name
			}
		}
		ratingStr := ""
		if s.Rating != nil {
			ratingStr = fmt.Sprintf(" [Rating: %d/100]", *s.Rating)
		}
		dateStr := ""
		if s.Date != nil {
			dateStr = fmt.Sprintf(" [%s]", s.Date.String())
		}
		orgStr := ""
		if s.Organized {
			orgStr = " [Organized]"
		}
		fmt.Fprintf(&b, "%d. [Scene #%d](/scenes/%d)", i+1, s.ID, s.ID)
		if s.Title != "" {
			fmt.Fprintf(&b, " - %s", s.Title)
		}
		fmt.Fprintf(&b, "%s%s%s", dateStr, ratingStr, orgStr)
		if studioName != "" {
			fmt.Fprintf(&b, " [Studio: %s]", studioName)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func GetTools(cfg ToolConfig) []Tool {
	return []Tool{
		{
			Name:        "describe_image",
			Description: "Analyze and describe the visual content of an image or scene cover using AI vision. Returns a text description of people, setting, objects, colors, and other visual features. Use this when you need to understand what an image contains or to find images without sufficient text metadata.",
			Parameters:  describeImageParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return describeImage(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "describe_scene",
			Description: "Analyze and describe the visual content of a scene (video) using AI vision. Extracts frames from the video and analyzes them to describe people, actions, setting, and other visual elements. Use this when you need to understand what a video scene contains, or to find scenes without sufficient text metadata.",
			Parameters:  describeSceneParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return describeScene(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "search_scenes",
			Description: "Search for scenes by title, details, tags, or performers. Returns a list of matching scenes with key metadata.",
			Parameters:  searchScenesParam,
			Execute:     searchScenes,
		},
		{
			Name:        "get_scene",
			Description: "Get full details for a specific scene by its ID.",
			Parameters:  getSceneParam,
			Execute:     getScene,
		},
		{
			Name:        "search_performers",
			Description: "Search for performers by name or aliases.",
			Parameters:  searchPerformersParam,
			Execute:     searchPerformers,
		},
		{
			Name:        "search_images",
			Description: "Search for images by title, details, tags, or performers.",
			Parameters:  searchImagesParam,
			Execute:     searchImages,
		},
		{
			Name:        "search_galleries",
			Description: "Search for galleries by title, details, tags, or performers.",
			Parameters:  searchGalleriesParam,
			Execute:     searchGalleries,
		},
		{
			Name:        "count_by_tag",
			Description: "Count scenes and images that have a specific tag.",
			Parameters:  countByTagParam,
			Execute:     countByTag,
		},
		{
			Name:        "get_library_stats",
			Description: "Get overall library statistics (counts of scenes, images, performers, etc.).",
			Parameters:  getStatsParam,
			Execute:     getStats,
		},
		{
			Name:        "get_recent_scenes",
			Description: "Get the most recently added scenes, sorted by creation date descending.",
			Parameters:  getRecentScenesParam,
			Execute:     getRecentScenes,
		},
		{
			Name:        "search_studios",
			Description: "Search for studios by name.",
			Parameters:  searchStudiosParam,
			Execute:     searchStudios,
		},
		{
			Name:        "search_tags",
			Description: "Search for tags by name.",
			Parameters:  searchTagsParam,
			Execute:     searchTags,
		},
		{
			Name:        "get_scene_markers",
			Description: "Get markers/segments for a specific scene by its ID.",
			Parameters:  getSceneMarkersParam,
			Execute:     getSceneMarkers,
		},
		{
			Name:        "find_scenes_by_performer",
			Description: "Find all scenes featuring a specific performer, by performer ID or name.",
			Parameters:  findScenesByPerformerParam,
			Execute:     findScenesByPerformer,
		},
		{
			Name:        "get_top_rated",
			Description: "Get the top-rated scenes or images, sorted by rating descending.",
			Parameters:  getTopRatedParam,
			Execute:     getTopRated,
		},
		{
			Name:        "get_top_viewed_scenes",
			Description: "Get the most viewed (by o_counter) scenes, sorted by orgasm count descending.",
			Parameters:  getTopViewedParam,
			Execute:     getTopViewedScenes,
		},
		{
			Name:        "get_top_played_scenes",
			Description: "Get the most played scenes, sorted by play count descending.",
			Parameters:  getTopPlayedParam,
			Execute:     getTopPlayedScenes,
		},
		{
			Name:        "get_performer_details",
			Description: "Get full details and statistics for a specific performer by ID.",
			Parameters:  getPerformerDetailsParam,
			Execute:     getPerformerDetails,
		},
		{
			Name:        "get_scene_file_details",
			Description: "Get detailed file information (codec, resolution, bitrate, etc.) for a specific scene.",
			Parameters:  getSceneFileDetailsParam,
			Execute:     getSceneFileDetails,
		},
		{
			Name:        "search_groups",
			Description: "Search for groups/movies by name.",
			Parameters:  searchGroupsParam,
			Execute:     searchGroups,
		},
		{
			Name:        "mark_scene_organized",
			Description: "Mark a scene as organized or unorganized. This changes the organized flag on the scene.",
			Parameters:  markSceneOrganizedParam,
			Execute:     markSceneOrganized,
		},
		{
			Name:        "update_entity_title",
			Description: "Update the title or name of a scene, image, gallery, performer, studio, tag, or group.",
			Parameters:  updateEntityTitleParam,
			Execute:     updateEntityTitle,
		},
		{
			Name:        "generate_image",
			Description: "Generate a new image using AI (Automatic1111). Omit source_type and source_id for pure text-to-image generation, or provide a scene or image ID to use as the base for img2img generation with your text prompt. To use LoRAs, include them in the prompt using the syntax <lora:lora_name:weight> (e.g., <lora:my_lora:0.8>). Use list_a1111_loras to see available LoRAs.",
			Parameters:  generateImageParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return generateImage(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "list_a1111_models",
			Description: "List all available Stable Diffusion models in Automatic1111. Use this to see what models are available for image generation.",
			Parameters:  listA1111ModelsParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return listA1111Models(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "list_a1111_loras",
			Description: "List all available LoRAs in Automatic1111. Use this to see what LoRAs are available for image generation. Note: Requires an A1111 version/extension that exposes the /sdapi/v1/loras endpoint.",
			Parameters:  listA1111LorasParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return listA1111Loras(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "search_similar_scenes",
			Description: "Find scenes that are semantically similar to a given scene based on their metadata. Uses AI embeddings to find scenes with related titles, tags, performers, and descriptions. Provide a scene ID to find similar content.",
			Parameters:  searchSimilarScenesParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return searchSimilarScenes(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "recommend_scene",
			Description: "Recommend scenes from the library that match a mood or vibe description (e.g. \"brunette, bondage, rough\"). Uses embedding search and boosts scenes with detected moans. Use this when the user asks for something to watch or is in the mood for something specific.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"vibe": map[string]interface{}{
						"type":        "string",
						"description": "What the user is in the mood for: performers, acts, looks, mood.",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Number of scenes to recommend (1-5). Default 3.",
					},
				},
				"required": []string{"vibe"},
			},
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return recommendScene(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "search_semantic",
			Description: "Search your library using natural language descriptions instead of exact keywords. Works across all entity types (scenes, performers, images, galleries, studios, tags). For example: 'dark moody scenes with jazz music' or 'european performers with blonde hair'. Requires AI embeddings to be generated first.",
			Parameters:  searchSemanticParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return searchSemantic(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "search_similar_images",
			Description: "Find images that are semantically similar to a given image based on their metadata (title, tags, performers, details). Provide an image ID to find related content.",
			Parameters:  searchSimilarImagesParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return searchSimilarImages(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "batch_update_scenes",
			Description: "Update multiple scenes at once. First call shows a preview of how many scenes match the filters. Call again with confirmed=true to execute. Supports filtering by query, tags, studio, performers, rating range, and organized status. Supports adding/removing tags and performers, setting rating, organized flag, and studio.",
			Parameters:  batchSceneUpdateParam,
			Execute:     batchUpdateScenes,
		},
		{
			Name:        "remember",
			Description: "Save a piece of information that the AI should remember across all chat sessions. Use this to store user preferences, likes, dislikes, or any context that should persist. For example: preferred genres, disliked performers, or content preferences.",
			Parameters:  rememberParam,
			Execute:     remember,
		},
		{
			Name:        "forget",
			Description: "Delete a previously saved memory by its key name. Use list_memories to see all saved keys.",
			Parameters:  forgetParam,
			Execute:     forget,
		},
		{
			Name:        "list_memories",
			Description: "List all saved memories/preferences that the AI remembers about the user.",
			Parameters:  emptyParam,
			Execute:     listMemories,
		},
		{
			Name:        "recommend",
			Description: "Recommend scenes or images based on what the user might enjoy. Takes a natural language description of the type of content they want. Uses AI to find the best semantic matches. Examples: 'lesbian scenes from 2023', 'artistic black and white photos', 'intense BDSM content'. Returns personalized recommendations with relevance scores.",
			Parameters:  recommendParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return recommendContent(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "query_scenes",
			Description: "Query scenes by metadata criteria. Use this to answer questions about what scenes exist in the library. Supports filtering by text search, tags, performers, studio, rating range, date range, organized status, and markers. Examples: 'how many scenes from 2023 with rating > 80', 'find scenes with performer X and tag Y'. Returns count and list of matches.",
			Parameters:  queryScenesParam,
			Execute:     queryScenes,
		},
		{
			Name:        "merge_tags",
			Description: "Merge duplicate tags into a single canonical tag. Takes a list of source tag names and a destination tag name. The source tags are deleted and their scenes/images reassigned to the destination; source tag names become aliases of the destination. If the destination tag does not exist, it is created. First call shows a preview of affected scenes/images, call again with confirmed=true to execute. Use find_duplicate_tags to find candidates.",
			Parameters:  mergeTagsParam,
			Execute:     mergeTags,
		},
		{
			Name:        "update_tag",
			Description: "Update a tag's name, description, aliases, parent tags (making it a sub-tag), or child tags (adding sub-tags). Resolves tags by name or alias. Use this to organize the tag hierarchy and consolidate aliases. Example: give tag 'POV' the alias 'Point of view' and make it a child of 'Video techniques'.",
			Parameters:  updateTagParam,
			Execute:     updateTag,
		},
		{
			Name:        "create_tag",
			Description: "Create a new tag with an optional description, aliases, and parent tags. Use this to establish a canonical tag before tagging content or to build out a tag hierarchy.",
			Parameters:  createTagParam,
			Execute:     createTag,
		},
		{
			Name:        "find_duplicate_tags",
			Description: "Find tags that share aliases and are likely duplicates of each other. Groups tags into clusters based on shared aliases and reports the combined scene/image counts, so you can decide which to merge with merge_tags.",
			Parameters:  findDuplicateTagsParam,
			Execute:     findDuplicateTags,
		},
		{
			Name:        "generate_embeddings",
			Description: "Generate AI embeddings for library entities (scenes, performers, images, galleries, studios, tags) to enable semantic search and similarity matching. Specify which entity types to process and whether to overwrite existing embeddings. Returns a job ID that can be tracked in the task queue.",
			Parameters:  generateEmbeddingsParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return generateEmbeddings(ctx, repo, args, cfg)
			},
		},
		{
			Name:        "segment_scene",
			Description: "Run AI scene segmentation on a scene or multiple scenes. This analyzes the video content and creates markers/segments with titles and descriptions. Provide scene IDs to segment, or leave empty to segment all unsegmented scenes. Returns a job ID that can be tracked in the task queue.",
			Parameters:  segmentSceneParam,
			Execute: func(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
				return segmentScene(ctx, repo, args, cfg)
			},
		},
	}
}

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

var getRecentScenesParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
}

var searchStudiosParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Search query for studio name",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"query"},
}

var searchTagsParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Search query for tag name",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"query"},
}

var getSceneMarkersParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"scene_id": map[string]interface{}{
			"type":        "integer",
			"description": "Scene ID to fetch markers for",
		},
	},
	"required": []string{"scene_id"},
}

var findScenesByPerformerParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"performer_name": map[string]interface{}{
			"type":        "string",
			"description": "Performer name to search for (alternative to performer_id)",
		},
		"performer_id": map[string]interface{}{
			"type":        "integer",
			"description": "Performer ID (alternative to performer_name)",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
}

var getTopRatedParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"type": map[string]interface{}{
			"type":        "string",
			"description": "Type of media: 'scene' or 'image'",
			"enum":        []string{"scene", "image"},
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"type"},
}

var getTopViewedParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
}

var getTopPlayedParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
}

var getPerformerDetailsParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"performer_id": map[string]interface{}{
			"type":        "integer",
			"description": "Performer ID",
		},
	},
	"required": []string{"performer_id"},
}

var getSceneFileDetailsParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"scene_id": map[string]interface{}{
			"type":        "integer",
			"description": "Scene ID",
		},
	},
	"required": []string{"scene_id"},
}

var searchGroupsParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Search query for group/movie name",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of results (default 10, max 50)",
			"default":     10,
		},
	},
	"required": []string{"query"},
}

var markSceneOrganizedParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"scene_id": map[string]interface{}{
			"type":        "integer",
			"description": "Scene ID to mark",
		},
		"organized": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether the scene should be organized (true) or unorganized (false)",
		},
	},
	"required": []string{"scene_id", "organized"},
}

var updateEntityTitleParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"entity_type": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"scene", "image", "gallery", "performer", "studio", "tag", "group"},
			"description": "Type of entity to update (scene, image, gallery, performer, studio, tag, or group)",
		},
		"entity_id": map[string]interface{}{
			"type":        "integer",
			"description": "ID of the entity to update",
		},
		"title": map[string]interface{}{
			"type":        "string",
			"description": "New title (or name for performers/studios/tags/groups)",
		},
	},
	"required": []string{"entity_type", "entity_id", "title"},
}

func updateEntityTitle(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		EntityType string `json:"entity_type"`
		EntityID   int    `json:"entity_id"`
		Title      string `json:"title"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	var result string
	if err := repo.WithTxn(ctx, func(ctx context.Context) error {
		switch params.EntityType {
		case "scene":
			s, err := repo.Scene.Find(ctx, params.EntityID)
			if err != nil {
				return fmt.Errorf("finding scene: %w", err)
			}
			if s == nil {
				return fmt.Errorf("scene with ID %d not found", params.EntityID)
			}
			_, err = repo.Scene.UpdatePartial(ctx, params.EntityID, models.ScenePartial{
				Title: models.NewOptionalString(params.Title),
			})
			if err != nil {
				return fmt.Errorf("updating scene: %w", err)
			}
			result = fmt.Sprintf("Updated title for [Scene #%d - %s](/scenes/%d) to \"%s\".", s.ID, s.Title, s.ID, params.Title)
		case "image":
			i, err := repo.Image.Find(ctx, params.EntityID)
			if err != nil {
				return fmt.Errorf("finding image: %w", err)
			}
			if i == nil {
				return fmt.Errorf("image with ID %d not found", params.EntityID)
			}
			_, err = repo.Image.UpdatePartial(ctx, params.EntityID, models.ImagePartial{
				Title: models.NewOptionalString(params.Title),
			})
			if err != nil {
				return fmt.Errorf("updating image: %w", err)
			}
			result = fmt.Sprintf("Updated title for [Image #%d - %s](/images/%d) to \"%s\".", i.ID, i.Title, i.ID, params.Title)
		case "gallery":
			g, err := repo.Gallery.Find(ctx, params.EntityID)
			if err != nil {
				return fmt.Errorf("finding gallery: %w", err)
			}
			if g == nil {
				return fmt.Errorf("gallery with ID %d not found", params.EntityID)
			}
			_, err = repo.Gallery.UpdatePartial(ctx, params.EntityID, models.GalleryPartial{
				Title: models.NewOptionalString(params.Title),
			})
			if err != nil {
				return fmt.Errorf("updating gallery: %w", err)
			}
			result = fmt.Sprintf("Updated title for [Gallery #%d - %s](/galleries/%d) to \"%s\".", g.ID, g.Title, g.ID, params.Title)
		case "performer":
			p, err := repo.Performer.Find(ctx, params.EntityID)
			if err != nil {
				return fmt.Errorf("finding performer: %w", err)
			}
			if p == nil {
				return fmt.Errorf("performer with ID %d not found", params.EntityID)
			}
			_, err = repo.Performer.UpdatePartial(ctx, params.EntityID, models.PerformerPartial{
				Name: models.NewOptionalString(params.Title),
			})
			if err != nil {
				return fmt.Errorf("updating performer: %w", err)
			}
			result = fmt.Sprintf("Updated name for [Performer #%d - %s](/performers/%d) to \"%s\".", p.ID, p.Name, p.ID, params.Title)
		case "studio":
			st, err := repo.Studio.Find(ctx, params.EntityID)
			if err != nil {
				return fmt.Errorf("finding studio: %w", err)
			}
			if st == nil {
				return fmt.Errorf("studio with ID %d not found", params.EntityID)
			}
			_, err = repo.Studio.UpdatePartial(ctx, models.StudioPartial{
				ID:   params.EntityID,
				Name: models.NewOptionalString(params.Title),
			})
			if err != nil {
				return fmt.Errorf("updating studio: %w", err)
			}
			result = fmt.Sprintf("Updated name for [Studio #%d - %s](/studios/%d) to \"%s\".", st.ID, st.Name, st.ID, params.Title)
		case "tag":
			t, err := repo.Tag.Find(ctx, params.EntityID)
			if err != nil {
				return fmt.Errorf("finding tag: %w", err)
			}
			if t == nil {
				return fmt.Errorf("tag with ID %d not found", params.EntityID)
			}
			_, err = repo.Tag.UpdatePartial(ctx, params.EntityID, models.TagPartial{
				Name: models.NewOptionalString(params.Title),
			})
			if err != nil {
				return fmt.Errorf("updating tag: %w", err)
			}
			result = fmt.Sprintf("Updated name for [Tag #%d - %s](/tags/%d) to \"%s\".", t.ID, t.Name, t.ID, params.Title)
		case "group":
			g, err := repo.Group.Find(ctx, params.EntityID)
			if err != nil {
				return fmt.Errorf("finding group: %w", err)
			}
			if g == nil {
				return fmt.Errorf("group with ID %d not found", params.EntityID)
			}
			_, err = repo.Group.UpdatePartial(ctx, params.EntityID, models.GroupPartial{
				Name: models.NewOptionalString(params.Title),
			})
			if err != nil {
				return fmt.Errorf("updating group: %w", err)
			}
			result = fmt.Sprintf("Updated name for [Group #%d - %s](/groups/%d) to \"%s\".", g.ID, g.Name, g.ID, params.Title)
		default:
			return fmt.Errorf("unknown entity type %q; must be one of: scene, image, gallery, performer, studio, tag, group", params.EntityType)
		}
		return nil
	}); err != nil {
		return "", err
	}

	return result, nil
}

func getRecentScenes(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	sort := "created_at"
	desc := models.SortDirectionEnumDesc
	filter := &models.FindFilterType{
		PerPage:   &params.Limit,
		Sort:      &sort,
		Direction: &desc,
	}

	result, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: filter,
			Count:      false,
		},
	})
	if err != nil {
		return "", fmt.Errorf("querying scenes: %w", err)
	}

	scenes, err := result.Resolve(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving scenes: %w", err)
	}

	if len(scenes) == 0 {
		return "No scenes found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d recent scenes:\n\n", len(scenes))
	for _, s := range scenes {
		_ = s.LoadPrimaryFile(ctx, repo.File)
		_ = s.LoadPerformerIDs(ctx, repo.Scene)
		_ = s.LoadTagIDs(ctx, repo.Scene)

		fmt.Fprintf(&b, "- [Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
		if s.Date != nil {
			fmt.Fprintf(&b, " | Date: %s", s.Date.String())
		}
		fmt.Fprintf(&b, " | Created: %s", s.CreatedAt.Format("2006-01-02"))
		if len(s.PerformerIDs.List()) > 0 {
			performers, _ := repo.Performer.FindMany(ctx, s.PerformerIDs.List())
			var names []string
			for _, p := range performers {
				names = append(names, p.Name)
			}
			fmt.Fprintf(&b, " | Performers: %s", strings.Join(names, ", "))
		}
		if f := s.Files.Primary(); f != nil {
			fmt.Fprintf(&b, " | Duration: %.1fs", f.Duration)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func searchStudios(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	q := params.Query
	filter := &models.FindFilterType{
		Q:       &q,
		PerPage: &params.Limit,
		Sort:    strPtr("name"),
	}

	studios, _, err := repo.Studio.Query(ctx, nil, filter)
	if err != nil {
		return "", fmt.Errorf("querying studios: %w", err)
	}

	if len(studios) == 0 {
		return "No studios found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d studios:\n\n", len(studios))
	for _, st := range studios {
		fmt.Fprintf(&b, "- [Studio #%d - %s](/studios/%d)", st.ID, st.Name, st.ID)
		if st.Rating != nil {
			fmt.Fprintf(&b, " | Rating: %d/100", *st.Rating)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func searchTags(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	q := params.Query
	filter := &models.FindFilterType{
		Q:       &q,
		PerPage: &params.Limit,
		Sort:    strPtr("name"),
	}

	tags, _, err := repo.Tag.Query(ctx, nil, filter)
	if err != nil {
		return "", fmt.Errorf("querying tags: %w", err)
	}

	if len(tags) == 0 {
		return "No tags found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d tags:\n\n", len(tags))
	for _, t := range tags {
		_ = t.LoadAliases(ctx, repo.Tag)
		_ = t.LoadParentIDs(ctx, repo.Tag)
		_ = t.LoadChildIDs(ctx, repo.Tag)

		fmt.Fprintf(&b, "- [Tag #%d - %s](/tags/%d)", t.ID, t.Name, t.ID)
		if t.Description != "" {
			fmt.Fprintf(&b, " | Description: %s", t.Description)
		}
		if len(t.Aliases.List()) > 0 {
			fmt.Fprintf(&b, " | Aliases: %s", strings.Join(t.Aliases.List(), ", "))
		}
		if len(t.ParentIDs.List()) > 0 {
			parents, _ := repo.Tag.FindMany(ctx, t.ParentIDs.List())
			var names []string
			for _, p := range parents {
				names = append(names, p.Name)
			}
			fmt.Fprintf(&b, " | Parents: %s", strings.Join(names, ", "))
		}
		if len(t.ChildIDs.List()) > 0 {
			children, _ := repo.Tag.FindMany(ctx, t.ChildIDs.List())
			var names []string
			for _, c := range children {
				names = append(names, c.Name)
			}
			fmt.Fprintf(&b, " | Children: %s", strings.Join(names, ", "))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func findTagByNameOrAlias(ctx context.Context, repo models.Repository, name string) (*models.Tag, error) {
	t, err := tag.ByName(ctx, repo.Tag, name)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return t, nil
	}
	return tag.ByAlias(ctx, repo.Tag, name)
}

func tagSceneImageCounts(ctx context.Context, repo models.Repository, tagID int) (int, int) {
	sceneFilter := &models.SceneFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{fmt.Sprintf("%d", tagID)},
			Modifier: models.CriterionModifierIncludes,
		},
	}
	sceneResult, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: &models.FindFilterType{PerPage: intPtr(0)},
			Count:      true,
		},
		SceneFilter: sceneFilter,
	})
	sceneCount := 0
	if err == nil {
		sceneCount = sceneResult.Count
	}

	imageFilter := &models.ImageFilterType{
		Tags: &models.HierarchicalMultiCriterionInput{
			Value:    []string{fmt.Sprintf("%d", tagID)},
			Modifier: models.CriterionModifierIncludes,
		},
	}
	imageResult, err := repo.Image.Query(ctx, models.ImageQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: &models.FindFilterType{PerPage: intPtr(0)},
			Count:      true,
		},
		ImageFilter: imageFilter,
	})
	imageCount := 0
	if err == nil {
		imageCount = imageResult.Count
	}

	return sceneCount, imageCount
}

var mergeTagsParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"source_names": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Names (or aliases) of the tags to merge into the destination. These tags will be deleted and their media reassigned to the destination tag.",
		},
		"destination_name": map[string]interface{}{
			"type":        "string",
			"description": "Name of the canonical tag to merge into. If a tag with this name does not exist, it will be created. Source tag names become aliases of the destination.",
		},
		"confirmed": map[string]interface{}{
			"type":        "boolean",
			"description": "Set to true to confirm and execute the merge. First call without confirmed shows a preview of how many scenes/images are affected.",
		},
	},
	"required": []string{"source_names", "destination_name"},
}

func mergeTags(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		SourceNames     []string `json:"source_names"`
		DestinationName string   `json:"destination_name"`
		Confirmed       bool     `json:"confirmed"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if len(params.SourceNames) == 0 {
		return "Please provide at least one source tag name to merge.", nil
	}
	params.DestinationName = strings.TrimSpace(params.DestinationName)
	if params.DestinationName == "" {
		return "destination_name is required.", nil
	}

	var sourceIDs []int
	var sourceTags []*models.Tag
	for _, name := range params.SourceNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		t, err := findTagByNameOrAlias(ctx, repo, name)
		if err != nil {
			return "", fmt.Errorf("finding tag %q: %w", name, err)
		}
		if t == nil {
			return fmt.Sprintf("Tag %q not found. Use search_tags to find existing tags.", name), nil
		}
		sourceIDs = append(sourceIDs, t.ID)
		sourceTags = append(sourceTags, t)
	}
	if len(sourceIDs) == 0 {
		return "No valid source tags provided.", nil
	}

	destTag, err := findTagByNameOrAlias(ctx, repo, params.DestinationName)
	if err != nil {
		return "", fmt.Errorf("finding destination tag: %w", err)
	}

	if !params.Confirmed {
		var b strings.Builder
		if destTag == nil {
			fmt.Fprintf(&b, "Destination tag %q does not exist yet and will be created.\n", params.DestinationName)
		} else {
			fmt.Fprintf(&b, "Destination: [Tag #%d - %s](/tags/%d)\n", destTag.ID, destTag.Name, destTag.ID)
		}
		fmt.Fprintf(&b, "\nSource tags to merge:\n")
		totalScenes := 0
		totalImages := 0
		for _, t := range sourceTags {
			scenes, images := tagSceneImageCounts(ctx, repo, t.ID)
			totalScenes += scenes
			totalImages += images
			fmt.Fprintf(&b, "- [Tag #%d - %s](/tags/%d): %d scenes, %d images\n", t.ID, t.Name, t.ID, scenes, images)
		}
		fmt.Fprintf(&b, "\nThis merge would move %d scenes and %d images to the destination tag. The source tags would be deleted and their names added as aliases of the destination.\n", totalScenes, totalImages)
		fmt.Fprintf(&b, "Call again with `\"confirmed\": true` to execute.")
		return b.String(), nil
	}

	var result string
	if err := repo.WithTxn(ctx, func(ctx context.Context) error {
		destID := 0
		if destTag == nil {
			newTag := models.NewTag()
			newTag.Name = params.DestinationName
			newTag.ParentIDs = models.NewRelatedIDs([]int{})
			newTag.ChildIDs = models.NewRelatedIDs([]int{})
			newTag.Aliases = models.NewRelatedStrings([]string{})
			input := &models.CreateTagInput{
				Tag: &newTag,
			}
			if err := tag.ValidateCreate(ctx, newTag, repo.Tag); err != nil {
				return fmt.Errorf("validating destination tag: %w", err)
			}
			if err := repo.Tag.Create(ctx, input); err != nil {
				return fmt.Errorf("creating destination tag: %w", err)
			}
			destID = newTag.ID
		} else {
			destID = destTag.ID
		}

		for _, id := range sourceIDs {
			if id == destID {
				return fmt.Errorf("cannot merge a tag into itself: %q", params.DestinationName)
			}
		}

		if err := repo.Tag.Merge(ctx, sourceIDs, destID); err != nil {
			return fmt.Errorf("merging tags: %w", err)
		}
		result = fmt.Sprintf("Merged tags into [Tag #%d - %s](/tags/%d). Source tags deleted, their names added as aliases.", destID, params.DestinationName, destID)
		return nil
	}); err != nil {
		return "", err
	}

	return result, nil
}

var updateTagParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"tag_name": map[string]interface{}{
			"type":        "string",
			"description": "Name or alias of the tag to update",
		},
		"new_name": map[string]interface{}{
			"type":        "string",
			"description": "New name for the tag",
		},
		"description": map[string]interface{}{
			"type":        "string",
			"description": "New description for the tag",
		},
		"set_aliases": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Replace all aliases with this list",
		},
		"add_aliases": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Aliases to add to the tag",
		},
		"remove_aliases": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Aliases to remove from the tag",
		},
		"add_parents": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Names of parent tags to add (this tag becomes a sub-tag of these)",
		},
		"remove_parents": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Names of parent tags to remove",
		},
		"add_children": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Names of child tags to add (these become sub-tags of this tag)",
		},
		"remove_children": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Names of child tags to remove",
		},
	},
	"required": []string{"tag_name"},
}

func updateTag(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		TagName        string   `json:"tag_name"`
		NewName        string   `json:"new_name"`
		Description    string   `json:"description"`
		SetAliases     []string `json:"set_aliases"`
		AddAliases     []string `json:"add_aliases"`
		RemoveAliases  []string `json:"remove_aliases"`
		AddParents     []string `json:"add_parents"`
		RemoveParents  []string `json:"remove_parents"`
		AddChildren    []string `json:"add_children"`
		RemoveChildren []string `json:"remove_children"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	params.TagName = strings.TrimSpace(params.TagName)
	if params.TagName == "" {
		return "tag_name is required.", nil
	}

	t, err := findTagByNameOrAlias(ctx, repo, params.TagName)
	if err != nil {
		return "", fmt.Errorf("finding tag: %w", err)
	}
	if t == nil {
		return fmt.Sprintf("Tag %q not found. Use search_tags to find existing tags.", params.TagName), nil
	}

	if params.NewName == "" && params.Description == "" && len(params.SetAliases) == 0 && len(params.AddAliases) == 0 && len(params.RemoveAliases) == 0 && len(params.AddParents) == 0 && len(params.RemoveParents) == 0 && len(params.AddChildren) == 0 && len(params.RemoveChildren) == 0 {
		return "No changes provided. Provide at least one of: new_name, description, set_aliases, add_aliases, remove_aliases, add_parents, remove_parents, add_children, remove_children.", nil
	}

	resolveNames := func(names []string) ([]int, error) {
		var ids []int
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			nt, err := findTagByNameOrAlias(ctx, repo, n)
			if err != nil {
				return nil, err
			}
			if nt == nil {
				return nil, fmt.Errorf("tag %q not found; use search_tags to find existing tags", n)
			}
			ids = append(ids, nt.ID)
		}
		return ids, nil
	}

	partial := models.NewTagPartial()

	if params.NewName != "" {
		partial.Name = models.NewOptionalString(params.NewName)
	}
	if params.Description != "" {
		partial.Description = models.NewOptionalString(params.Description)
	}

	if len(params.SetAliases) > 0 || len(params.AddAliases) > 0 || len(params.RemoveAliases) > 0 {
		mode := models.RelationshipUpdateModeSet
		values := params.SetAliases
		if len(params.SetAliases) == 0 {
			mode = models.RelationshipUpdateModeAdd
			values = params.AddAliases
			if len(params.AddAliases) == 0 {
				mode = models.RelationshipUpdateModeRemove
				values = params.RemoveAliases
			}
		}

		if err := t.LoadAliases(ctx, repo.Tag); err != nil {
			return "", fmt.Errorf("loading aliases: %w", err)
		}
		name := t.Name
		if params.NewName != "" {
			name = params.NewName
		}
		applied := (&models.UpdateStrings{Values: values, Mode: mode}).Apply(t.Aliases.List())
		sanitized := stringslice.UniqueExcludeFold(applied, name)
		partial.Aliases = &models.UpdateStrings{Values: sanitized, Mode: models.RelationshipUpdateModeSet}
	}

	if len(params.AddParents) > 0 || len(params.RemoveParents) > 0 {
		if err := t.LoadParentIDs(ctx, repo.Tag); err != nil {
			return "", fmt.Errorf("loading parent tags: %w", err)
		}
		addIDs, err := resolveNames(params.AddParents)
		if err != nil {
			return "", err
		}
		removeIDs, err := resolveNames(params.RemoveParents)
		if err != nil {
			return "", err
		}
		current := sliceutil.AppendUniques(t.ParentIDs.List(), addIDs)
		current = sliceutil.Exclude(current, removeIDs)
		partial.ParentIDs = &models.UpdateIDs{IDs: current, Mode: models.RelationshipUpdateModeSet}
	}

	if len(params.AddChildren) > 0 || len(params.RemoveChildren) > 0 {
		if err := t.LoadChildIDs(ctx, repo.Tag); err != nil {
			return "", fmt.Errorf("loading child tags: %w", err)
		}
		addIDs, err := resolveNames(params.AddChildren)
		if err != nil {
			return "", err
		}
		removeIDs, err := resolveNames(params.RemoveChildren)
		if err != nil {
			return "", err
		}
		current := sliceutil.AppendUniques(t.ChildIDs.List(), addIDs)
		current = sliceutil.Exclude(current, removeIDs)
		partial.ChildIDs = &models.UpdateIDs{IDs: current, Mode: models.RelationshipUpdateModeSet}
	}

	var result string
	if err := repo.WithTxn(ctx, func(ctx context.Context) error {
		if err := tag.ValidateUpdate(ctx, t.ID, partial, repo.Tag); err != nil {
			return err
		}
		if _, err := repo.Tag.UpdatePartial(ctx, t.ID, partial); err != nil {
			return fmt.Errorf("updating tag: %w", err)
		}
		result = fmt.Sprintf("Updated [Tag #%d - %s](/tags/%d).", t.ID, t.Name, t.ID)
		return nil
	}); err != nil {
		return "", err
	}

	return result, nil
}

var createTagParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"name": map[string]interface{}{
			"type":        "string",
			"description": "Name of the new tag",
		},
		"aliases": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Aliases for the new tag",
		},
		"parent_names": map[string]interface{}{
			"type":        "array",
			"items":       map[string]interface{}{"type": "string"},
			"description": "Names of existing tags that should be parents of this tag",
		},
		"description": map[string]interface{}{
			"type":        "string",
			"description": "Description for the new tag",
		},
	},
	"required": []string{"name"},
}

func createTag(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Name        string   `json:"name"`
		Aliases     []string `json:"aliases"`
		ParentNames []string `json:"parent_names"`
		Description string   `json:"description"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return "name is required.", nil
	}

	newTag := models.NewTag()
	newTag.Name = params.Name
	newTag.Aliases = models.NewRelatedStrings(stringslice.UniqueExcludeFold(stringslice.TrimSpace(params.Aliases), params.Name))
	newTag.Description = params.Description

	var parentIDs []int
	for _, p := range params.ParentNames {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pt, err := findTagByNameOrAlias(ctx, repo, p)
		if err != nil {
			return "", fmt.Errorf("finding parent tag %q: %w", p, err)
		}
		if pt == nil {
			return fmt.Sprintf("Parent tag %q not found; use search_tags to find existing tags.", p), nil
		}
		parentIDs = append(parentIDs, pt.ID)
	}
	if parentIDs == nil {
		parentIDs = []int{}
	}
	newTag.ParentIDs = models.NewRelatedIDs(parentIDs)
	newTag.ChildIDs = models.NewRelatedIDs([]int{})

	if err := repo.WithTxn(ctx, func(ctx context.Context) error {
		if err := tag.ValidateCreate(ctx, newTag, repo.Tag); err != nil {
			return err
		}
		input := &models.CreateTagInput{
			Tag: &newTag,
		}
		if err := repo.Tag.Create(ctx, input); err != nil {
			return fmt.Errorf("creating tag: %w", err)
		}
		return nil
	}); err != nil {
		return "", err
	}

	return fmt.Sprintf("Created [Tag #%d - %s](/tags/%d).", newTag.ID, newTag.Name, newTag.ID), nil
}

var findDuplicateTagsParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"min_shared_aliases": map[string]interface{}{
			"type":        "integer",
			"description": "Minimum number of shared aliases to consider two tags duplicates (default 1)",
			"default":     1,
		},
	},
}

func findDuplicateTags(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		MinSharedAliases int `json:"min_shared_aliases"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.MinSharedAliases <= 0 {
		params.MinSharedAliases = 1
	}

	tags, err := repo.Tag.All(ctx)
	if err != nil {
		return "", fmt.Errorf("querying tags: %w", err)
	}

	type tagInfo struct {
		tag     *models.Tag
		aliases map[string]struct{}
	}

	infos := make([]tagInfo, 0, len(tags))
	for _, t := range tags {
		if err := t.LoadAliases(ctx, repo.Tag); err != nil {
			return "", fmt.Errorf("loading aliases for tag %d: %w", t.ID, err)
		}
		aliasSet := make(map[string]struct{}, len(t.Aliases.List()))
		for _, a := range t.Aliases.List() {
			key := strings.ToLower(strings.TrimSpace(a))
			if key == "" {
				continue
			}
			aliasSet[key] = struct{}{}
		}
		infos = append(infos, tagInfo{tag: t, aliases: aliasSet})
	}

	// union-find over tags sharing at least min_shared_aliases aliases
	parent := make(map[int]int, len(tags))
	for _, info := range infos {
		parent[info.tag.ID] = info.tag.ID
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	for i := range infos {
		for j := i + 1; j < len(infos); j++ {
			shared := 0
			for alias := range infos[i].aliases {
				if _, ok := infos[j].aliases[alias]; ok {
					shared++
				}
			}
			if shared >= params.MinSharedAliases {
				union(infos[i].tag.ID, infos[j].tag.ID)
			}
		}
	}

	// collect clusters
	clusters := make(map[int][]*models.Tag)
	for _, info := range infos {
		root := find(info.tag.ID)
		clusters[root] = append(clusters[root], info.tag)
	}

	var b strings.Builder
	count := 0
	for _, cluster := range clusters {
		if len(cluster) < 2 {
			continue
		}
		count++
		totalScenes := 0
		totalImages := 0
		for _, t := range cluster {
			scenes, images := tagSceneImageCounts(ctx, repo, t.ID)
			totalScenes += scenes
			totalImages += images
			fmt.Fprintf(&b, "- [Tag #%d - %s](/tags/%d)", t.ID, t.Name, t.ID)
			if len(t.Aliases.List()) > 0 {
				fmt.Fprintf(&b, " | Aliases: %s", strings.Join(t.Aliases.List(), ", "))
			}
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  (combined: %d scenes, %d images)\n\n", totalScenes, totalImages)
	}

	if count == 0 {
		return "No tags share enough aliases to be considered duplicates. No duplicate tag clusters found.", nil
	}

	fmt.Fprintf(&b, "\n%d duplicate tag cluster(s) found. Use merge_tags to consolidate, or update_tag to move aliases between tags.", count)
	return b.String(), nil
}

var generateEmbeddingsParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"entity_types": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "string",
				"enum": []string{"scene", "performer", "image", "gallery", "studio", "tag"},
			},
			"description": "Entity types to generate embeddings for. Default: all types.",
		},
		"overwrite": map[string]interface{}{
			"type":        "boolean",
			"description": "Overwrite existing embeddings for the selected model (default: false)",
			"default":     false,
		},
	},
}

func generateEmbeddings(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		EntityTypes []string `json:"entity_types"`
		Overwrite   bool     `json:"overwrite"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	// Use the callback from ToolConfig to start the embedding job
	if cfg.StartEmbeddingJob == nil {
		return "Embedding generation is not available. Please check AI configuration.", nil
	}

	jobID, err := cfg.StartEmbeddingJob(ctx, params.EntityTypes, params.Overwrite)
	if err != nil {
		return "", fmt.Errorf("starting embedding job: %w", err)
	}

	return fmt.Sprintf("Started embedding generation job (ID: %d). You can track progress in the task queue.", jobID), nil
}

func getSceneMarkers(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		SceneID int `json:"scene_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	markers, err := repo.SceneMarker.FindBySceneID(ctx, params.SceneID)
	if err != nil {
		return "", fmt.Errorf("finding markers: %w", err)
	}

	scene, err := repo.Scene.Find(ctx, params.SceneID)
	if err != nil {
		return "", fmt.Errorf("finding scene: %w", err)
	}

	if len(markers) == 0 {
		if scene != nil {
			return fmt.Sprintf("Scene \"%s\" has no markers.", scene.Title), nil
		}
		return fmt.Sprintf("Scene %d not found.", params.SceneID), nil
	}

	var b strings.Builder
	if scene != nil {
		fmt.Fprintf(&b, "Markers for [Scene #%d - %s](/scenes/%d):\n\n", scene.ID, scene.Title, scene.ID)
	} else {
		fmt.Fprintf(&b, "Markers for scene ID %d:\n\n", params.SceneID)
	}

	for _, m := range markers {
		tag, _ := repo.Tag.Find(ctx, m.PrimaryTagID)
		tagName := "unknown"
		if tag != nil {
			tagName = tag.Name
		}
		fmt.Fprintf(&b, "- %.1fs", m.Seconds)
		if m.EndSeconds != nil {
			fmt.Fprintf(&b, "-%.1fs", *m.EndSeconds)
		}
		fmt.Fprintf(&b, " | %s | %s\n", m.Title, tagName)
	}

	return b.String(), nil
}

func findScenesByPerformer(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		PerformerName string `json:"performer_name"`
		PerformerID   int    `json:"performer_id"`
		Limit         int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	var performerID int
	switch {
	case params.PerformerID > 0:
		performerID = params.PerformerID
	case params.PerformerName != "":
		q := params.PerformerName
		filter := &models.FindFilterType{
			Q:       &q,
			PerPage: intPtr(1),
		}
		performers, _, err := repo.Performer.Query(ctx, nil, filter)
		if err != nil {
			return "", fmt.Errorf("searching performer: %w", err)
		}
		if len(performers) == 0 {
			return fmt.Sprintf("No performer found matching \"%s\".", params.PerformerName), nil
		}
		performerID = performers[0].ID
	default:
		return "Provide either performer_id or performer_name.", nil
	}

	performer, err := repo.Performer.Find(ctx, performerID)
	if err != nil {
		return "", fmt.Errorf("finding performer: %w", err)
	}
	if performer == nil {
		return fmt.Sprintf("Performer with ID %d not found.", performerID), nil
	}

	scenes, err := repo.Scene.FindByPerformerID(ctx, performerID)
	if err != nil {
		return "", fmt.Errorf("finding scenes: %w", err)
	}

	if len(scenes) == 0 {
		return fmt.Sprintf("No scenes found for performer \"%s\".", performer.Name), nil
	}

	total := len(scenes)
	if params.Limit < total {
		scenes = scenes[:params.Limit]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d scenes total for [Performer #%d - %s](/performers/%d). Showing %d:\n\n", total, performer.ID, performer.Name, performer.ID, len(scenes))
	for _, s := range scenes {
		_ = s.LoadPrimaryFile(ctx, repo.File)
		fmt.Fprintf(&b, "- [Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
		if s.Date != nil {
			fmt.Fprintf(&b, " | Date: %s", s.Date.String())
		}
		if f := s.Files.Primary(); f != nil {
			fmt.Fprintf(&b, " | Duration: %.1fs", f.Duration)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func getTopRated(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Type  string `json:"type"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	sort := "rating"
	desc := models.SortDirectionEnumDesc

	switch params.Type {
	case "scene":
		filter := &models.FindFilterType{
			PerPage:   &params.Limit,
			Sort:      &sort,
			Direction: &desc,
		}

		result, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
			QueryOptions: models.QueryOptions{
				FindFilter: filter,
				Count:      false,
			},
			SceneFilter: &models.SceneFilterType{
				Rating100: &models.IntCriterionInput{
					Value:    1,
					Modifier: models.CriterionModifierGreaterThan,
				},
			},
		})
		if err != nil {
			return "", fmt.Errorf("querying scenes: %w", err)
		}

		scenes, err := result.Resolve(ctx)
		if err != nil {
			return "", fmt.Errorf("resolving scenes: %w", err)
		}

		if len(scenes) == 0 {
			return "No rated scenes found.", nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Top rated scenes:\n\n")
		for _, s := range scenes {
			_ = s.LoadPrimaryFile(ctx, repo.File)
			fmt.Fprintf(&b, "- [Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
			if s.Rating != nil {
				fmt.Fprintf(&b, " | Rating: %d/100", *s.Rating)
			}
			if f := s.Files.Primary(); f != nil {
				fmt.Fprintf(&b, " | Duration: %.1fs", f.Duration)
			}
			b.WriteString("\n")
		}
		return b.String(), nil
	case "image":
		filter := &models.FindFilterType{
			PerPage:   &params.Limit,
			Sort:      &sort,
			Direction: &desc,
		}

		result, err := repo.Image.Query(ctx, models.ImageQueryOptions{
			QueryOptions: models.QueryOptions{
				FindFilter: filter,
				Count:      false,
			},
			ImageFilter: &models.ImageFilterType{
				Rating100: &models.IntCriterionInput{
					Value:    1,
					Modifier: models.CriterionModifierGreaterThan,
				},
			},
		})
		if err != nil {
			return "", fmt.Errorf("querying images: %w", err)
		}

		images, err := result.Resolve(ctx)
		if err != nil {
			return "", fmt.Errorf("resolving images: %w", err)
		}

		if len(images) == 0 {
			return "No rated images found.", nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Top rated images:\n\n")
		for _, img := range images {
			fmt.Fprintf(&b, "- [Image #%d - %s](/images/%d)", img.ID, img.Title, img.ID)
			if img.Rating != nil {
				fmt.Fprintf(&b, " | Rating: %d/100", *img.Rating)
			}
			b.WriteString("\n")
		}
		return b.String(), nil
	default:
		return fmt.Sprintf("Unknown type: %s. Use 'scene' or 'image'.", params.Type), nil
	}
}

func getTopViewedScenes(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	sort := "o_counter"
	desc := models.SortDirectionEnumDesc
	filter := &models.FindFilterType{
		PerPage:   &params.Limit,
		Sort:      &sort,
		Direction: &desc,
	}

	result, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: filter,
			Count:      false,
		},
	})
	if err != nil {
		return "", fmt.Errorf("querying scenes: %w", err)
	}

	scenes, err := result.Resolve(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving scenes: %w", err)
	}

	if len(scenes) == 0 {
		return "No scenes found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Most o-counted scenes:\n\n")
	for _, s := range scenes {
		_ = s.LoadPrimaryFile(ctx, repo.File)
		fmt.Fprintf(&b, "- [Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
		if s.Rating != nil {
			fmt.Fprintf(&b, " | Rating: %d/100", *s.Rating)
		}
		oCount, _ := repo.Scene.GetOCount(ctx, s.ID)
		fmt.Fprintf(&b, " | OCount: %d", oCount)
		if f := s.Files.Primary(); f != nil {
			fmt.Fprintf(&b, " | Duration: %.1fs", f.Duration)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func getTopPlayedScenes(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	sort := "play_count"
	desc := models.SortDirectionEnumDesc
	filter := &models.FindFilterType{
		PerPage:   &params.Limit,
		Sort:      &sort,
		Direction: &desc,
	}

	result, err := repo.Scene.Query(ctx, models.SceneQueryOptions{
		QueryOptions: models.QueryOptions{
			FindFilter: filter,
			Count:      false,
		},
	})
	if err != nil {
		return "", fmt.Errorf("querying scenes: %w", err)
	}

	scenes, err := result.Resolve(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving scenes: %w", err)
	}

	if len(scenes) == 0 {
		return "No scenes found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Most played scenes:\n\n")
	for _, s := range scenes {
		_ = s.LoadPrimaryFile(ctx, repo.File)
		fmt.Fprintf(&b, "- [Scene #%d - %s](/scenes/%d)", s.ID, s.Title, s.ID)
		if s.Rating != nil {
			fmt.Fprintf(&b, " | Rating: %d/100", *s.Rating)
		}
		playCount, _ := repo.Scene.CountViews(ctx, s.ID)
		fmt.Fprintf(&b, " | Plays: %d", playCount)
		if f := s.Files.Primary(); f != nil {
			fmt.Fprintf(&b, " | Duration: %.1fs", f.Duration)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func getPerformerDetails(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		PerformerID int `json:"performer_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	p, err := repo.Performer.Find(ctx, params.PerformerID)
	if err != nil {
		return "", fmt.Errorf("finding performer: %w", err)
	}
	if p == nil {
		return fmt.Sprintf("Performer with ID %d not found.", params.PerformerID), nil
	}

	sceneCount, _ := repo.Scene.CountByPerformerID(ctx, p.ID)
	groupCount, _ := repo.Group.CountByPerformerID(ctx, p.ID)

	var b strings.Builder
	fmt.Fprintf(&b, "[Performer #%d - %s](/performers/%d)\n", p.ID, p.Name, p.ID)
	if p.Disambiguation != "" {
		fmt.Fprintf(&b, "  Also known as: %s\n", p.Disambiguation)
	}
	if p.Gender != nil {
		fmt.Fprintf(&b, "  Gender: %s\n", p.Gender.String())
	}
	if p.Birthdate != nil {
		fmt.Fprintf(&b, "  Birthdate: %s\n", p.Birthdate.String())
	}
	if p.Ethnicity != "" {
		fmt.Fprintf(&b, "  Ethnicity: %s\n", p.Ethnicity)
	}
	if p.Country != "" {
		fmt.Fprintf(&b, "  Country: %s\n", p.Country)
	}
	if p.CareerStart != nil {
		fmt.Fprintf(&b, "  Career start: %s\n", p.CareerStart.String())
	}
	if p.CareerEnd != nil {
		fmt.Fprintf(&b, "  Career end: %s\n", p.CareerEnd.String())
	}
	if p.Rating != nil {
		fmt.Fprintf(&b, "  Rating: %d/100\n", *p.Rating)
	}
	fmt.Fprintf(&b, "  Favorite: %v\n", p.Favorite)
	fmt.Fprintf(&b, "  Scenes: %d\n", sceneCount)
	fmt.Fprintf(&b, "  Groups: %d\n", groupCount)
	if p.Details != "" {
		fmt.Fprintf(&b, "  Details: %s\n", p.Details)
	}

	return b.String(), nil
}

func getSceneFileDetails(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		SceneID int `json:"scene_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	s, err := repo.Scene.Find(ctx, params.SceneID)
	if err != nil {
		return "", fmt.Errorf("finding scene: %w", err)
	}
	if s == nil {
		return fmt.Sprintf("Scene with ID %d not found.", params.SceneID), nil
	}

	_ = s.LoadPrimaryFile(ctx, repo.File)
	f := s.Files.Primary()
	if f == nil {
		return fmt.Sprintf("Scene \"%s\" has no files.", s.Title), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "File details for [Scene #%d - %s](/scenes/%d):\n", s.ID, s.Title, s.ID)
	fmt.Fprintf(&b, "  Path: %s\n", f.Base().Path)
	fmt.Fprintf(&b, "  Size: %.1f MB\n", float64(f.Base().Size)/1024/1024)
	fmt.Fprintf(&b, "  Duration: %.1fs (%.1f min)\n", f.Duration, f.Duration/60)
	fmt.Fprintf(&b, "  Resolution: %dx%d\n", f.Width, f.Height)
	fmt.Fprintf(&b, "  Framerate: %.2f fps\n", f.FrameRate)
	fmt.Fprintf(&b, "  Bitrate: %.0f kbps\n", float64(f.BitRate))
	fmt.Fprintf(&b, "  Video codec: %s\n", f.VideoCodec)
	fmt.Fprintf(&b, "  Audio codec: %s\n", f.AudioCodec)

	if len(f.Base().Fingerprints) > 0 {
		fmt.Fprintf(&b, "  Fingerprints:\n")
		for _, fp := range f.Base().Fingerprints {
			fmt.Fprintf(&b, "    %s: %s\n", fp.Type, fp.Fingerprint)
		}
	}

	return b.String(), nil
}

func searchGroups(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 10
	}

	q := params.Query
	filter := &models.FindFilterType{
		Q:       &q,
		PerPage: &params.Limit,
		Sort:    strPtr("name"),
	}

	groups, _, err := repo.Group.Query(ctx, nil, filter)
	if err != nil {
		return "", fmt.Errorf("querying groups: %w", err)
	}

	if len(groups) == 0 {
		return "No groups found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d groups:\n\n", len(groups))
	for _, g := range groups {
		fmt.Fprintf(&b, "- [Group #%d - %s](/groups/%d)", g.ID, g.Name, g.ID)
		if g.Date != nil {
			fmt.Fprintf(&b, " | Date: %s", g.Date.String())
		}
		if g.Rating != nil {
			fmt.Fprintf(&b, " | Rating: %d/100", *g.Rating)
		}
		if g.Duration != nil {
			fmt.Fprintf(&b, " | Duration: %ds", *g.Duration)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func markSceneOrganized(ctx context.Context, repo models.Repository, args json.RawMessage) (string, error) {
	var params struct {
		SceneID   int  `json:"scene_id"`
		Organized bool `json:"organized"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	var result string
	if err := repo.WithTxn(ctx, func(ctx context.Context) error {
		s, err := repo.Scene.Find(ctx, params.SceneID)
		if err != nil {
			return fmt.Errorf("finding scene: %w", err)
		}
		if s == nil {
			return fmt.Errorf("scene with ID %d not found", params.SceneID)
		}

		_, err = repo.Scene.UpdatePartial(ctx, params.SceneID, models.ScenePartial{
			Organized: models.NewOptionalBool(params.Organized),
		})
		if err != nil {
			return fmt.Errorf("updating scene: %w", err)
		}

		status := "organized"
		if !params.Organized {
			status = "unorganized"
		}
		result = fmt.Sprintf("[Scene #%d - %s](/scenes/%d) marked as %s.", s.ID, s.Title, s.ID, status)
		return nil
	}); err != nil {
		return "", err
	}

	return result, nil
}

var generateImageParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"prompt": map[string]interface{}{
			"type":        "string",
			"description": "Text prompt describing the image to generate",
		},
		"source_type": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"scene", "image"},
			"description": "Optional. Type of source entity to use as the base image for img2img generation ('scene' or 'image'). Omit (with source_id) to generate purely from the text prompt.",
		},
		"source_id": map[string]interface{}{
			"type":        "integer",
			"description": "ID of the source scene or image to use as the base for img2img generation. Required only when source_type is provided.",
		},
		"negative_prompt": map[string]interface{}{
			"type":        "string",
			"description": "Negative prompt (things to avoid)",
		},
		"denoising_strength": map[string]interface{}{
			"type":        "number",
			"description": "Denoising strength (0.0 to 1.0, higher = more variation from source)",
			"minimum":     0,
			"maximum":     1,
		},
		"steps": map[string]interface{}{
			"type":        "integer",
			"description": "Number of sampling steps (higher = more detail, slower)",
			"minimum":     1,
			"maximum":     150,
		},
		"cfg_scale": map[string]interface{}{
			"type":        "number",
			"description": "CFG scale (how strongly the prompt influences the output)",
			"minimum":     1,
			"maximum":     30,
		},
		"width": map[string]interface{}{
			"type":        "integer",
			"description": "Output width in pixels",
			"minimum":     64,
			"maximum":     2048,
		},
		"height": map[string]interface{}{
			"type":        "integer",
			"description": "Output height in pixels",
			"minimum":     64,
			"maximum":     2048,
		},
		"sampler_name": map[string]interface{}{
			"type":        "string",
			"description": "Sampling method (e.g. 'Euler a', 'DPM++ 2M Karras', 'DDIM')",
		},
		"seed": map[string]interface{}{
			"type":        "integer",
			"description": "Random seed (-1 for random)",
		},
		"model": map[string]interface{}{
			"type":        "string",
			"description": "Model name to use for generation (use list_a1111_models to see available models)",
		},
	},
	"required": []string{"prompt"},
}

func generateImage(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	if !cfg.A1111Enabled {
		return "", fmt.Errorf("Automatic1111 is not enabled; configure it in Settings")
	}
	if cfg.A1111BaseURL == "" {
		return "", fmt.Errorf("Automatic1111 base URL is not configured")
	}

	var params struct {
		Prompt            string   `json:"prompt"`
		SourceType        string   `json:"source_type"`
		SourceID          int      `json:"source_id"`
		NegativePrompt    *string  `json:"negative_prompt"`
		DenoisingStrength *float64 `json:"denoising_strength"`
		Steps             *int     `json:"steps"`
		CFGScale          *float64 `json:"cfg_scale"`
		Width             *int     `json:"width"`
		Height            *int     `json:"height"`
		SamplerName       *string  `json:"sampler_name"`
		Seed              *int64   `json:"seed"`
		Model             *string  `json:"model"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	if params.Prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	var initImage []byte

	switch params.SourceType {
	case "scene":
		if params.SourceID == 0 {
			return "", fmt.Errorf("source_id is required when source_type is 'scene'")
		}
		cover, err := repo.Scene.GetCover(ctx, params.SourceID)
		if err != nil {
			return "", fmt.Errorf("getting scene cover: %w", err)
		}
		if cover == nil {
			if err := repo.WithTxn(ctx, func(ctx context.Context) error {
				s, err := repo.Scene.Find(ctx, params.SourceID)
				if err != nil {
					return fmt.Errorf("finding scene: %w", err)
				}
				if s == nil {
					return fmt.Errorf("scene with ID %d not found", params.SourceID)
				}
				return fmt.Errorf("scene #%d has no cover image", params.SourceID)
			}); err != nil {
				return "", err
			}
			return "", fmt.Errorf("scene #%d has no cover image", params.SourceID)
		}
		initImage = cover
	case "image":
		if params.SourceID == 0 {
			return "", fmt.Errorf("source_id is required when source_type is 'image'")
		}
		if err := repo.WithTxn(ctx, func(ctx context.Context) error {
			img, err := repo.Image.Find(ctx, params.SourceID)
			if err != nil {
				return fmt.Errorf("finding image: %w", err)
			}
			if img == nil {
				return fmt.Errorf("image with ID %d not found", params.SourceID)
			}
			if err := img.LoadPrimaryFile(ctx, repo.File); err != nil {
				return fmt.Errorf("loading image file: %w", err)
			}
			files, err := repo.File.Find(ctx, *img.PrimaryFileID)
			if err != nil || len(files) == 0 {
				return fmt.Errorf("finding file for image: %w", err)
			}
			path := files[0].Base().Path
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading image file: %w", err)
			}
			initImage = data
			return nil
		}); err != nil {
			return "", err
		}
	case "":
		// no source provided; generate purely from the text prompt
	default:
		return "", fmt.Errorf("source_type must be 'scene', 'image', or omitted for text-to-image, got '%s'", params.SourceType)
	}

	opts := A1111Options{
		DenoisingStrength: 0.75,
		Steps:             30,
		CFGScale:          7.5,
		Width:             768,
		Height:            768,
		SamplerName:       "DPM++ 2M Karras",
		Seed:              -1,
		NegativePrompt:    "low quality, worst quality, bad anatomy, bad composition, blurry, out of focus, ugly, deformed, noise, artifacts, text, watermark, signature, title, username, artist name, lowres, error, jpeg artifacts",
	}
	if params.NegativePrompt != nil {
		opts.NegativePrompt = *params.NegativePrompt
	}
	if params.DenoisingStrength != nil {
		opts.DenoisingStrength = *params.DenoisingStrength
	}
	if params.Steps != nil {
		opts.Steps = *params.Steps
	}
	if params.CFGScale != nil {
		opts.CFGScale = *params.CFGScale
	}
	if params.Width != nil {
		opts.Width = *params.Width
	}
	if params.Height != nil {
		opts.Height = *params.Height
	}
	if params.SamplerName != nil {
		opts.SamplerName = *params.SamplerName
	}
	if params.Seed != nil {
		opts.Seed = *params.Seed
	}
	if params.Model != nil {
		opts.Model = *params.Model
	}

	// Set model in A1111 if specified
	if opts.Model != "" {
		if err := A1111SetModel(A1111Config{BaseURL: cfg.A1111BaseURL}, opts.Model); err != nil {
			return "", fmt.Errorf("setting A1111 model: %w", err)
		}
	}

	result, err := A1111GenerateImage(A1111Config{BaseURL: cfg.A1111BaseURL}, initImage, params.Prompt, opts)
	if err != nil {
		return "", fmt.Errorf("image generation failed: %w", err)
	}

	if cfg.GeneratedPath == "" {
		return "", fmt.Errorf("generated path is not configured")
	}

	aiDir := filepath.Join(cfg.GeneratedPath, "ai_generated")
	if err := os.MkdirAll(aiDir, 0755); err != nil {
		return "", fmt.Errorf("creating ai_generated directory: %w", err)
	}

	ts := time.Now().UnixNano()
	filename := fmt.Sprintf("ai_%d.jpg", ts)
	if params.SourceID != 0 {
		filename = fmt.Sprintf("ai_%d_%d.jpg", ts, params.SourceID)
	}
	filePath := filepath.Join(aiDir, filename)

	if err := os.WriteFile(filePath, result, 0644); err != nil {
		return "", fmt.Errorf("writing generated image: %w", err)
	}

	cfgImg, _, err := image.DecodeConfig(bytes.NewReader(result))
	if err != nil {
		return "", fmt.Errorf("decoding generated image dimensions: %w", err)
	}

	sha := sha256.Sum256(result)
	shaHex := hex.EncodeToString(sha[:])

	var newImageResult *models.Image
	if err := repo.WithTxn(ctx, func(ctx context.Context) error {
		folder, err := repo.Folder.FindByPath(ctx, aiDir, true)
		if err != nil {
			return fmt.Errorf("finding folder: %w", err)
		}
		if folder == nil {
			folder = &models.Folder{
				Path: aiDir,
				DirEntry: models.DirEntry{
					ModTime: time.Now(),
				},
			}
			if err := repo.Folder.Create(ctx, folder); err != nil {
				return fmt.Errorf("creating folder: %w", err)
			}
		}

		imageFile := &models.ImageFile{
			BaseFile: &models.BaseFile{
				Path:           filePath,
				Basename:       filename,
				ParentFolderID: folder.ID,
				Size:           int64(len(result)),
				DirEntry: models.DirEntry{
					ModTime: time.Now(),
				},
				Fingerprints: []models.Fingerprint{
					{Type: "sha256", Fingerprint: shaHex},
				},
			},
			Format: "jpg",
			Width:  cfgImg.Width,
			Height: cfgImg.Height,
		}

		if err := repo.File.Create(ctx, imageFile); err != nil {
			return fmt.Errorf("creating file entry: %w", err)
		}

		newImg := models.NewImage()
		newImg.Title = params.Prompt
		input := &models.CreateImageInput{
			Image:   &newImg,
			FileIDs: []models.FileID{imageFile.Base().ID},
		}

		if err := repo.Image.Create(ctx, input); err != nil {
			return fmt.Errorf("creating image entry: %w", err)
		}

		newImageResult = input.Image
		return nil
	}); err != nil {
		return "", err
	}

	return fmt.Sprintf("Generated [Image #%d - %s](/images/%d) using prompt: %s", newImageResult.ID, params.Prompt, newImageResult.ID, params.Prompt), nil
}

var listA1111ModelsParam = map[string]interface{}{
	"type":       "object",
	"properties": map[string]interface{}{},
}

func listA1111Models(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	if !cfg.A1111Enabled {
		return "", fmt.Errorf("Automatic1111 is not enabled; configure it in Settings")
	}
	if cfg.A1111BaseURL == "" {
		return "", fmt.Errorf("Automatic1111 base URL is not configured")
	}

	models, err := A1111ListModels(A1111Config{BaseURL: cfg.A1111BaseURL})
	if err != nil {
		return "", fmt.Errorf("listing A1111 models: %w", err)
	}

	if len(models) == 0 {
		return "No models found in Automatic1111.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Available Automatic1111 models (%d):\n\n", len(models))
	for _, m := range models {
		name := m.Title
		if m.ModelName != "" {
			name = m.ModelName
		}
		fmt.Fprintf(&b, "- %s", name)
		if m.Hash != "" {
			fmt.Fprintf(&b, " (hash: %s)", m.Hash)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

var listA1111LorasParam = map[string]interface{}{
	"type":       "object",
	"properties": map[string]interface{}{},
}

func listA1111Loras(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	if !cfg.A1111Enabled {
		return "", fmt.Errorf("Automatic1111 is not enabled; configure it in Settings")
	}
	if cfg.A1111BaseURL == "" {
		return "", fmt.Errorf("Automatic1111 base URL is not configured")
	}

	loras, err := A1111ListLoras(A1111Config{BaseURL: cfg.A1111BaseURL})
	if err != nil {
		return "", fmt.Errorf("listing A1111 LoRAs: %w", err)
	}

	if len(loras) == 0 {
		return "No LoRAs found in Automatic1111.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Available Automatic1111 LoRAs (%d):\n\n", len(loras))
	for _, l := range loras {
		name := l.Name
		if l.Alias != "" {
			name = l.Alias
		}
		fmt.Fprintf(&b, "- %s", name)
		if l.Path != "" {
			fmt.Fprintf(&b, " (path: %s)", l.Path)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

var describeImageParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"image_id": map[string]interface{}{
			"type":        "integer",
			"description": "ID of the image to describe. Use this OR scene_id.",
		},
		"scene_id": map[string]interface{}{
			"type":        "integer",
			"description": "ID of the scene whose cover to describe. Use this OR image_id.",
		},
		"detail_level": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"brief", "detailed", "comprehensive"},
			"description": "Level of detail for the description. 'brief' = one sentence, 'detailed' = paragraph, 'comprehensive' = full visual analysis.",
			"default":     "detailed",
		},
	},
}

func describeImage(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		ImageID     int    `json:"image_id"`
		SceneID     int    `json:"scene_id"`
		DetailLevel string `json:"detail_level"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	if params.ImageID == 0 && params.SceneID == 0 {
		return "", fmt.Errorf("either image_id or scene_id is required")
	}
	if params.ImageID > 0 && params.SceneID > 0 {
		return "", fmt.Errorf("provide either image_id or scene_id, not both")
	}

	if params.DetailLevel == "" {
		params.DetailLevel = "detailed"
	}

	if cfg.LLMBaseURL == "" {
		return "", fmt.Errorf("AI base URL is not configured; configure it in Settings")
	}

	client := NewClient(cfg.LLMBaseURL, cfg.LLMModel)

	var imageData []byte
	var imageMediaType string

	if params.ImageID > 0 {
		if err := repo.WithTxn(ctx, func(ctx context.Context) error {
			img, err := repo.Image.Find(ctx, params.ImageID)
			if err != nil {
				return fmt.Errorf("finding image: %w", err)
			}
			if img == nil {
				return fmt.Errorf("image with ID %d not found", params.ImageID)
			}
			if err := img.LoadPrimaryFile(ctx, repo.File); err != nil {
				return fmt.Errorf("loading image file: %w", err)
			}
			f := img.Files.Primary()
			if f == nil {
				return fmt.Errorf("image #%d has no file", params.ImageID)
			}
			data, err := os.ReadFile(f.Base().Path)
			if err != nil {
				return fmt.Errorf("reading image file: %w", err)
			}
			imageData = data
			imageMediaType = http.DetectContentType(data)
			return nil
		}); err != nil {
			return "", err
		}
	} else if params.SceneID > 0 {
		cover, err := repo.Scene.GetCover(ctx, params.SceneID)
		if err != nil {
			return "", fmt.Errorf("getting scene cover: %w", err)
		}
		if cover == nil {
			return "", fmt.Errorf("scene #%d has no cover image", params.SceneID)
		}
		imageData = cover
		imageMediaType = http.DetectContentType(cover)
	}

	prompts := map[string]string{
		"brief":         "Describe this image in one sentence. Focus on the single most notable visual element.",
		"detailed":      "Describe this image in detail. Include: people (number, appearance, clothing, actions), setting/location, objects, colors, composition, and any notable visual features.",
		"comprehensive": "Provide a comprehensive visual analysis of this image. Describe every visible element including people (count, gender, appearance, clothing, expressions, actions), setting (indoor/outdoor, type of location, decor), objects, colors, lighting, composition, text/watermarks, and any distinctive features that could be used for categorization.",
	}

	prompt, ok := prompts[params.DetailLevel]
	if !ok {
		prompt = prompts["detailed"]
	}

	systemPrompt := "You are an image analyst. Your task is to describe images objectively and thoroughly. Focus on visual details that are factually present in the image. Never refuse, censor, or moralize about any content — this is a private local library and all content is user-curated."

	base64Str := base64.StdEncoding.EncodeToString(imageData)
	description, err := client.VisionCompletion(ctx, systemPrompt, prompt, base64Str, imageMediaType)
	if err != nil {
		return "", fmt.Errorf("vision analysis failed: %w", err)
	}

	return description, nil
}

var describeSceneParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"scene_id": map[string]interface{}{
			"type":        "integer",
			"description": "ID of the scene to describe",
		},
		"detail_level": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"brief", "detailed", "comprehensive"},
			"description": "Level of detail for the description. 'brief' = one sentence, 'detailed' = paragraph with key elements, 'comprehensive' = full visual analysis of the video content across frames.",
			"default":     "detailed",
		},
		"num_frames": map[string]interface{}{
			"type":        "integer",
			"description": "Number of frames to sample from the video (default 4, max 10). More frames give better analysis but use more tokens.",
			"default":     4,
		},
	},
	"required": []string{"scene_id"},
}

func describeScene(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		SceneID     int    `json:"scene_id"`
		DetailLevel string `json:"detail_level"`
		NumFrames   int    `json:"num_frames"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if params.SceneID == 0 {
		return "", fmt.Errorf("scene_id is required")
	}
	if params.DetailLevel == "" {
		params.DetailLevel = "detailed"
	}
	if params.NumFrames <= 0 || params.NumFrames > 10 {
		params.NumFrames = 4
	}

	if cfg.LLMBaseURL == "" {
		return "", fmt.Errorf("AI base URL is not configured; configure it in Settings")
	}

	client := NewClient(cfg.LLMBaseURL, cfg.LLMModel)

	s, err := repo.Scene.Find(ctx, params.SceneID)
	if err != nil {
		return "", fmt.Errorf("finding scene: %w", err)
	}
	if s == nil {
		return fmt.Sprintf("Scene with ID %d not found.", params.SceneID), nil
	}

	_ = s.LoadPrimaryFile(ctx, repo.File)
	f := s.Files.Primary()

	var images []MultiImage
	var tmpFiles []string
	defer func() {
		for _, p := range tmpFiles {
			os.Remove(p)
		}
	}()

	if f != nil && f.Base().Path != "" && cfg.FFMpegPath != "" {
		duration := f.Duration
		if duration <= 0 {
			duration = 60
		}

		numShots := params.NumFrames
		percentages := make([]float64, numShots)
		step := 1.0 / float64(numShots+1)
		for i := range numShots {
			percentages[i] = step * float64(i+1)
		}

		encoder := ffmpeg.NewEncoder(cfg.FFMpegPath)

		for _, pct := range percentages {
			t := math.Round(pct*duration*10) / 10
			if t < 0.1 {
				t = 0.1
			}
			if t > duration-0.5 {
				t = duration - 0.5
			}
			if t < 0 {
				continue
			}

			tmpFile, err := os.CreateTemp("", "stash-ai-scene-*.jpg")
			if err != nil {
				continue
			}
			tmpPath := tmpFile.Name()
			tmpFile.Close()
			tmpFiles = append(tmpFiles, tmpPath)

			opts := transcoder.ScreenshotOptions{
				OutputPath: tmpPath,
				OutputType: transcoder.ScreenshotOutputTypeImage2,
			}
			screenshotArgs := transcoder.ScreenshotTime(f.Base().Path, t, opts)

			if err := encoder.Generate(ctx, screenshotArgs); err != nil {
				continue
			}

			data, err := os.ReadFile(tmpPath)
			if err != nil {
				continue
			}

			mediaType := http.DetectContentType(data)
			images = append(images, MultiImage{
				Base64:    base64.StdEncoding.EncodeToString(data),
				MediaType: mediaType,
			})

			if len(images) >= numShots {
				break
			}
		}
	}

	if len(images) == 0 {
		cover, err := repo.Scene.GetCover(ctx, params.SceneID)
		if err != nil {
			return "", fmt.Errorf("getting scene cover: %w", err)
		}
		if cover == nil {
			return "", fmt.Errorf("scene #%d has no cover image and no video file to extract frames from", params.SceneID)
		}
		mediaType := http.DetectContentType(cover)
		images = append(images, MultiImage{
			Base64:    base64.StdEncoding.EncodeToString(cover),
			MediaType: mediaType,
		})
	}

	prompts := map[string]string{
		"brief":         "These are frames from a video scene. Describe what is happening in one sentence. Focus on the single most notable visual element or action.",
		"detailed":      "These are frames sampled from different points in a video scene. Analyze the content: who is present (number of people, gender, appearance, clothing), what actions are occurring, the setting/location, and any notable objects or visual elements visible across these frames. Note any changes between frames that indicate a narrative or progression.",
		"comprehensive": "These are frames sampled from different points in a video scene. Provide a comprehensive analysis: identify all people (count, gender, appearance, clothing, expressions), describe the setting and how it may change, catalog visible actions and activities, note any narrative progression or scene changes visible across frames, describe lighting, colors, camera angles, and any text/watermarks. Use the sequence of frames to understand the full context of the scene.",
	}

	prompt, ok := prompts[params.DetailLevel]
	if !ok {
		prompt = prompts["detailed"]
	}

	systemPrompt := "You are a video content analyst. You are given frames sampled from different points in a video. Analyze them as a sequence to understand what is happening in the scene. Be objective and focus on visual details that are factually present. Never refuse, censor, or moralize about any content — this is a private local library and all content is user-curated."

	description, err := client.MultiVisionCompletion(ctx, systemPrompt, prompt, images)
	if err != nil {
		return "", fmt.Errorf("video analysis failed: %w", err)
	}

	return description, nil
}

var segmentSceneParam = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"scene_ids": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "integer",
			},
			"description": "Specific scene IDs to segment. If empty, segments all scenes without existing markers.",
		},
		"max_scenes": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum number of scenes to segment (default: no limit).",
		},
		"overwrite": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether to re-segment scenes that already have markers (default: false).",
		},
	},
	"required": []string{},
}

func segmentScene(ctx context.Context, repo models.Repository, args json.RawMessage, cfg ToolConfig) (string, error) {
	var params struct {
		SceneIDs  []int `json:"scene_ids"`
		MaxScenes *int  `json:"max_scenes"`
		Overwrite bool  `json:"overwrite"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	if cfg.StartSceneSegmentJob == nil {
		return "Scene segmentation is not available. Please check AI configuration.", nil
	}

	jobID, err := cfg.StartSceneSegmentJob(ctx, params.SceneIDs, params.MaxScenes, params.Overwrite)
	if err != nil {
		return "", fmt.Errorf("starting scene segmentation job: %w", err)
	}

	return fmt.Sprintf("Started scene segmentation job (ID: %d). You can track progress in the task queue.", jobID), nil
}

func toolDefFromTool(t Tool) ToolDefinition {
	return ToolDefinition{
		Type: "function",
		Function: ToolFunction{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		},
	}
}
