CREATE TABLE IF NOT EXISTS "ai_suggestions" (
  `id` integer not null primary key autoincrement,
  `entity_type` text not null,
  `entity_id` integer not null,
  `title` text not null default '',
  `details` text not null default '',
  `performers` text not null default '[]',
  `tags` text not null default '[]',
  `status` text not null default 'pending',
  `created_at` integer not null,
  `updated_at` integer not null
);

CREATE INDEX IF NOT EXISTS "index_ai_suggestions_entity" ON "ai_suggestions" (`entity_type`, `entity_id`);
CREATE INDEX IF NOT EXISTS "index_ai_suggestions_status" ON "ai_suggestions" (`status`);
