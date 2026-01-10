DROP INDEX IF EXISTS idx_original_url;
 
ALTER TABLE urls
ADD CONSTRAINT urls_original_url_key UNIQUE (original_url);

CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);