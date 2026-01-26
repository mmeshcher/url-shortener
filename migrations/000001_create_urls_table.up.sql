CREATE TABLE IF NOT EXISTS urls (
    id SERIAL PRIMARY KEY
    short_id VARCHAR(10) NOT NULL UNIQUE,
    original_url TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_urls_short_id ON urls(short_id);
CREATE INDEX IF NOT EXISTS idx_urls_original_url ON urls(original_url);