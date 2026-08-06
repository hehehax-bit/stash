CREATE TABLE IF NOT EXISTS "embeddings" (
  `id` integer not null primary key autoincrement,
  `entity_type` text not null,
  `entity_id` integer not null,
  `model` text not null,
  `embedding` blob not null,
  `created_at` integer not null
);

CREATE UNIQUE INDEX IF NOT EXISTS `idx_embeddings_entity` on `embeddings` (`entity_type`, `entity_id`, `model`);
