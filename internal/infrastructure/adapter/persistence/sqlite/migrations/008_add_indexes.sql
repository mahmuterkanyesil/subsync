CREATE INDEX IF NOT EXISTS idx_subtitles_created_at ON subtitles(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_subtitles_series_name ON subtitles(series_name);
