CREATE TABLE ai_scene_audio (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  scene_id INTEGER NOT NULL,
  has_audio INTEGER NOT NULL DEFAULT 0,
  silence_ratio INTEGER NOT NULL DEFAULT 0,
  music INTEGER NOT NULL DEFAULT 0,
  speech INTEGER NOT NULL DEFAULT 0,
  moans INTEGER NOT NULL DEFAULT 0,
  ambient INTEGER NOT NULL DEFAULT 0,
  transcript TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  audio_codec TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX ai_scene_audio_scene ON ai_scene_audio (scene_id);
