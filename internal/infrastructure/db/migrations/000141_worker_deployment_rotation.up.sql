-- Worker deployment class, so the control plane can tell an ephemeral Cloud
-- Run worker (fresh process and egress IP every run) from a persistent one,
-- and when the current process started. Both come from the heartbeat.
ALTER TABLE workers ADD COLUMN IF NOT EXISTS deployment text NOT NULL DEFAULT '';
ALTER TABLE workers ADD COLUMN IF NOT EXISTS started_at timestamptz;

-- When a mailbox was placed on its current worker. SMTP/IMAP mailboxes are
-- rotated across ephemeral workers after a dwell time; this is the clock.
ALTER TABLE email_accounts ADD COLUMN IF NOT EXISTS worker_assigned_at timestamptz;
