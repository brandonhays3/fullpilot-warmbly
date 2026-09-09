BEGIN;

DROP INDEX IF EXISTS public.idx_unibox_emails_campaign_thread;

ALTER TABLE public.unibox_emails
    DROP COLUMN IF EXISTS campaign_linked;

DROP TABLE IF EXISTS public.mailbox_sync_boundaries;

COMMIT;
