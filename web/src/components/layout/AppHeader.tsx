// Top breadcrumb bar.
//
// Reads as one continuous line across the entire top of the shell:
//
//   [Fullpilot wordmark]  >  [Org picker]  >  [Current section]      [⌘K  ⚡]
//
// The wordmark sits over the sidebar column, the org picker + section live
// in the open area, the right side has presence, notifications + search.
// All on the sky-colored chrome — text is white-ish, dividers are faint.
//
// This component is purely the row. Layout (where it sits) is decided by
// AppShell, not here.

import { Link, useLocation } from "react-router-dom";
import { ChevronRight, MenuIcon, Search } from "@/components/icons";
import { Logo, Wordmark } from "@/components/svg";
import AgentMark from "@/components/app/agent/AgentMark";
import { useAppStore } from "@/stores";
import { usePermission } from "@/hooks/usePermission";
import ShortcutTooltip from "@/components/ui/shortcut-tooltip";
import PresenceAvatars from "@/components/app/presence/PresenceAvatars";
import { NotificationBell } from "./NotificationBell";
import { OrgSwitcher } from "./OrgSwitcher";
import { PlanPill } from "./PlanPill";
import { CreditsMeter } from "./CreditsMeter";

// Pretty labels for path segments. Anything missing falls back to the
// raw segment with its first letter capitalised.
const labelMap: Record<string, string> = {
    app: "Home",
    emails: "Accounts",
    unibox: "Inbox",
    contacts: "Contacts",
    segments: "Segments",
    categories: "Categories",
    campaigns: "Campaigns",
    analytics: "Analytics",
    crm: "CRM",
    pipelines: "Pipelines",
    deals: "Deals",
    tasks: "Tasks",
    templates: "Templates",
    "api-keys": "API Keys",
    settings: "Settings",
    billing: "Billing",
    team: "Team",
    admin: "Admin",
    workers: "Workers",
    credentials: "Credentials",
    audit: "Audit",
    leads: "Leads",
    preferences: "Preferences",
    schedule: "Schedule",
    steps: "Steps",
};

function pretty(segment: string): string {
    return labelMap[segment] ?? segment.charAt(0).toUpperCase() + segment.slice(1);
}

