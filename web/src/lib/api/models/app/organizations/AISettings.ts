// Workspace AI settings: the OpenRouter key every AI feature runs on and the
// default model. GET never returns the key, only whether one is set and its
// last four characters.
export interface AISettings {
    provider: "openrouter";
    has_key: boolean;
    key_last4: string;
    model: string;
    updated_at?: string | Date;
}

// PUT /organization/current/ai. Send `api_key` to replace the key, `model` to
// change the default ("" restores the built-in default), or both.
export interface UpdateAISettingsRequest {
    api_key?: string;
    model?: string;
}

// One entry of OpenRouter's model catalog, prices in USD per million tokens.
export interface AIModel {
    id: string;
    name: string;
    context_length: number;
    prompt_per_million: number;
    completion_per_million: number;
}

// Stable `code` values the API answers when the workspace key is missing or
// refused, so callers can branch to a Settings > AI link.
export const AI_KEY_MISSING = "ai_key_missing";
export const CAMPAIGN_AI_KEY_MISSING = "campaign_ai_key_missing";
export const AI_KEY_REJECTED = "ai_key_rejected";
export const AI_SETTINGS_PATH = "/app/settings/ai";
export const OPENROUTER_KEYS_URL = "https://openrouter.ai/keys";

// formatModelPrice renders a per-million price for a picker row: "$0.15" or
// "free" for a zero price, three decimals below a cent.
export function formatModelPrice(perMillion: number): string {
    if (!perMillion || perMillion <= 0) return "free";
    if (perMillion < 0.01) return `$${perMillion.toFixed(3)}`;
    return `$${perMillion.toFixed(2)}`;
}
