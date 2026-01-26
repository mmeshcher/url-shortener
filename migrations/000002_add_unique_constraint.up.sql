BEGIN;

DROP INDEX IF EXISTS idx_original_url;

ALTER TABLE urls ADD CONSTRAINT urls_original_url_unique UNIQUE (original_url);

CREATE INDEX idx_original_url ON urls(original_url);

COMMIT;