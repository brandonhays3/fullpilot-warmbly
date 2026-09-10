-- Postgres cannot drop a single enum value, so 'paused_ai_key' stays on
-- campaign_status. Campaigns parked in it return to a plain pause first.
UPDATE campaigns SET status = 'paused' WHERE status = 'paused_ai_key';

DROP TABLE IF EXISTS organization_ai_settings;
