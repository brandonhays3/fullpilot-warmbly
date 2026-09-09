// Global toast host. Every `toast.*` call in the app renders through this
// one component, so the size, card styling, and icons live here rather
// than in the library defaults: a compact white card in the dashboard's
// own tokens, bottom-right, with 14px status icons instead of the
// library's large coloured circles.

import { Toaster as HotToaster, resolveValue, type Toast } from "react-hot-toast";
import { motion } from "motion/react";

function StatusIcon({ t }: { t: Toast }) {
    // A caller-provided icon (toast(msg, { icon })) wins over the type icon.
    if (t.icon) return <span className="shrink-0 inline-flex items-center">{t.icon}</span>;
    switch (t.type) {
        case "success":
            return (
                <svg
                    className="w-3.5 h-3.5 shrink-0 text-emerald-600"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={2.5}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    aria-hidden
                >
                    <path d="M20 6 9 17l-5-5" />
                </svg>
            );
        case "error":
            return (
                <svg
                    className="w-3.5 h-3.5 shrink-0 text-rose-600"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={2}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    aria-hidden
                >
                    <circle cx="12" cy="12" r="9" />
                    <path d="M12 8v4.5M12 16h.01" />
                </svg>
            );
        case "loading":
            return (
                <span
                    className="w-3.5 h-3.5 shrink-0 rounded-full border-[1.5px] border-slate-200 border-t-slate-600 animate-spin"
                    aria-hidden
                />
            );
        default:
            return null;
    }
}

export function Toaster() {
    return (
        <HotToaster
            position="bottom-right"
            gutter={6}
            containerStyle={{ bottom: 16, right: 16 }}
            toastOptions={{
                duration: 4000,
                success: { duration: 3000 },
                error: { duration: 5000 },
            }}
        >
            {(t) => (
                <motion.div
                    initial={{ opacity: 0, y: 8, scale: 0.96 }}
                    animate={{
                        opacity: t.visible ? 1 : 0,
                        y: t.visible ? 0 : 8,
                        scale: t.visible ? 1 : 0.96,
                    }}
                    transition={{ duration: 0.18, ease: "easeOut" }}
                    role="status"
                    aria-live="polite"
                    className="pointer-events-auto flex items-center gap-2 max-w-[320px] rounded-md border border-slate-200 bg-white px-2.5 py-2 text-[12.5px] leading-[1.35] text-slate-800 shadow-[0_4px_14px_rgba(15,23,42,0.08)]"
                >
                    <StatusIcon t={t} />
                    <div className="min-w-0 break-words">{resolveValue(t.message, t)}</div>
                </motion.div>
            )}
        </HotToaster>
    );
}
