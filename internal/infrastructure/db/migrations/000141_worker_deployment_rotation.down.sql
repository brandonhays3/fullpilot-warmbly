ALTER TABLE email_accounts DROP COLUMN IF EXISTS worker_assigned_at;
ALTER TABLE workers DROP COLUMN IF EXISTS started_at;
ALTER TABLE workers DROP COLUMN IF EXISTS deployment;
