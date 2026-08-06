CREATE TABLE IF NOT EXISTS "ai_performer_candidates" (
  id integer not null primary key autoincrement,
  name text not null default '',
  description text not null default '',
  member_count integer not null default 0,
  entity_type text not null default '',
  entity_id integer not null default 0,
  member_ids text not null default '[]',
  status text not null default 'pending',
  created_at integer not null,
  updated_at integer not null
);
CREATE INDEX IF NOT EXISTS "ai_performer_candidates_status" ON ai_performer_candidates (status);
