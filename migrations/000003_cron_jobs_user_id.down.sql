DROP INDEX IF EXISTS idx_cron_jobs_agent_user;
DROP INDEX IF EXISTS idx_cron_jobs_user_id;
ALTER TABLE cron_jobs DROP COLUMN IF EXISTS interval_ms;
ALTER TABLE cron_jobs DROP COLUMN IF EXISTS user_id;
