// Shape of GET /emails/:id/warmup/instantly. `configured` is whether the
// install has an Instantly API key at all; `enrolled` is whether this mailbox
// is the one Instantly warms (warmup_provider = "instantly"); `found` is
// whether Instantly's workspace knows the address.

export interface InstantlyWarmupAnalytics {
    sent: number;
    received: number;
    landed_inbox: number;
    landed_spam: number;
    health_score: number;
    health_score_label: string;
}

export interface InstantlyWarmupSettings {
    limit: number;
    /** "disabled" or "0".."4" emails added per day. */
    increment: string;
    /** Fraction, 0.65 is 65%. */
    reply_rate: number;
    advanced?: {
        open_rate: number;
        important_rate: number;
        read_emulation: boolean;
        spam_save_rate: number;
        weekday_only: boolean;
    };
}

export interface InstantlyWarmupStatus {
    email: string;
    active: boolean;
    warmup_status: number;
    warmup_label: string;
    account_status: number;
    account_label: string;
    setup_pending: boolean;
    started_at?: string | null;
    warmup_score?: number | null;
    settings?: InstantlyWarmupSettings;
    analytics?: InstantlyWarmupAnalytics;
}

export default interface InstantlyWarmup {
    configured: boolean;
    enrolled: boolean;
    found: boolean;
    dashboard_url: string;
    status?: InstantlyWarmupStatus;
}
