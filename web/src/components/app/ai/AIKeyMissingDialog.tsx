// Shown when someone tries to add an AI block (or preview one) and the
// workspace has no OpenRouter key. Nothing is inserted: the block would only
// fail at send time. One way forward, Settings > AI, where a member with
// Manage settings pastes the key.

import React from "react";
import { createPortal } from "react-dom";
import { Link } from "react-router-dom";
import { KeyRoundIcon } from "@/components/icons";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { AI_SETTINGS_PATH } from "@/lib/api/models/app/organizations/AISettings";

export default function AIKeyMissingDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
    return (
        // Non-modal with its own backdrop, like the AI block config dialog, so
        // it works from inside the step editor's own layered dialog.
        <Dialog open={open} onOpenChange={(o) => !o && onClose()} modal={false}>
            {open &&
                createPortal(
                    <div className="fixed inset-0 z-[215] bg-slate-900/30 duration-200 animate-in fade-in-0" aria-hidden />,
                    document.body,
                )}
            <DialogContent
                showCloseButton={false}
                className="z-[220] gap-0 overflow-hidden p-0 sm:max-w-[420px]"
                onOpenAutoFocus={(e) => e.preventDefault()}
            >
                <div className="px-5 pt-5 pb-4">
                    <div className="flex items-start gap-3">
                        <span className="shrink-0 w-8 h-8 bg-sky-50 text-sky-600 inline-flex items-center justify-center">
                            <KeyRoundIcon className="w-4 h-4" />
                        </span>
                        <div className="min-w-0">
                            <DialogTitle className="text-[14px] font-semibold text-slate-900 tracking-tight">
                                AI blocks need your OpenRouter key
                            </DialogTitle>
                            <DialogDescription className="mt-1 text-[12.5px] leading-relaxed text-slate-600">
                                AI blocks are written with your workspace's own OpenRouter key, so OpenRouter bills you
                                directly. Add a key under Settings, AI, then come back and insert the block. Until then a
                                campaign with an AI block cannot start.
                            </DialogDescription>
                        </div>
                    </div>
                </div>
                <div className="flex items-center justify-end gap-2 border-t border-slate-200 bg-slate-50/50 px-5 py-3">
                    <button
                        type="button"
                        onClick={onClose}
                        className="h-7 px-3 border border-slate-200 bg-white text-[12.5px] font-medium text-slate-700 transition-colors hover:border-slate-300 hover:text-slate-900"
                    >
                        Not now
                    </button>
                    <Link
                        to={AI_SETTINGS_PATH}
                        onClick={onClose}
                        className="h-7 px-3 inline-flex items-center bg-sky-600 text-[12.5px] font-medium text-white transition-colors hover:bg-sky-700"
                    >
                        Open Settings, AI
                    </Link>
                </div>
            </DialogContent>
        </Dialog>
    );
}
