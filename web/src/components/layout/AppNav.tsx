// Sidebar for the sky-chrome shell.
//
// Brae-density structure: small tracked-uppercase section labels, h-8
// nav rows, hairline dividers between sections. The header slot is
// the LivePanel — a small ambient telemetry card that replaces the
// generic "+ New Campaign" pill. Cold-email work is always-on; the
// sidebar should reflect that rather than nag with a CTA.

import { Link, useLocation } from "react-router-dom";
import {
    ClipboardListIcon,
    BarChart3Icon,
    CableIcon,
    CalendarClockIcon,
    CheckSquareIcon,
    CircleDollarSignIcon,
    FileTextIcon,
    GitBranchIcon,
    InboxIcon,
    KeyIcon,
    ListChecksIcon,
    type LucideIcon,
    MailIcon,
    MegaphoneIcon,
    SettingsIcon,
    ShieldCheckIcon,
    UsersIcon,
    LockIcon,
    XIcon,
    ZapIcon,
} from "@/components/icons";
import { useMemo, useState } from "react";
import { useAppStore } from "@/stores";
import useFeatureAccess from "@/hooks/useFeatureAccess";
import { usePermission, type PermissionKey } from "@/hooks/usePermission";
import { useUpgradeDialog } from "@/hooks/context/upgrade";
import { PLAN_ACCENT_CLASSES, getPlan, type PlanID } from "@/lib/plans";
import AccessLockedDialog from "./AccessLockedDialog";
import useDashboard from "@/lib/api/hooks/app/analytics/useDashboard";
import AdvisorNavBadge from "@/components/app/advisor/AdvisorNavBadge";
import type { AdvisorSurface } from "@/lib/api/models/app/advisor/Advisor";
import { UserNav } from "./UserNav";
import { Wordmark } from "@/components/svg";
import { cn } from "@/lib/utils";

// Stable (module-level) empty contacts search so the sidebar's contact-count
// query key never changes identity between renders (which would refetch-loop).
// limit 1 keeps the payload tiny — we only read pagination.total.

interface NavItem {
    title: string;
    url: string;
    icon: LucideIcon;
    badgeStoreKey?: "unseenCount";
    /** Feature gate key — when set, sidebar dims the row and shows a plan badge. */
    requires?: "inbox" | "advanced" | "subscription";
    /** Role gate — when set, sidebar hides the row entirely for non-matching roles. */
    rolesAllowed?: "manage";
    /** Permission gate — when the member lacks it, the row shows a lock and a
     *  click pops an access dialog instead of navigating to an empty page. */
    permission?: PermissionKey;
    /** Friendly label of the permission, for the access dialog. */
    permissionLabel?: string;
    /** Advisor surface — when set, the row badges the count of critical/high
     *  recommendations the Advisor has open for that area, so a problem is
     *  visible from the sidebar on the tab where its fix lives. */
    advisorSurface?: AdvisorSurface;
    /** Live indicator key — renders an ambient, realtime activity cluster.
     *  Each key has its OWN motif (campaigns = dot-grid, accounts = flame,
     *  tasks = red attention dot) so the rows stay visually distinct rather
     *  than a column of identical loaders. */
    indicator?:
        | "campaigns"
        | "accounts"
        | "tasks"
        | "contacts"
        | "deals"
        | "pipelines"
        | "meetings"
        | "templates"
        | "analytics"
        | "apikeys"
        | "integrations";
}

// Minimum plan behind each sidebar gate. The badge label and colour come
// from lib/plans so the marketing site, header pill, sidebar badge and the
// upgrade dialog all agree on "what does Starter look like".
//
//   inbox / subscription → Starter (any paid tier)
//   advanced             → Business (15k/day + isolated sending tier)
const REQUIRES_TO_MIN_PLAN: Record<NonNullable<NavItem["requires"]>, PlanID> = {
    inbox: "starter",
    subscription: "starter",
    advanced: "business",
};

interface NavSection {
    label: string;
    items: NavItem[];
}

