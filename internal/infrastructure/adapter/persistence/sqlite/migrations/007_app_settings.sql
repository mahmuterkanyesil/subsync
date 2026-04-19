CREATE TABLE IF NOT EXISTS app_settings (
    key   TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);
INSERT OR IGNORE INTO app_settings (key, value) VALUES ('target_language', 'tr');
