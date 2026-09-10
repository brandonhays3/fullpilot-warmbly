// Shown on a campaign parked at paused_ai_key: its copy carries an AI block
// and the workspace has no OpenRouter key, or OpenRouter refused the one it
// has. The fix lives under Settings > AI; saving a key there resumes the
// campaign on its own, and Start is offered for the case where the key was
// already fixed by someone else.

import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { Link } from "react-router-dom";
import { KeyRoundIcon, Loader2Icon, PlayIcon } from "@/components/icons";
import toast from "react-hot-toast";

import PermissionButton from "@/components/ui/PermissionButton";
import useStartCampaign from "@/lib/api/hooks/app/campaigns/useStartCampaign";
import { useAISettings } from "@/lib/api/hooks/app/organizations/useAISettings";
import { AI_SETTINGS_PATH } from "@/lib/api/models/app/organizations/AISettings";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

export default function AIKeyBanner({ campaignId, status }: { campaignId: string; status: string }) {
    const shown = status === "paused_ai_key";
    const settings = useAISettings(shown);
    const start = useStartCampaign();
    const hasKey = !!settings.data?.has_key;

    const resume = async () => {
        try {
            await start.mutateAsync(campaignId);
            toast.success("Campaign resumed");
        } catch (e) {
            toast.error(buildError(e as AppError));
        }
    };

    return (
        <AnimatePresence initial={false}>
            {shown && (
                <motion.div
                    key="ai-key"
                    initial={{ opacity: 0, y: -8, height: 0 }}
                    animate={{ opacity: 1, y: 0, height: "auto" }}
                    exit={{ opacity: 0, y: -8, height: 0 }}
                    transition={{ type: "spring", duration: 0.4, bounce: 0.2 }}
                    className="overflow-hidden"
                >
                    <div className="mx-5 mb-3 border border-amber-200 bg-amber-50/70 px-3.5 py-3 flex flex-col md:flex-row md:items-center gap-3">
                        <span className="shrink-0 w-7 h-7 bg-amber-100 text-amber-700 inline-flex items-center justify-center">
                            <KeyRoundIcon className="w-4 h-4" />
                        </span>
                        <div className="min-w-0 flex-1">
                            <p className="text-[12.5px] font-medium text-amber-900">
                                {hasKey
                                    ? "Sending paused: OpenRouter refused the workspace's API key"
                                    : "Sending paused: this campaign's AI blocks need an OpenRouter key"}
                            </p>
                            <p className="text-[11.5px] text-amber-800/80 leading-snug mt-0.5">
                                {hasKey
                                    ? `The key ending in •••• ${settings.data?.key_last4} was rejected. Replace it under `
                                    : "AI blocks are written with your workspace's own key. Add one under "}
                                <Link to={AI_SETTINGS_PATH} className="underline underline-offset-2 hover:text-amber-900">
                                    Settings, AI
                                </Link>
                                {hasKey ? "; the campaign resumes as soon as a working key is saved." : "; the campaign resumes as soon as it is saved."}
                            </p>
                        </div>
                        <div className="shrink-0 flex items-center gap-1.5">
                            <Link
                                to={AI_SETTINGS_PATH}
                                className="h-7 px-2.5 border border-amber-300 bg-white hover:bg-amber-100 text-amber-900 text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors"
                            >
                                <KeyRoundIcon className="w-3.5 h-3.5" />
                                {hasKey ? "Replace key" : "Add key"}
                            </Link>
                            {hasKey && (
                                <PermissionButton
                                    permission="SEND_CAMPAIGNS"
                                    type="button"
                                    onClick={() => void resume()}
                                    disabled={start.isPending}
                                    className="h-7 px-2.5 bg-amber-600 hover:bg-amber-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                                >
                                    {start.isPending ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <PlayIcon className="w-3.5 h-3.5" />}
                                    Start again
                                </PermissionButton>
                            )}
                        </div>
                    </div>
                </motion.div>
            )}
        </AnimatePresence>
    );
}
