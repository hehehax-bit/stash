CREATE TABLE ai_media_quality (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL,
  entity_id INTEGER NOT NULL,
  quality_score INTEGER NOT NULL DEFAULT 0,
  visual_clarity INTEGER NOT NULL DEFAULT 0,
  lighting INTEGER NOT NULL DEFAULT 0,
  composition INTEGER NOT NULL DEFAULT 0,
  camera_work INTEGER NOT NULL DEFAULT 0,
  notes TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX ai_media_quality_entity ON ai_media_quality (entity_type, entity_id);
