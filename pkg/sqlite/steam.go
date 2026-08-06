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

// FlightLog returns one entry per day (for the last N days) with O counts,
// distinct scenes finished, the summed height of those scenes, and the day's
// top performer.
func (s *SceneStore) FlightLog(ctx context.Context, days int) ([]*models.AIFlightLogEntry, error) {
	from := fmt.Sprintf("-%d days", days)

	type dayAgg struct {
		oCount      int
		sceneCount  int
		sceneIDs    []int
		altitude    int
		performer   string
		performerID *int
	}

	agg := map[string]*dayAgg{}
	var dayOrder []string

	rows, err := dbWrapper.Queryx(ctx, `
		SELECT date(o_date) AS day, COUNT(*) AS o_count, COUNT(DISTINCT scene_id) AS scene_count
		FROM scenes_o_dates
		WHERE o_date >= datetime('now', ?)
		GROUP BY date(o_date)
		ORDER BY day`, from)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var day string
		var e dayAgg
		if err := rows.Scan(&day, &e.oCount, &e.sceneCount); err != nil {
			rows.Close()
			return nil, err
		}
		agg[day] = &e
		dayOrder = append(dayOrder, day)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	// distinct scenes per day, for the altitude sum
	rows, err = dbWrapper.Queryx(ctx, `
		SELECT date(o_date) AS day, scene_id
		FROM scenes_o_dates
		WHERE o_date >= datetime('now', ?)
		GROUP BY date(o_date), scene_id`, from)
	if err != nil {
		return nil, err
	}
	var allIDs []int
	for rows.Next() {
		var day string
		var sceneID int
		if err := rows.Scan(&day, &sceneID); err != nil {
			rows.Close()
			return nil, err
		}
		if e, ok := agg[day]; ok {
			e.sceneIDs = append(e.sceneIDs, sceneID)
			allIDs = append(allIDs, sceneID)
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	heights := map[int]int{}
	if len(allIDs) > 0 {
		heights, err = s.GetSceneHeights(ctx, allIDs)
		if err != nil {
			return nil, err
		}
	}
	for _, e := range agg {
		for _, id := range e.sceneIDs {
			e.altitude += heights[id]
		}
	}

	// top performer per day (most O's that day)
	rows, err = dbWrapper.Queryx(ctx, `
		SELECT day, performer_name, performer_id FROM (
			SELECT date(od.o_date) AS day, p.name AS performer_name, p.id AS performer_id,
			       COUNT(*) AS c,
			       ROW_NUMBER() OVER (PARTITION BY date(od.o_date) ORDER BY COUNT(*) DESC, p.name) AS rn
			FROM scenes_o_dates od
			JOIN performers_scenes ps ON ps.scene_id = od.scene_id
			JOIN performers p ON p.id = ps.performer_id
			WHERE od.o_date >= datetime('now', ?)
			GROUP BY date(od.o_date), p.id
		) WHERE rn = 1`, from)
	if err != nil {
		return nil, err
	}
	topPerformer := map[string]string{}
	topPerformerID := map[string]int{}
	for rows.Next() {
		var day, name string
		var performerID int
		if err := rows.Scan(&day, &name, &performerID); err != nil {
			rows.Close()
			return nil, err
		}
		topPerformer[day] = name
		topPerformerID[day] = performerID
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	out := make([]*models.AIFlightLogEntry, 0, len(dayOrder))
	for _, day := range dayOrder {
		e := agg[day]
		entry := &models.AIFlightLogEntry{
			Date:       day,
			OCount:     e.oCount,
			SceneCount: e.sceneCount,
			Altitude:   e.altitude,
		}
		if name, ok := topPerformer[day]; ok {
			entry.TopPerformer = name
			id := topPerformerID[day]
			entry.TopPerformerID = &id
		}
		out = append(out, entry)
	}
	return out, nil
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
