CREATE TABLE ai_file_rename (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL,
  entity_id INTEGER NOT NULL,
  current_name TEXT NOT NULL,
  suggested_name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX ai_file_rename_entity ON ai_file_rename (entity_type, entity_id);
