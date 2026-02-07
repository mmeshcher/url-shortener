ALTER TABLE urls ADD COLUMN user_id VARCHAR(36);

CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls(user_id);

UPDATE urls SET user_id = 'anonymous' WHERE user_id IS NULL;