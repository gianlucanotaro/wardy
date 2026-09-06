ALTER TABLE usage ADD COLUMN user_id INTEGER;
CREATE INDEX idx_usage_user_daily ON usage(user_id, model, created_at);
