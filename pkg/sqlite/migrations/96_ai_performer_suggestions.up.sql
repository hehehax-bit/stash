CREATE TABLE IF NOT EXISTS "ai_performer_suggestions" (
  id integer not null primary key autoincrement,
  source_performer_id integer not null,
  target_performer_id integer not null,
  confidence real not null default 0,
  status text not null default 'pending',
  created_at integer not null,
  updated_at integer not null
);
CREATE UNIQUE INDEX IF NOT EXISTS "ai_performer_suggestions_pair" ON ai_performer_suggestions (source_performer_id, target_performer_id);
CREATE INDEX IF NOT EXISTS "ai_performer_suggestions_status" ON ai_performer_suggestions (status);