const topItems: NavItem[] = [
    {
        title: "Inbox",
        url: "/app/unibox",
        icon: InboxIcon,
        badgeStoreKey: "unseenCount",
        requires: "inbox",
        permission: "ACCESS_UNIBOX",
        permissionLabel: "Use unified inbox",
    },
];

const sections: NavSection[] = [
    {
        label: "Email",
        items: [
            { title: "Accounts", url: "/app/emails", icon: MailIcon, indicator: "accounts", advisorSurface: "emails", permission: "MANAGE_EMAILS", permissionLabel: "Manage mailboxes" },
            { title: "Campaigns", requires: "subscription", url: "/app/campaigns", icon: MegaphoneIcon, indicator: "campaigns", advisorSurface: "campaigns", permission: "VIEW_CAMPAIGNS", permissionLabel: "View campaigns" },
            { title: "Contacts", requires: "subscription", url: "/app/contacts", icon: UsersIcon, indicator: "contacts", advisorSurface: "contacts", permission: "VIEW_CONTACTS", permissionLabel: "View contacts" },
            { title: "Forms", requires: "subscription", url: "/app/forms", icon: ClipboardListIcon, permission: "VIEW_CONTACTS", permissionLabel: "View contacts" },
            { title: "Analytics", requires: "subscription", url: "/app/analytics", icon: BarChart3Icon, indicator: "analytics", permission: "VIEW_ANALYTICS", permissionLabel: "View analytics" },
            { title: "Deliverability", requires: "subscription", url: "/app/deliverability", icon: ShieldCheckIcon, advisorSurface: "deliverability", permission: "VIEW_ANALYTICS", permissionLabel: "View analytics" },
        ],
    },
    {
        label: "CRM",
        items: [
            { title: "Pipelines", requires: "subscription", url: "/app/crm/pipelines", icon: GitBranchIcon, indicator: "pipelines", permission: "VIEW_CONTACTS", permissionLabel: "View contacts" },
            { title: "Deals", requires: "subscription", url: "/app/crm/deals", icon: CircleDollarSignIcon, indicator: "deals", permission: "VIEW_CONTACTS", permissionLabel: "View contacts" },
            { title: "Tasks", requires: "subscription", url: "/app/crm/tasks", icon: CheckSquareIcon, indicator: "tasks", permission: "VIEW_CONTACTS", permissionLabel: "View contacts" },
            { title: "Meetings", requires: "subscription", url: "/app/crm/meetings", icon: CalendarClockIcon, indicator: "meetings", permission: "VIEW_CONTACTS", permissionLabel: "View contacts" },
        ],
    },
    {
        label: "Resources",
        items: [
            { title: "Templates", requires: "subscription", url: "/app/templates", icon: FileTextIcon, indicator: "templates" },
            { title: "Integrations", requires: "subscription", url: "/app/integrations", icon: CableIcon, indicator: "integrations", permission: "USE_INTEGRATIONS", permissionLabel: "Use integrations" },
            { title: "Automations", requires: "subscription", url: "/app/automations", icon: ZapIcon, permission: "USE_INTEGRATIONS", permissionLabel: "Use integrations" },
            { title: "API Keys", requires: "subscription", url: "/app/api-keys", icon: KeyIcon, indicator: "apikeys", permission: "MANAGE_API_KEYS", permissionLabel: "Manage API keys" },
            { title: "Audit log", requires: "subscription", url: "/app/audit", icon: ListChecksIcon, rolesAllowed: "manage" },
        ],
    },
];

