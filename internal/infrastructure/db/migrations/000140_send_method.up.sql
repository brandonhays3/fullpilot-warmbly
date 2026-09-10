-- How each send left the worker: <transport>_<provider>_<deployment>, for
-- example smtp_google_cloud_run_worker or api_microsoft_cloud_vm. Stamped by
-- the consumer from the worker's EMAIL_SENT result, so it is NULL until the
-- send is confirmed and for every send made before the stamp existed.
ALTER TABLE campaign_tasks ADD COLUMN IF NOT EXISTS send_method text;
ALTER TABLE warmup_tasks ADD COLUMN IF NOT EXISTS send_method text;

-- The per-method breakdown resolves each progress row's method through its
-- confirmed task; only stamped rows take part.
CREATE INDEX IF NOT EXISTS campaign_tasks_send_method_progress_idx
    ON campaign_tasks (campaign_id, contact_id, sequence_id)
    WHERE send_method IS NOT NULL;
