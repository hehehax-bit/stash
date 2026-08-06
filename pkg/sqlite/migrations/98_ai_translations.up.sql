CREATE TABLE IF NOT EXISTS "ai_translations" (
  id integer not null primary key autoincrement,
  entity_type text not null,
  entity_id integer not null,
  language text not null,
  field text not null,
  translated_text text not null default '',
  status text not null default 'pending',
  created_at integer not null,
  updated_at integer not null
);
CREATE UNIQUE INDEX IF NOT EXISTS "ai_translations_unique" ON ai_translations (entity_type, entity_id, language, field);
CREATE INDEX IF NOT EXISTS "ai_translations_status" ON ai_translations (status);
