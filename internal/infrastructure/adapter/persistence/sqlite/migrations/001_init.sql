CREATE TABLE IF NOT EXISTS subtitles (
    eng_path        TEXT PRIMARY KEY,
    media_type      TEXT NOT NULL,
    series_name     TEXT NOT NULL DEFAULT '',
    season_number   INTEGER NOT NULL DEFAULT 0,
    episode_number  INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'queued',
    last_error      TEXT NOT NULL DEFAULT '',
    embedded        INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL,
    updated_at      DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS api_keys (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    service             TEXT NOT NULL,
    key_value           TEXT NOT NULL,
    is_active           INTEGER NOT NULL DEFAULT 1,
    is_quota_exceeded   INTEGER NOT NULL DEFAULT 0,
    quota_reset_time    DATETIME,
    request_made        INTEGER NOT NULL DEFAULT 0,
    last_used_at        DATETIME,
    last_error          TEXT NOT NULL DEFAULT '',
    created_at          DATETIME NOT NULL,
    updated_at          DATETIME NOT NULL,
    UNIQUE(service, key_value)
);

CREATE INDEX IF NOT EXISTS idx_subtitles_status ON subtitles(status);
CREATE INDEX IF NOT EXISTS idx_subtitles_embedded ON subtitles(embedded);
CREATE INDEX IF NOT EXISTS idx_api_keys_service_active ON api_keys(service, is_active, is_quota_exceeded);
