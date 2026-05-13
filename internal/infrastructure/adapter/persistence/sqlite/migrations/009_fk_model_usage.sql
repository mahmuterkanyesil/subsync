PRAGMA foreign_keys = OFF;

CREATE TABLE api_key_model_usage_new (
    api_key_id   INTEGER NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    model        TEXT    NOT NULL,
    request_made INTEGER NOT NULL DEFAULT 0,
    updated_at   DATETIME NOT NULL,
    PRIMARY KEY (api_key_id, model)
);

INSERT INTO api_key_model_usage_new
SELECT mu.api_key_id, mu.model, mu.request_made, mu.updated_at
FROM api_key_model_usage mu
INNER JOIN api_keys ak ON ak.id = mu.api_key_id;

DROP TABLE api_key_model_usage;

ALTER TABLE api_key_model_usage_new RENAME TO api_key_model_usage;

PRAGMA foreign_keys = ON;
