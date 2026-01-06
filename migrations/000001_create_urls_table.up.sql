CREATE TABLE IF NOT EXISTS urls (
    id VARCHAR(10) PRIMARY KEY,
    original_url TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);