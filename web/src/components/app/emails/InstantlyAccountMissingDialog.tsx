// Shown when starting warmup answers `instantly_account_missing`: warmup on
// this install runs through Instantly, and a Google or Microsoft mailbox can
// only be added to an Instantly workspace from Instantly's own app. The user
// adds it there, then "Check again" retries the same start.

import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import { ExternalLinkIcon, FlameIcon, RefreshCwIcon, XIcon } from "@/components/icons";
import toast from "react-hot-toast";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { Loading } from "@/components/loader";

export const INSTANTLY_ACCOUNTS_URL = "https://app.instantly.ai/app/accounts";
export const INSTANTLY_ACCOUNT_MISSING = "instantly_account_missing";

const EASE = [0.22, 1, 0.36, 1] as const;

export default function InstantlyAccountMissingDialog({
    open,
    email,
    onClose,
    onRetry,
}: {
    open: boolean;
    email: string;
    onClose: () => void;
    /** Re-runs the start; resolves once Instantly accepted the mailbox. */
    onRetry: () => Promise<unknown>;
}) {
    const cardRef = useRef<HTMLDivElement>(null);
    const [busy, setBusy] = useState(false);

    useEffect(() => {
        if (!open) return;
        const previous = document.activeElement as HTMLElement | null;
        cardRef.current?.focus();
        const onKey = (e: KeyboardEvent) => {
            if (e.key !== "Escape") return;
            if (document.querySelector("[role='alertdialog']")) return;
            e.stopPropagation();
            onClose();
        };
        document.addEventListener("keydown", onKey, true);
        return () => {
            document.removeEventListener("keydown", onKey, true);
            previous?.focus?.();
        };
    }, [open, onClose]);

    const retry = async () => {
        if (busy) return;
        setBusy(true);
        try {
            await onRetry();
            toast.success(`Warmup started for ${email} through Instantly`);
            onClose();
        } catch (e) {
            const err = e as AppError;
            if (err.code === INSTANTLY_ACCOUNT_MISSING) {
                toast.error(`${email} is still not in your Instantly workspace`);
            } else {
                toast.error(buildError(err));
            }
        } finally {
            setBusy(false);
        }
    };

    // Portal events still bubble through the React tree, so the overlay stops
    // them before they reach the mailbox row's open handler.
    return createPortal(
        <AnimatePresence>
            {open && (
                <motion.div
                    key="instantly-missing-overlay"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.15 }}
                    onMouseDown={(e) => {
                        e.stopPropagation();
                        if (!busy) onClose();
                    }}
                    onClick={(e) => e.stopPropagation()}
                    className="fixed inset-0 z-[140] flex items-center justify-center bg-slate-900/40 backdrop-blur-[2px] px-4"
                >
                    <motion.div
                        key="instantly-missing-card"
                        ref={cardRef}
                        tabIndex={-1}
                        role="dialog"
                        aria-modal="true"
                        aria-label="Add this mailbox in Instantly first"
                        data-floating
                        data-nested-modal
                        initial={{ y: 10, opacity: 0, scale: 0.98 }}
                        animate={{ y: 0, opacity: 1, scale: 1 }}
                        exit={{ y: 10, opacity: 0, scale: 0.98 }}
                        transition={{ duration: 0.2, ease: EASE }}
                        onMouseDown={(e) => e.stopPropagation()}
                        className="w-full max-w-[460px] rounded-lg bg-white border border-slate-200 shadow-[0_24px_48px_-12px_rgba(15,23,42,0.22),0_8px_16px_-8px_rgba(15,23,42,0.12)] overflow-hidden outline-none"
                    >
                        <div className="px-5 h-14 flex items-center gap-3 border-b border-slate-200">
                            <div className="w-8 h-8 rounded-lg bg-orange-50 text-orange-600 flex items-center justify-center shrink-0">
                                <FlameIcon className="w-4 h-4" />
                            </div>
                            <div className="min-w-0 flex-1">
                                <div className="text-[13px] font-medium text-slate-900">Add this mailbox in Instantly first</div>
                                <div className="text-[11px] text-slate-400 truncate">{email}</div>
                            </div>
                            <button
                                type="button"
                                onClick={onClose}
                                disabled={busy}
                                aria-label="Close"
                                className="w-7 h-7 rounded-md flex items-center justify-center text-slate-400 hover:text-slate-900 hover:bg-slate-100 transition-colors disabled:opacity-50"
                            >
                                <XIcon className="w-4 h-4" />
                            </button>
                        </div>

                        <div className="px-5 py-4 space-y-3">
                            <p className="text-[12.5px] text-slate-700 leading-relaxed">
                                Warmup on this workspace runs through Instantly, and this mailbox is not in your Instantly
                                workspace yet. Google and Microsoft mailboxes can only be connected from Instantly's own app,
                                because their sign-in has to happen there.
                            </p>
                            <ol className="text-[12px] text-slate-600 leading-relaxed list-decimal pl-4 space-y-1">
                                <li>Open Instantly and add <b className="text-slate-900">{email}</b> under Accounts.</li>
                                <li>Come back here and check again. Fullpilot applies the warmup settings and turns warmup on.</li>
                            </ol>
                        </div>

                        <div className="px-5 py-3 border-t border-slate-200 flex items-center justify-end gap-2">
                            <a
                                href={INSTANTLY_ACCOUNTS_URL}
                                target="_blank"
                                rel="noreferrer"
                                className="h-8 px-3 rounded-md border border-slate-200 hover:border-slate-300 text-[12px] font-medium text-slate-700 hover:text-slate-900 inline-flex items-center gap-1.5 transition-colors"
                            >
                                <ExternalLinkIcon className="w-3.5 h-3.5" />
                                Open Instantly
                            </a>
                            <button
                                type="button"
                                onClick={() => void retry()}
                                disabled={busy}
                                className="h-8 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                            >
                                {busy ? <Loading className="!w-3.5 h-3.5 text-white" /> : <RefreshCwIcon className="w-3.5 h-3.5" />}
                                Check again
                            </button>
                        </div>
                    </motion.div>
                </motion.div>
            )}
        </AnimatePresence>,
        document.body,
    );
}
