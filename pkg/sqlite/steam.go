package sqlite

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/models"
)

// GetSteamScores returns a 0-10 steam score per scene id, combining the moan
// flag, the silence ratio, and explicit (any) tag coverage from the audio
// analysis and tagging data. Scenes without audio analysis score 0.
func (s *SceneStore) GetSteamScores(ctx context.Context, sceneIDs []int) (map[int]int, error) {
	if len(sceneIDs) == 0 {
		return map[int]int{}, nil
	}

	inBinding := getInBinding(len(sceneIDs))
	args := make([]interface{}, len(sceneIDs))
	for i, id := range sceneIDs {
		args[i] = id
	}

	query := `SELECT s.id, MIN(10,
		CASE WHEN a.moans = 1 THEN 4 ELSE 0 END +
		CASE WHEN a.silence_ratio < 30 THEN 3 ELSE 0 END +
		CASE WHEN EXISTS(SELECT 1 FROM scenes_tags st WHERE st.scene_id = s.id) THEN 3 ELSE 0 END
	) AS steam
	FROM scenes s
	LEFT JOIN ai_scene_audio a ON a.scene_id = s.id
	WHERE s.id IN ` + inBinding

	rows, err := dbWrapper.Queryx(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int]int, len(sceneIDs))
	for rows.Next() {
		var id, steam int
		if err := rows.Scan(&id, &steam); err != nil {
			return nil, err
		}
		out[id] = steam
	}
	return out, rows.Err()
}

var _ = models.Scene{}

// OHistoryLeaderboard returns performers ordered by the number of distinct
// scenes the user logged an O for, from the manual O history.
func (s *SceneStore) OHistoryLeaderboard(ctx context.Context, limit int) ([]*models.AOHistoryLeaderboardEntry, error) {
	rows, err := dbWrapper.Queryx(ctx, `
		SELECT ps.performer_id, COUNT(DISTINCT od.scene_id) AS o_scenes
		FROM scenes_o_dates od
		JOIN performers_scenes ps ON ps.scene_id = od.scene_id
		GROUP BY ps.performer_id
		ORDER BY o_scenes DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.AOHistoryLeaderboardEntry
	for rows.Next() {
		var e models.AOHistoryLeaderboardEntry
		if err := rows.Scan(&e.PerformerID, &e.OScenes); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// OHistoryTimeline returns the number of logged O dates per day for the last
// N days.
func (s *SceneStore) OHistoryTimeline(ctx context.Context, days int) ([]*models.AIOHistoryTimelineEntry, error) {
	rows, err := dbWrapper.Queryx(ctx, `
		SELECT date(o_date) AS day, COUNT(*) FROM scenes_o_dates
		WHERE o_date >= datetime('now', ?)
		GROUP BY date(o_date)
		ORDER BY day`, fmt.Sprintf("-%d days", days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.AIOHistoryTimelineEntry
	for rows.Next() {
		var e models.AIOHistoryTimelineEntry
		if err := rows.Scan(&e.Date, &e.Count); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// GetSceneHeights returns a 0-25 height score per scene id: steam plus a
// bonus for logged O's and AI-detected moods.
func (s *SceneStore) GetSceneHeights(ctx context.Context, sceneIDs []int) (map[int]int, error) {
	if len(sceneIDs) == 0 {
		return map[int]int{}, nil
	}

	inBinding := getInBinding(len(sceneIDs))
	args := make([]interface{}, len(sceneIDs))
	for i, id := range sceneIDs {
		args[i] = id
	}

	steamExpr := "(MIN(10, COALESCE((SELECT CASE WHEN a.moans = 1 THEN 4 ELSE 0 END + CASE WHEN a.silence_ratio < 30 THEN 3 ELSE 0 END FROM ai_scene_audio a WHERE a.scene_id = scenes.id), 0) + CASE WHEN EXISTS(SELECT 1 FROM scenes_tags st WHERE st.scene_id = scenes.id) THEN 3 ELSE 0 END))"
	query := `SELECT scenes.id, MIN(25, ` + steamExpr + ` +
		COALESCE((SELECT MIN(9, COUNT(*) * 3) FROM scenes_o_dates od WHERE od.scene_id = scenes.id), 0) +
		COALESCE((SELECT MIN(6, COUNT(*)) FROM ai_scene_moods m WHERE m.scene_id = scenes.id), 0))
	FROM scenes WHERE scenes.id IN ` + inBinding

	rows, err := dbWrapper.Queryx(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int]int, len(sceneIDs))
	for rows.Next() {
		var id, height int
		if err := rows.Scan(&id, &height); err != nil {
			return nil, err
		}
		out[id] = height
	}
	return out, rows.Err()
}
