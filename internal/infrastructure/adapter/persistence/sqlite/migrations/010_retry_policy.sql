ALTER TABLE subtitles ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE subtitles ADD COLUMN last_retry_at DATETIME;
CREATE INDEX IF NOT EXISTS idx_subtitles_status ON subtitles(status);
