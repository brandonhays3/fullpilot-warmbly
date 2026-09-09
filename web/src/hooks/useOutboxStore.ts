// Reply-restore handoff. A standalone zustand store (mirroring
// useComposeStore) so any surface can hand ThreadView a reply payload to
// reopen its composer with, without going through the app store.
//
// The pending "undo send" queue that used to live here is gone: instant
// sends leave immediately (undo_send_seconds defaults to 0), so there is no
// countdown pill and nothing to cancel.

import { create } from "zustand";

// Everything needed to reopen the thread's reply composer exactly as it
// was when a reply is handed back.
export interface OutboxReplyPayload {
    threadId: string;
    messageId: string;
    mode: "reply" | "forward";
    to: string[];
    cc: string[];
    bcc: string[];
    subject: string;
    body: string;
}

interface OutboxStore {
    // A reply waiting for its thread to be open; ThreadView consumes it and
    // reopens the reply composer with the payload.
    pendingReplyRestore: OutboxReplyPayload | null;
    setReplyRestore: (restore: OutboxReplyPayload | null) => void;
}

export const useOutboxStore = create<OutboxStore>((set) => ({
    pendingReplyRestore: null,
    setReplyRestore: (restore) => set({ pendingReplyRestore: restore }),
}));
