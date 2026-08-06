CREATE TABLE IF NOT EXISTS "ai_saved_moments" (
  id integer not null primary key autoincrement,
  marker_id integer not null,
  created_at integer not null
);
CREATE UNIQUE INDEX IF NOT EXISTS "ai_saved_moments_marker" ON ai_saved_moments (marker_id);
