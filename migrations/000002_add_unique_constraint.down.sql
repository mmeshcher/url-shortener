ALTER TABLE urls
DROP CONSTRAINT IF EXISTS urls_original_url_key;

CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);