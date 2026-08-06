CREATE TABLE IF NOT EXISTS "ai_scene_moods" (
  id integer not null primary key autoincrement,
  scene_id integer not null,
  mood text not null,
  confidence real not null default 0,
  created_at integer not null
);
CREATE INDEX IF NOT EXISTS "ai_scene_moods_scene" ON ai_scene_moods (scene_id);
