CREATE TABLE IF NOT EXISTS "ai_audits" (
  id integer not null primary key autoincrement,
  entity_type text not null,
  entity_id integer not null,
  field text not null,
  current_value text not null default '',
  ai_value text not null default '',
  status text not null default 'pending',
  created_at integer not null,
  updated_at integer not null
);
CREATE INDEX IF NOT EXISTS "ai_audits_status" ON ai_audits (status);
CREATE INDEX IF NOT EXISTS "ai_audits_entity" ON ai_audits (entity_type, entity_id);
