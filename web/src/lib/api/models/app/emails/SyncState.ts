// GET /emails/:id/sync: where a mailbox's import stands, whether fair use is
// holding new mail, and the budget it runs under. state is null until the
// worker has reported once (a mailbox connected seconds ago).
export type SyncBackfillStatus = "pending" | "running" | "complete";

export type SyncThrottleReason =
    | "burst"
    | "hourly"
    | "daily"
    | "org_daily"
    | "priority_daily";

export interface SyncState {
    backfill_status: SyncBackfillStatus;
    backfill_synced: number;
    backfill_since?: string;
    backfill_started_at?: string;
    backfill_completed_at?: string;
    throttled_until?: string;
    throttle_reason?: SyncThrottleReason | "";
    // Live messages seen on the server but waiting on budget.
    deferred: number;
    /** Folders the last listing could not follow because the mailbox has more than the sync covers. */
    folders_skipped_cap?: number;
    /** Folders left out because the mail server listed their name more than once. */
    folders_skipped_conflict?: number;
    last_synced_at?: string;
}

export interface SyncPolicy {
    backfill_days: number;
    backfill_messages: number;
    daily_messages: number;
    org_daily_messages: number;
}

// Where the mailbox runs right now. SMTP/IMAP mailboxes rotate across
// ephemeral workers, so this changes every few minutes for them.
export interface SyncWorker {
    id: string;
    deployment: "cloud_run_worker" | "cloud_vm" | "cloud_dev" | "";
    ephemeral: boolean;
    started_at?: string;
    last_seen_at?: string;
}

export default interface EmailSync {
    state: SyncState | null;
    policy: SyncPolicy;
    // null while the mailbox is not placed on a worker yet.
    worker: SyncWorker | null;
}
