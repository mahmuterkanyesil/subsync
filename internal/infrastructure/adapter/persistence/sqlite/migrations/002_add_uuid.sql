-- Migration 002: Subtitle entity'sine UUID domain kimliği eklenir.
-- Mevcut kayıtlar bir sonraki Save() çağrısında otomatik UUID alır.
ALTER TABLE subtitles ADD COLUMN id TEXT;
CREATE INDEX IF NOT EXISTS idx_subtitles_id ON subtitles(id);
