-- Which OAuth client connected a mailbox. Refresh tokens are bound to the
-- client that issued them, so a deployment with two Gmail clients (the web
-- client and a desktop-type one) has to refresh each mailbox with the right
-- one. 'default' is the BOX_GOOGLE_* / BOX_OUTLOOK_* client every existing row
-- was connected with.
ALTER TABLE email_accounts_oauth
    ADD COLUMN IF NOT EXISTS oauth_client text NOT NULL DEFAULT 'default';
