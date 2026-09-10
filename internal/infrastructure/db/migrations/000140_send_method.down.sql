DROP INDEX IF EXISTS campaign_tasks_send_method_progress_idx;
ALTER TABLE warmup_tasks DROP COLUMN IF EXISTS send_method;
ALTER TABLE campaign_tasks DROP COLUMN IF EXISTS send_method;
