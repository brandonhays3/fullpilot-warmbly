-- Undo send is off by default: instant sends leave immediately. 0 is now a
-- valid window (config.UndoSendSecondsMin), the default, and what every
-- existing user gets, since the dashboard no longer exposes the setting.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_undo_send_seconds_range;
ALTER TABLE users ADD CONSTRAINT users_undo_send_seconds_range
    CHECK (undo_send_seconds >= 0 AND undo_send_seconds <= 120);
ALTER TABLE users ALTER COLUMN undo_send_seconds SET DEFAULT 0;
UPDATE users SET undo_send_seconds = 0 WHERE undo_send_seconds <> 0;
