-- Per-workspace AI provider settings: the OpenRouter key a workspace pays
-- with and the model its AI blocks default to. The key is sealed with the
-- organization DEK; only its last four characters are kept in the clear so
-- the settings page can show which key is set without decrypting it.
CREATE TABLE IF NOT EXISTS organization_ai_settings (
    organization_id    uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    provider           text NOT NULL DEFAULT 'openrouter' CHECK (provider = 'openrouter'),
    api_key_ciphertext text,
    api_key_last4      text NOT NULL DEFAULT '',
    model              text NOT NULL DEFAULT '',
    updated_at         timestamptz NOT NULL DEFAULT now()
);

-- A campaign whose email carries an AI block parks here when the workspace
-- has no OpenRouter key (or the provider rejects it), resumable once a key
-- is saved under Settings > AI.
ALTER TYPE public.campaign_status ADD VALUE IF NOT EXISTS 'paused_ai_key';
