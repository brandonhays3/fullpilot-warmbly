-- Which body format a confirmed send used: 'text' (text/plain only) or
-- 'html' (multipart with the HTML part). Set at dispatch by the backend.
ALTER TABLE campaign_tasks ADD COLUMN IF NOT EXISTS send_format TEXT;
