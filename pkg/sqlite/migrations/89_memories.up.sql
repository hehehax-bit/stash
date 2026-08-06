CREATE TABLE IF NOT EXISTS "ai_memories" (
  `id` integer not null primary key autoincrement,
  `key` text not null unique,
  `value` text not null,
  `created_at` integer not null,
  `updated_at` integer not null
);
