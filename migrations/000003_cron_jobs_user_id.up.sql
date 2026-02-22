ALTER TABLE cron_jobs ADD COLUMN IF NOT EXISTS user_id TEXT;
ALTER TABLE cron_jobs ADD COLUMN IF NOT EXISTS interval_ms BIGINT;
CREATE INDEX IF NOT EXISTS idx_cron_jobs_user_id ON cron_jobs (user_id);
CREATE INDEX IF NOT EXISTS idx_cron_jobs_agent_user ON cron_jobs (agent_id, user_id);
