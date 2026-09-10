import type { SendMethodStats } from "@/lib/api/models/app/analytics/SendMethods";

// A send method is <transport>_<provider>_<deployment>; the deployment may
// itself contain underscores, so only the first two are split off.
function describeSendMethod(method: string, format?: string): { title: string; detail: string } {
    if (!method) return { title: "Not recorded", detail: "sent before methods were recorded" };
    const formatLabel = format === "html" ? " · HTML" : format === "text" ? " · Text" : "";
    const [transport, provider, ...rest] = method.split("_");
    const transportLabel = transport === "api" ? "API" : transport === "smtp" ? "SMTP" : transport;
    const providerLabel =
        provider === "google" ? "Google" : provider === "microsoft" ? "Microsoft" : provider === "smtpimap" ? "SMTP/IMAP" : provider;
    const deployment = rest.join("_") || "unknown";
    return { title: `${transportLabel} · ${providerLabel}${formatLabel}`, detail: deployment };
}

const pct = (v: number | undefined) => (v == null ? "—" : `${v.toFixed(1)}%`);

// One row per send method: sent, then open, reply and bounce rates. Neutral
// styling to sit under the Totals card and the analytics Breakdown column.
export default function SendMethodRows({
    methods,
    loading,
    rowClassName = "h-9 px-5",
}: {
    methods: SendMethodStats[] | undefined;
    loading: boolean;
    rowClassName?: string;
}) {
    if (loading) {
        return (
            <div className="divide-y divide-slate-200/60">
                {Array.from({ length: 2 }).map((_, i) => (
                    <div key={i} className={`${rowClassName} flex items-center gap-3`}>
                        <div className="h-3 w-32 bg-slate-100 rounded animate-pulse" />
                        <div className="ml-auto h-3 w-28 bg-slate-100 rounded animate-pulse" />
                    </div>
                ))}
            </div>
        );
    }
    if (!methods || methods.length === 0) {
        return (
            <div className="px-5 py-4 text-[11.5px] text-slate-400">
                No confirmed sends yet. Each send records whether it went through the provider API or SMTP, from which worker, and whether it carried HTML.
            </div>
        );
    }
    return (
        <div className="divide-y divide-slate-200/60">
            <div className={`${rowClassName} flex items-center gap-2 text-[9.5px] uppercase tracking-[0.14em] text-slate-400`}>
                <span>Method</span>
                <span className="ml-auto grid grid-cols-4 gap-x-3 text-right font-mono tabular-nums">
                    <span title="Confirmed sends">Sent</span>
                    <span title="Opened as a share of sent">Open</span>
                    <span title="Replied as a share of sent">Reply</span>
                    <span title="Bounced as a share of sent">Bounce</span>
                </span>
            </div>
            {methods.map((m) => {
                const d = describeSendMethod(m.send_method, m.send_format);
                return (
                    <div key={`${m.send_method || "unrecorded"}:${m.send_format || ""}`} className={`${rowClassName} flex items-center gap-2`} title={m.send_method || undefined}>
                        <span className="min-w-0 flex items-baseline gap-1.5 truncate">
                            <span className="text-[12px] text-slate-700 truncate">{d.title}</span>
                            <span className="font-mono text-[9.5px] text-slate-400 truncate">{d.detail}</span>
                        </span>
                        <span className="ml-auto grid grid-cols-4 gap-x-3 text-right font-mono text-[11px] text-slate-500 tabular-nums shrink-0">
                            <span>{m.sent.toLocaleString()}</span>
                            <span>{pct(m.open_rate)}</span>
                            <span>{pct(m.reply_rate)}</span>
                            <span>{pct(m.bounce_rate)}</span>
                        </span>
                    </div>
                );
            })}
        </div>
    );
}
