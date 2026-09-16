-- Step 1: data model

CREATE TABLE IF NOT EXISTS urls (
    short_code  VARCHAR(10) PRIMARY KEY,
    long_url    TEXT NOT NULL,
    user_id     BIGINT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_urls_created_at ON urls (created_at);