function NavRow({ item }: { item: NavItem }) {
    const { pathname } = useLocation();
    const unseen = useAppStore((s) => s.unseenCount);
    const access = useFeatureAccess();
    const hasItemPermission = usePermission(item.permission ?? "VIEW_CAMPAIGNS");
    const [deniedOpen, setDeniedOpen] = useState(false);
    const upgradeDialog = useUpgradeDialog();
    const active =
        pathname === item.url || pathname.startsWith(item.url + "/");
    const badge = item.badgeStoreKey === "unseenCount" ? unseen : undefined;

    // Role-gated items disappear from the sidebar for users that
    // can't access them, instead of showing a lock — these are
    // administrative tools, not premium features to tease.
    if (item.rolesAllowed === "manage" && !access.canManage) return null;

    // Permission-gated items the member lacks: render a locked row that pops
    // an access dialog on click, so the feature is visibly unavailable (a
    // lock) rather than a blank/empty page that reads as "no data".
    const accessDenied = !!item.permission && !hasItemPermission;
    if (accessDenied) {
        return (
            <>
                <button
                    type="button"
                    onClick={() => setDeniedOpen(true)}
                    title={`${item.title} · no access`}
                    className="group w-[calc(100%-1rem)] mx-2 flex items-center gap-2.5 px-2.5 h-7 rounded-md text-[12.5px] text-slate-400 hover:text-slate-600 hover:bg-slate-200/40 transition-colors duration-100"
                >
                    <LockIcon className="w-[13px] h-[13px] shrink-0 text-slate-300 group-hover:text-slate-500" strokeWidth={1.8} />
                    <span className="truncate flex-1 min-w-0 text-left">{item.title}</span>
                </button>
                <AccessLockedDialog
                    open={deniedOpen}
                    onClose={() => setDeniedOpen(false)}
                    feature={item.title}
                    permissionLabel={item.permissionLabel ?? "the required"}
                />
            </>
        );
    }

    // Subscription-gated items stay visible but dim with a lock so
    // the user knows the feature exists.
    const locked =
        (item.requires === "inbox" && !access.hasInbox) ||
        (item.requires === "advanced" && !access.hasAdvanced) ||
        (item.requires === "subscription" && access.locked);

    const minPlan = locked && item.requires ? REQUIRES_TO_MIN_PLAN[item.requires] : null;
    const planBadge = minPlan
        ? { label: getPlan(minPlan).label, classes: PLAN_ACCENT_CLASSES[getPlan(minPlan).accent].pill }
        : null;

    // Plan-gated items the org's plan doesn't include: lock the row and open
    // the full-screen upgrade dialog on click (plans, interval, one-click
    // checkout), instead of routing to a teasing empty page.
    if (locked && minPlan && planBadge) {
        return (
            <button
                type="button"
                onClick={() => upgradeDialog.open({ feature: item.title, minPlan })}
                title={`${item.title} · ${planBadge.label} plan`}
                className="group w-[calc(100%-1rem)] mx-2 flex items-center gap-2.5 px-2.5 h-7 rounded-md text-[12.5px] text-slate-400 hover:text-slate-700 hover:bg-slate-200/40 transition-colors duration-100"
            >
                <LockIcon className="w-[13px] h-[13px] shrink-0 text-slate-300 group-hover:text-slate-500" strokeWidth={1.8} />
                <span className="truncate flex-1 min-w-0 text-left">{item.title}</span>
                <span
                    className={cn(
                        "h-4 px-1.5 rounded text-[9.5px] font-semibold uppercase tracking-[0.06em] border inline-flex items-center",
                        planBadge.classes,
                    )}
                >
                    {planBadge.label}
                </span>
            </button>
        );
    }

    return (
        <Link
            to={item.url}
            title={planBadge ? `${item.title} · ${planBadge.label} plan` : undefined}
            className={cn(
                "group mx-2 flex items-center gap-2.5 px-2.5 h-7 rounded-md text-[12.5px] transition-colors duration-100",
                active
                    ? "bg-slate-200/70 text-slate-900 font-medium"
                    : locked
                        ? "text-slate-400 hover:text-slate-700 hover:bg-slate-200/40"
                        : "text-slate-600 hover:text-slate-900 hover:bg-slate-200/40",
            )}
        >
            <item.icon
                className={cn(
                    "w-[14px] h-[14px] shrink-0 transition-colors",
                    active
                        ? "text-slate-900 [&_*]:[fill:currentColor] [&_*]:[fill-opacity:0.22]"
                        : locked
                            ? "text-slate-300 group-hover:text-slate-500"
                            : "text-slate-400 group-hover:text-slate-600",
                )}
                strokeWidth={1.6}
            />
            {/* min-w-0 lets the label shrink/truncate so the count cluster (and its
                separator) is never pushed off the row — longer labels like
                "Campaigns"/"Accounts" used to clip it at narrower widths. */}
            <span className="truncate flex-1 min-w-0">{item.title}</span>
            {item.advisorSurface && !locked && <AdvisorNavBadge surface={item.advisorSurface} />}
            {planBadge ? (
                <span
                    className={cn(
                        "h-4 px-1.5 rounded text-[9.5px] font-semibold uppercase tracking-[0.06em] border inline-flex items-center",
                        planBadge.classes,
                    )}
                >
                    {planBadge.label}
                </span>
            ) : (
                badge != null && badge > 0 && (
                    <span className="text-[10px] font-medium bg-red-500 text-white rounded-full min-w-[16px] h-4 flex items-center justify-center px-1 tabular-nums">
                        {badge > 99 ? "99+" : badge}
                    </span>
                )
            )}
        </Link>
    );
}


