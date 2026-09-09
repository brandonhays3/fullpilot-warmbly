import React from "react";
import { Navigate, useLocation, useNavigate, useOutlet } from "react-router-dom";
import { AnimatePresence, motion } from "motion/react";
import { APP_URL, WEBSITE_URL } from "@/lib/information";
import getToken from "@/lib/helper/getToken";
import { Wordmark } from "@/components/svg";

// Auth layout: one centered card on a quiet page. Logo above, form inside,
// legal links below. Same on every viewport.
export default function AuthLayout({
    redirectIfAuthenticated = true,
}: { redirectIfAuthenticated?: boolean } = {}) {
    const navigate = useNavigate();
    const location = useLocation();
    const outlet = useOutlet();

    React.useEffect(() => {
        const receiveMessage = (event: MessageEvent) => {
            if (event.origin !== APP_URL) return;
            if (event.data?.type === "auth") navigate("/app/emails");
        };
        window.addEventListener("message", receiveMessage);
        return () => window.removeEventListener("message", receiveMessage);
    }, [navigate]);

    if (redirectIfAuthenticated && getToken()) {
        return <Navigate to="/app/emails" replace />;
    }

    return (
        <div className="flex min-h-dvh w-full items-center justify-center bg-slate-50 px-4 py-10 text-slate-900">
            <div className="w-full max-w-[420px]">
                <a href={WEBSITE_URL} className="mb-6 flex w-fit items-center mx-auto">
                    <Wordmark size={28} />
                </a>

                <div className="rounded-2xl border border-slate-200 bg-white px-6 py-8 shadow-[0_1px_2px_rgba(15,23,42,0.04),0_24px_48px_-24px_rgba(15,23,42,0.18)] sm:px-9">
                    <AnimatePresence mode="wait" initial={false}>
                        <motion.div
                            key={location.pathname}
                            initial={{ opacity: 0, x: 12 }}
                            animate={{ opacity: 1, x: 0 }}
                            exit={{ opacity: 0, x: -12 }}
                            transition={{ duration: 0.28, ease: [0.16, 1, 0.3, 1] }}
                        >
                            {outlet}
                        </motion.div>
                    </AnimatePresence>
                </div>

                <div className="mt-5 flex items-center justify-center gap-3 text-[12px] text-slate-400">
                    <a href={`${WEBSITE_URL}/terms`} target="_blank" rel="noopener noreferrer" className="hover:text-slate-700 transition-colors">Terms</a>
                    <span className="text-slate-300">·</span>
                    <a href={`${WEBSITE_URL}/privacy`} target="_blank" rel="noopener noreferrer" className="hover:text-slate-700 transition-colors">Privacy</a>
                    <span className="text-slate-300">·</span>
                    <span>© {new Date().getFullYear()} Fullpilot</span>
                </div>
            </div>
        </div>
    );
}
