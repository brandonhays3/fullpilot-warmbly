-- Sync start boundary and inbox triage.
--
-- mailbox_sync_boundaries remembers, per organization and address, the moment
-- the mailbox was first connected. Nothing received before that moment is ever
-- imported or processed, and the row is never moved forward: disconnecting and
-- reconnecting the same address keeps its original boundary. Existing mailboxes
-- get their connect time.
--
-- unibox_emails.campaign_linked marks mail that belongs to a campaign
-- conversation (a reply to something Warmbly sent, or a message in a thread
-- Warmbly started). Only that mail is shown in the Inbox and classified;
-- everything else lands in the Other view untouched.

BEGIN;

CREATE TABLE public.mailbox_sync_boundaries (
    organization_id uuid NOT NULL REFERENCES public.organizations (id) ON DELETE CASCADE,
    email text NOT NULL CHECK (email = lower(email)),
    sync_since timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, email)
);

INSERT INTO public.mailbox_sync_boundaries (organization_id, email, sync_since)
SELECT organization_id, lower(email), min(created_at)
FROM public.email_accounts
WHERE organization_id IS NOT NULL
GROUP BY organization_id, lower(email)
ON CONFLICT DO NOTHING;

ALTER TABLE public.unibox_emails
    ADD COLUMN campaign_linked boolean NOT NULL DEFAULT false;

-- Threads containing a message Warmbly sent, or a reply to one, are campaign
-- conversations; every message in such a thread is linked.
WITH linked AS (
    SELECT DISTINCT ue.email_id, ue.thread_id
    FROM public.unibox_emails ue
    JOIN public.tasks t
      ON t.email_account_id = ue.email_id
     AND t.message_id <> ''
     AND (
         t.message_id = ue.message_id
         OR t.message_id = ue.parent_id
         OR t.message_id = ue.thread_id
         OR t.message_id = ANY (ue.in_reply_to)
     )
    WHERE ue.thread_id <> ''
)
UPDATE public.unibox_emails ue
SET campaign_linked = true
FROM linked l
WHERE ue.email_id = l.email_id AND ue.thread_id = l.thread_id;

UPDATE public.unibox_emails ue
SET campaign_linked = true
WHERE ue.thread_id = ''
  AND EXISTS (
      SELECT 1 FROM public.tasks t
      WHERE t.email_account_id = ue.email_id
        AND t.message_id <> ''
        AND (
            t.message_id = ue.message_id
            OR t.message_id = ue.parent_id
            OR t.message_id = ANY (ue.in_reply_to)
        )
  );

-- The consumer asks "is this thread already a campaign conversation?" once
-- per new message.
CREATE INDEX idx_unibox_emails_campaign_thread
    ON public.unibox_emails USING btree (email_id, thread_id)
    WHERE campaign_linked;

COMMIT;