export function AppHeader({ onMenu }: { onMenu?: () => void }) {
    const { pathname } = useLocation();
    const setCommandPaletteOpen = useAppStore((s) => s.setCommandPaletteOpen);

    // Path under /app — first segment is the section ("emails", "admin", ...),
    // subsequent ones are subpages. Don't show UUID-looking segments verbatim
    // because nobody wants "Campaigns > 47a3-..." in their chrome.
    const segments = pathname
        .split("/")
        .filter(Boolean)
        .filter((s) => s !== "app");
    // Each crumb links to its own path prefix so "Campaigns > Leads" gets you
    // back to the list; hidden UUID segments still count toward the prefix.
    const crumbs = segments
        .map((seg, i) => ({ seg, to: `/app/${segments.slice(0, i + 1).join("/")}` }))
        .filter(({ seg }) => !/^[0-9a-f]{8}-[0-9a-f]{4}/i.test(seg));
    // A crumb whose prefix is the page itself is a label; every other one is a
    // link (so "Campaigns" stays clickable on /campaigns/<id>, where the hidden
    // id is the real last segment).
    const currentPath = `/app/${segments.join("/")}`;

    return (
        <div className="h-14 flex items-center shrink-0">
            {/* Logo zone — sidebar-width on >=md, compact with a menu button
                on mobile (the sidebar collapses into a drawer below md). */}
            <button
                type="button"
                onClick={onMenu}
                aria-label="Open menu"
                className="md:hidden ml-1.5 w-9 h-9 rounded-md flex items-center justify-center text-slate-600 hover:text-slate-900 hover:bg-slate-200/60 transition-colors shrink-0"
            >
                <MenuIcon className="w-5 h-5" />
            </button>
            <Link
                to="/app/emails"
                className="h-full flex items-center gap-2.5 shrink-0 group pl-2 pr-3 md:w-64 md:px-5"
            >
                {/* The word hides on mobile: the mark + the drawer's own brand
                    header carry it there, leaving room for the workspace pill. */}
                <Logo className="w-6 text-[#0c58c6] group-hover:text-sky-700 transition-colors duration-150 md:hidden" />
                <Wordmark size={20} className="hidden md:inline-flex group-hover:opacity-80 transition-opacity duration-150" />
            </Link>

            {/* Breadcrumb: org switcher (always) > section > subpages. The
                section crumbs are redundant with each page's own title on a
                phone, so they only show on >=md. */}
            <div className="flex items-center gap-1.5 min-w-0 flex-1 pr-2 md:pr-4">
                {crumbs.map(({ seg, to }, i) => (
                    <div key={to} className="hidden md:flex items-center gap-2 min-w-0">
                        {i > 0 && <ChevronRight className="w-3.5 h-3.5 text-slate-300 shrink-0" />}
                        {to === currentPath ? (
                            <span className="text-[13px] font-medium text-slate-900 truncate">
                                {pretty(seg)}
                            </span>
                        ) : (
                            <Link
                                to={to}
                                className="text-[13px] text-slate-500 hover:text-slate-900 truncate transition-colors"
                            >
                                {pretty(seg)}
                            </Link>
                        )}
                    </div>
                ))}
            </div>

            <div className="flex items-center gap-2 px-2 sm:px-4 shrink-0">
                <div className="hidden sm:flex items-center gap-2">
                    <PlanPill />
                    <CreditsMeter />
                    <div className="h-4 w-px bg-slate-200/80" />
                </div>
                {/* Workspace switcher lives at the right edge, next to the
                    account controls, rather than heading the breadcrumb. */}
                <OrgSwitcher />
                <div className="h-4 w-px bg-slate-200/80 hidden sm:block" />
                <PresenceAvatars />
                <NotificationBell />
                <AssistantButton />
                <button
                    onClick={() => setCommandPaletteOpen(true)}
                    className="flex items-center gap-2 px-2 h-7 rounded-md text-slate-500 hover:text-slate-900 hover:bg-slate-200/60 transition-colors text-[12.5px]"
                >
                    <Search className="w-3.5 h-3.5" />
                    <span className="hidden sm:inline">Search</span>
                    <kbd className="hidden md:inline-flex h-4 items-center px-1 rounded border border-slate-300/70 bg-white/60 font-mono text-[10px] text-slate-500 ml-0.5">
                        ⌘K
                    </kbd>
                </button>
            </div>
        </div>
    );
}


// The assistant toggle, with a live status badge so background work is never
// invisible: pulsing sky while a run streams, amber when a tool waits for
// approval, solid sky when a finished response hasn't been read yet.
function AssistantButton() {
    const open = useAppStore((s) => s.aiAssistantOpen);
    const minimized = useAppStore((s) => s.agentMinimized);
    const setOpen = useAppStore((s) => s.setAIAssistantOpen);
    const setMinimized = useAppStore((s) => s.setAgentMinimized);
    const tabs = useAppStore((s) => s.agentTabs);
    const canAI = usePermission("USE_AI");

    if (!canAI) return null;

    const running = tabs.some((t) => t.running);
    const pending = tabs.some((t) => t.pending);
    const unseen = tabs.some((t) => t.unseen);

    return (
        <ShortcutTooltip label="AI assistant" combo="mod+I" side="bottom">
        <button
            onClick={() => {
                if (open && minimized) {
                    // Docked: bring the panel back instead of closing it.
                    setMinimized(false);
                } else if (open) {
                    setOpen(false);
                } else {
                    setMinimized(false);
                    setOpen(true);
                }
            }}
            aria-label="AI assistant"
            className="relative flex items-center justify-center size-7 rounded-md text-slate-500 hover:text-sky-700 hover:bg-sky-50 transition-colors"
        >
            <AgentMark className="w-4 h-4" />
            {(running || pending || unseen) && (
                <span
                    className={
                        "absolute top-0.5 right-0.5 size-1.5 rounded-full ring-2 ring-white " +
                        (running
                            ? "bg-sky-500 animate-pulse"
                            : pending
                              ? "bg-amber-500"
                              : "bg-sky-500")
                    }
                />
            )}
        </button>
        </ShortcutTooltip>
    );
}
