CREATE TABLE IF NOT EXISTS api_key_model_usage (
    api_key_id   INTEGER NOT NULL,
    model        TEXT    NOT NULL,
    request_made INTEGER NOT NULL DEFAULT 0,
    updated_at   DATETIME NOT NULL,
    PRIMARY KEY (api_key_id, model)
);
