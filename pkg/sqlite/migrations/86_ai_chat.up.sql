CREATE TABLE IF NOT EXISTS "ai_chat_sessions" (
  `id` varchar(32) not null primary key,
  `created_at` integer not null,
  `updated_at` integer not null
);

CREATE TABLE IF NOT EXISTS "ai_chat_messages" (
  `id` varchar(32) not null primary key,
  `session_id` varchar(32) not null,
  `role` varchar(20) not null,
  `content` text not null,
  `created_at` integer not null,
  FOREIGN KEY (`session_id`) REFERENCES `ai_chat_sessions`(`id`) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS `idx_ai_chat_messages_session_id` on `ai_chat_messages` (`session_id`);
