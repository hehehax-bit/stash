CREATE TABLE ai_performer_career (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  performer_id INTEGER NOT NULL,
  career_start INTEGER,
  career_end INTEGER,
  active_years TEXT NOT NULL DEFAULT '[]',
  primary_niches TEXT NOT NULL DEFAULT '[]',
  notable_studios TEXT NOT NULL DEFAULT '[]',
  career_highlights TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX ai_performer_career_performer ON ai_performer_career (performer_id);
