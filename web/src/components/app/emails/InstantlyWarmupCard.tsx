// The Instantly block in a mailbox's Warmup tab. When Instantly warms the
// mailbox it shows what Instantly reports (status, health, inbox vs spam, the
// settings applied); when the integration is on but the mailbox is not warmed
// there yet, it says what a start will do.

import { ExternalLinkIcon, ZapIcon } from "@/components/icons";
import type Inbox from "@/lib/api/models/app/emails/Inbox";
import useInstantlyWarmup from "@/lib/api/hooks/app/emails/useInstantlyWarmup";
import { Loading } from "@/components/loader";
import { cn } from "@/lib/utils";

const Eyebrow = ({ children }: { children: React.ReactNode }) => (
    <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{children}</div>
);

function Metric({ label, value, tone }: { label: string; value: string | number; tone?: string }) {
    return (
        <div className="rounded-md border border-slate-200 px-3 py-2">
            <div className="text-[10px] uppercase tracking-[0.12em] text-slate-400 font-medium">{label}</div>
            <div className={cn("mt-0.5 text-[15px] font-semibold tabular-nums text-slate-900", tone)}>{value}</div>
        </div>
    );
}

const pct = (v: number | undefined) => (v === undefined ? "" : `${Math.round(v * 100)}%`);

export default function InstantlyWarmupCard({ mailbox }: { mailbox: Inbox }) {
    const enrolled = mailbox.warmup_provider === "instantly";
    const q = useInstantlyWarmup(mailbox.id);
    const data = q.data;

    if (q.isPending) {
        return enrolled ? (
            <div className="px-5 py-4 flex items-center gap-2 text-[12px] text-slate-400">
                <Loading className="!w-3.5 h-3.5" /> Reading Instantly
            </div>
        ) : null;
    }
    if (!data?.configured) return null;

    if (!enrolled) {
        return (
            <div className="px-5 py-3 flex items-center gap-2.5 text-[12px] text-slate-600">
                <ZapIcon className="w-3.5 h-3.5 text-sky-600 shrink-0" />
                <span className="min-w-0 flex-1">
                    Warmup on this workspace runs through Instantly. Starting it enrolls this mailbox there with Fullpilot's
                    warmup settings and turns the built-in pool off for it.
                    {data.found ? "" : mailbox.provider === "smtp_imap" ? " It will be added with its stored SMTP and IMAP details." : " Add it in Instantly first."}
                </span>
                <a href={data.dashboard_url} target="_blank" rel="noreferrer" className="shrink-0 font-medium text-sky-700 hover:text-sky-900 inline-flex items-center gap-1">
                    Instantly <ExternalLinkIcon className="w-3 h-3" />
                </a>
            </div>
        );
    }

    const st = data.status;
    const a = st?.analytics;
    const s = st?.settings;
    const label = !data.found ? "not in Instantly" : st?.setup_pending ? "setup pending" : st?.warmup_label ?? "unknown";
    const healthy = !!st?.active && st.account_status === 1;

    return (
        <div className="px-5 py-4 space-y-3">
            <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2.5 min-w-0">
                    <span className="size-8 rounded-lg text-white inline-flex items-center justify-center shrink-0">
                        <ZapIcon className="w-4 h-4" />
                    </span>
                    <div className="min-w-0">
                        <div className="text-[12.5px] font-medium text-slate-900">Warming via Instantly</div>
                        <div className="text-[11px] text-slate-500 truncate">
                            Warmup <span className={cn("font-medium", healthy ? "text-emerald-600" : "text-amber-600")}>{label}</span>
                            {st && st.account_status !== 1 ? ` · account ${st.account_label}` : ""}
                            {st?.started_at ? ` · since ${new Date(st.started_at).toLocaleDateString()}` : ""}
                        </div>
                    </div>
                </div>
                <a href={data.dashboard_url} target="_blank" rel="noreferrer" className="shrink-0 text-[11.5px] font-medium text-sky-700 hover:text-sky-900 inline-flex items-center gap-1">
                    Open in Instantly <ExternalLinkIcon className="w-3 h-3" />
                </a>
            </div>

            {!data.found && (
                <p className="text-[11.5px] text-amber-700 bg-amber-50 border border-amber-100 rounded-md px-3 py-2 leading-relaxed">
                    Instantly no longer lists this mailbox, so nothing is warming it. Add it back in Instantly, or stop and start
                    warmup here to hand it to the built-in pool.
                </p>
            )}

            {(a || st?.warmup_score != null) && (
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                    <Metric label="Health" value={a ? a.health_score_label || `${a.health_score}%` : `${st?.warmup_score ?? ""}`} tone={a && a.health_score < 80 ? "text-amber-600" : "text-emerald-600"} />
                    <Metric label="Sent" value={a?.sent ?? 0} />
                    <Metric label="Inbox" value={a?.landed_inbox ?? 0} />
                    <Metric label="Spam" value={a?.landed_spam ?? 0} tone={a && a.landed_spam > 0 ? "text-rose-600" : undefined} />
                </div>
            )}

            {s && (
                <div>
                    <Eyebrow>Instantly settings</Eyebrow>
                    <div className="mt-1.5 text-[11.5px] text-slate-600 leading-relaxed">
                        Up to <b className="text-slate-900">{s.limit}</b> warmup emails a day
                        {s.increment !== "disabled" ? <>, ramping by <b className="text-slate-900">{s.increment}</b> a day</> : null}
                        , <b className="text-slate-900">{pct(s.reply_rate)}</b> replies
                        {s.advanced ? (
                            <>
                                , {pct(s.advanced.open_rate)} opens, {pct(s.advanced.important_rate)} marked important,
                                {" "}{pct(s.advanced.spam_save_rate)} rescued from spam, read emulation {s.advanced.read_emulation ? "on" : "off"},
                                {" "}{s.advanced.weekday_only ? "weekdays only" : "every day"}
                            </>
                        ) : null}
                        . Managed by Fullpilot; the ramp fields below do not apply while Instantly warms this mailbox.
                    </div>
                </div>
            )}
        </div>
    );
}
