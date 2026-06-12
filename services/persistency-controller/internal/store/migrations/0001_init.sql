CREATE TABLE IF NOT EXISTS urls (
    code       TEXT PRIMARY KEY,
    long_url   TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS urls_expires_at_idx ON urls (expires_at);
