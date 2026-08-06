CREATE TABLE IF NOT EXISTS "scene_markers_performers" (
  scene_marker_id integer not null,
  performer_id integer not null,
  primary key (scene_marker_id, performer_id)
);
CREATE INDEX IF NOT EXISTS "index_scene_markers_performers_performer" ON scene_markers_performers (performer_id);