// The "how many" total at the end of a nav row. Light slate so it reads as
// ambient metadata (lifting a touch on row hover), but visible, and it tweens
// (AnimatedNumber) on change. An optional `glyph` — a small coloured activity
// motif (sending dot-grid, warming flame, overdue ping) — sits in front to flag
// a live state without stealing the number, which stays the plain total. Hidden
// only when there's truly nothing to show.














function Section({ section, first = false }: { section: NavSection; first?: boolean }) {
    return (
        <div className={first ? "" : "mt-5"}>
            <div className="px-4 mb-1.5">
                <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">
                    {section.label}
                </span>
            </div>
            <div className="space-y-px">
                {section.items.map((it) => (
                    <NavRow key={it.url} item={it} />
                ))}
            </div>
        </div>
    );
}

/**
 * LivePanel — the sidebar's footer stat, beneath the navigation tabs.
 *
 * Anatomy:
 *
 *   ┌──────────────────────────────────┐
 *   │  128 of 400 sent today           │   ← small number (scrubs on hover)
 *   │  ━━━━━━━─────────                │   ← capacity meter (today vs cap)
 *   │      ∿∿∿∿∿∿                     │   ← 14-day area sparkline
 *   │  ✉ 8   ● 5              ⬇ 3     │   ← mailboxes · active · unread
 *   └──────────────────────────────────┘
 *
 * Reads as ambient telemetry: even when idle, it tells you "n mailboxes,
 * n sent today." Clicking jumps to analytics; hovering a day on the
 * sparkline swaps the number to that day. There is deliberately no
 * LIVE/OFFLINE status row: the numbers ticking realtime already say the
 * system is up, so the panel spends its pixels on the data instead.
 *
 * Data sources at this layer:
 *   - useAppStore.emails  → mailbox count, active count
 *   - useDashboard("30d") daily_trend → today's sent volume + the sparkline
 *     (shares the dashboard page's query cache; realtime invalidation keeps
 *     it current)
 *
 * The capacity denominator sums each mailbox's configured campaign_limit
 * (default 50/day, from internal/config/constants.go).
 */
