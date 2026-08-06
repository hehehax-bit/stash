CREATE TABLE IF NOT EXISTS "ai_saved_plans" (
  id integer not null primary key autoincrement,
  name text not null,
  scene_ids text not null default '[]',
  created_at integer not null
);
