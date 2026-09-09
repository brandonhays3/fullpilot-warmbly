-- Record who warms a mailbox. 'internal' is Warmbly's own pool; 'instantly'
-- means the mailbox is enrolled in Instantly.ai warmup through its API and the
-- local warmup scheduler must leave it alone.
ALTER TABLE email_accounts
    ADD COLUMN warmup_provider text NOT NULL DEFAULT 'internal'
    CHECK (warmup_provider IN ('internal', 'instantly'));

COMMENT ON COLUMN email_accounts.warmup_provider IS
    'Who runs warmup for this mailbox: internal (Warmbly pool) or instantly (Instantly.ai via API). The local scheduler skips instantly.';