function LivePanel() {
    const emails = useAppStore((s) => s.emails);
    const dash = useDashboard("30d");

    const capacity = useMemo(
        () => emails.reduce((sum, e) => sum + (e.campaign_limit ?? 50), 0),
        [emails],
    );
    const sentToday = useMemo(() => {
        const key = new Date().toISOString().slice(0, 10);
        const today = (dash.data?.daily_trend ?? []).find((d) => d.date?.slice(0, 10) === key);
        return today?.sent ?? 0;
    }, [dash.data]);
    const pct = capacity > 0 ? Math.min(100, (sentToday / capacity) * 100) : 0;

    // One line and a hairline meter under the profile: a glance at today's
    // usage, nothing more. Clicking opens analytics.
    return (
        <Link
            to="/app/analytics"
            className="group block px-4 pt-2 pb-2.5 shrink-0 hover:bg-slate-50/80 transition-colors"
            title="Open analytics"
        >
            <div className="flex items-baseline justify-between gap-2 text-[10.5px] leading-none">
                <span className="text-slate-500">Sent today</span>
                <span className="tabular-nums text-slate-700">
                    <span className="font-semibold text-slate-800">{sentToday.toLocaleString()}</span>
                    {capacity > 0 && <span className="text-slate-400"> / {capacity.toLocaleString()}</span>}
                </span>
            </div>
            <div className="mt-1.5 h-[3px] w-full rounded-full bg-slate-200/80 overflow-hidden">
                <div
                    className="h-full rounded-full bg-sky-600 transition-[width] duration-300"
                    style={{ width: `${pct}%` }}
                />
            </div>
        </Link>
    );
}

/** "2026-08-30" → "Aug 30" for the sparkline scrub readout. */

// Sparkline geometry. Width matches the card's inner width (sidebar w-64
// minus mx-2 and borders) so preserveAspectRatio="none" barely distorts
// the dots; side padding keeps markers clear of the overflow-hidden edges.

/**
 * Sparkline — the last two weeks of send volume as a smooth area line
 * (Catmull-Rom smoothing, gradient wash under the stroke, end-of-series
 * dot with a surface ring). Full-bleed across the card; the chips row's
 * top border underneath doubles as the baseline. Invisible per-day hit
 * columns report the hovered day via onHover so the hero number above
 * scrubs with the cursor.
 */

export function AppNav({ open = false, onClose }: { open?: boolean; onClose?: () => void }) {
    return (
        <>
            {/* Mobile-only scrim. Tapping it closes the drawer. */}
            <div
                aria-hidden
                onClick={onClose}
                className={cn(
                    "fixed inset-0 z-40 bg-slate-900/40 transition-opacity duration-300 md:hidden",
                    open ? "opacity-100" : "pointer-events-none opacity-0",
                )}
            />

            <aside
                className={cn(
                    // Mobile: off-canvas drawer that slides in from the left.
                    "fixed inset-y-0 left-0 z-50 w-64 flex flex-col text-slate-900 bg-white shadow-2xl transition-transform duration-300 ease-out",
                    open ? "translate-x-0" : "-translate-x-full",
                    // >=md: static sidebar column over the chrome, no transform/shadow.
                    "md:static md:z-auto md:translate-x-0 md:bg-transparent md:shadow-none md:transition-none shrink-0",
                )}
            >
                {/* Mobile drawer header: brand + close. (The desktop sidebar
                    has no chrome of its own — the brand lives in AppHeader.) */}
                <div className="md:hidden flex items-center justify-between px-3 h-14 border-b border-slate-200/70">
                    <Link to="/app/emails" onClick={onClose} className="flex items-center">
                        <Wordmark size={20} />
                    </Link>
                    <button
                        type="button"
                        onClick={onClose}
                        aria-label="Close menu"
                        className="w-8 h-8 -mr-1 rounded-md flex items-center justify-center text-slate-500 hover:text-slate-900 hover:bg-slate-100 transition-colors"
                    >
                        <XIcon className="w-4 h-4" />
                    </button>
                </div>

            <nav className="flex-1 overflow-y-auto pt-2 pb-3">
                <div className="space-y-px">
                    {topItems.map((it) => (
                        <NavRow key={it.url + it.title} item={it} />
                    ))}
                </div>
                {sections.map((s, i) => (
                    <Section key={s.label} section={s} first={i === 0 && topItems.length === 0} />
                ))}
            </nav>

            <div className="border-t border-slate-200/60 py-1 shrink-0">
                <NavRow
                    item={{ title: "Settings", url: "/app/settings", icon: SettingsIcon }}
                />
            </div>

            <div className="border-t border-slate-200/60 shrink-0">
                <UserNav />
                <LivePanel />
            </div>
            </aside>
        </>
    );
}
