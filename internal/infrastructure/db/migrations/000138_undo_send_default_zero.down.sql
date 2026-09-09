-- Restore the 30 second default and the 5 second floor from 000075.
ALTER TABLE users ALTER COLUMN undo_send_seconds SET DEFAULT 30;
UPDATE users SET undo_send_seconds = 30 WHERE undo_send_seconds < 5;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_undo_send_seconds_range;
ALTER TABLE users ADD CONSTRAINT users_undo_send_seconds_range
    CHECK (undo_send_seconds >= 5 AND undo_send_seconds <= 120);
