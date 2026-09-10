// Two-step "Insert personalization" dialog behind the braces button of the
// subject field and the body toolbar. Step 1 lists the common fields in
// three groups (recipient, sender, other); recipient rows carry a fallback
// used when the contact's field is blank, remembered per field. Step 2 is a
// custom field by name with the org's real keys as suggestions. Insert calls
// onPick(token) and closes. Portaled and marked data-floating so the email
// editor dialog beneath leaves Escape to this one.

import React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import { ArrowLeftIcon, ChevronRightIcon, XIcon } from "@/components/icons";
import { Label, TextInput } from "@/components/ui/field";
import useCustomFieldKeys from "@/lib/api/hooks/app/contacts/useCustomFieldKeys";
import {
    LINK_VARS,
    SAMPLE,
    SENDER_VARS,
    STANDARD_VARS,
    buildFallbackToken,
    cleanFieldName,
    isStandardKey,
} from "@/lib/templateVars";

const EASE = [0.22, 1, 0.36, 1] as const;

// Last-used fallback per recipient field, so a typed "Hey" sticks.
const STORAGE_KEY = "warmbly.personalization.fallbacks";

function loadFallbacks(): Record<string, string> {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        const parsed = raw ? (JSON.parse(raw) as unknown) : null;
        if (parsed && typeof parsed === "object") {
            return Object.fromEntries(
                Object.entries(parsed as Record<string, unknown>).filter(([, v]) => typeof v === "string"),
            ) as Record<string, string>;
        }
    } catch {
        // Storage unavailable or unreadable: defaults apply.
    }
    return {};
}

function saveFallbacks(next: Record<string, string>) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
    } catch {
        // Nothing to do; the fallback still applies this session.
    }
}

interface RecipientRow {
    id: string;
    label: string;
    desc: string;
    sample: string;
    defaultFallback: string;
    build: (fallback: string) => string;
}

const STD = Object.fromEntries(STANDARD_VARS.map((v) => [v.key, v]));

const RECIPIENT_ROWS: RecipientRow[] = [
    {
        id: "FirstName",
        label: "First name",
        desc: STD.FirstName.desc,
        sample: STD.FirstName.sample,
        defaultFallback: "there",
        build: (fb) => buildFallbackToken("FirstName", fb),
    },
    {
        id: "LastName",
        label: "Last name",
        desc: STD.LastName.desc,
        sample: STD.LastName.sample,
        defaultFallback: "",
        build: (fb) => buildFallbackToken("LastName", fb),
    },
    {
        id: "FullName",
        label: "Full name",
        desc: "First and last name together",
        sample: `${STD.FirstName.sample} ${STD.LastName.sample}`,
        defaultFallback: "there",
        // Two chips: the fallback covers the first name, the last name follows.
        build: (fb) => `${buildFallbackToken("FirstName", fb)} {{.LastName}}`,
    },
    {
        id: "Company",
        label: "Company",
        desc: STD.Company.desc,
        sample: STD.Company.sample,
        defaultFallback: "your company",
        build: (fb) => buildFallbackToken("Company", fb),
    },
    {
        id: "Email",
        label: "Email",
        desc: STD.Email.desc,
        sample: STD.Email.sample,
        defaultFallback: "",
        build: (fb) => buildFallbackToken("Email", fb),
    },
    {
        id: "Phone",
        label: "Phone",
        desc: STD.Phone.desc,
        sample: STD.Phone.sample,
        defaultFallback: "",
        build: (fb) => buildFallbackToken("Phone", fb),
    },
];

export default function PersonalizationDialog({
    open,
    onClose,
    onPick,
    links = [],
}: {
    open: boolean;
    onClose: () => void;
    onPick: (token: string) => void;
    // Per-send link tokens (the recipient's unsubscribe link); body editors only.
    links?: string[];
}) {
    const [step, setStep] = React.useState<"list" | "custom">("list");
    const [fallbacks, setFallbacks] = React.useState<Record<string, string>>({});
    const [custom, setCustom] = React.useState("");
    const [customFallback, setCustomFallback] = React.useState("");

    // Fresh start on every open: the list, with the remembered fallbacks.
    React.useEffect(() => {
        if (!open) return;
        setStep("list");
        setCustom("");
        setCustomFallback("");
        const stored = loadFallbacks();
        setFallbacks(
            Object.fromEntries(RECIPIENT_ROWS.map((r) => [r.id, stored[r.id] ?? r.defaultFallback])),
        );
    }, [open]);

    // Escape closes this dialog only; the confirm owns it while up.
    React.useEffect(() => {
        if (!open) return;
        const onKey = (e: KeyboardEvent) => {
            if (e.key !== "Escape") return;
            if (document.querySelector("[role='alertdialog']")) return;
            e.stopPropagation();
            onClose();
        };
        document.addEventListener("keydown", onKey, true);
        return () => document.removeEventListener("keydown", onKey, true);
    }, [open, onClose]);

    const setFallback = (id: string, value: string) => {
        setFallbacks((prev) => {
            const next = { ...prev, [id]: value };
            saveFallbacks(next);
            return next;
        });
    };

    const insert = (token: string) => {
        if (!token) return;
        onPick(token);
        onClose();
    };

    const { data: customKeys = [] } = useCustomFieldKeys();
    const customName = cleanFieldName(custom);
    const q = customName.toLowerCase();
    const suggestions = customKeys
        .filter((k) => !isStandardKey(k) && (!q || k.toLowerCase().includes(q)))
        .slice(0, 8);
    const shadowsStandard = customName !== "" && isStandardKey(customName);
    const insertCustom = () => {
        if (!customName) return;
        insert(buildFallbackToken(customName, customFallback));
    };

    const linkRows = LINK_VARS.filter((v) => links.includes(v.token));

    if (typeof document === "undefined") return null;

    return createPortal(
        <AnimatePresence>
            {open && (
                <motion.div
                    key="personalization-overlay"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.12 }}
                    onMouseDown={(e) => {
                        e.stopPropagation();
                        onClose();
                    }}
                    onClick={(e) => e.stopPropagation()}
                    className="fixed inset-0 z-[220] flex items-center justify-center bg-slate-900/30 px-4"
                >
                    <motion.div
                        key="personalization-card"
                        role="dialog"
                        aria-modal="true"
                        aria-label={step === "list" ? "Insert personalization" : "Custom field"}
                        data-floating
                        initial={{ y: 8, opacity: 0 }}
                        animate={{ y: 0, opacity: 1 }}
                        exit={{ y: 8, opacity: 0 }}
                        transition={{ duration: 0.16, ease: EASE }}
                        onMouseDown={(e) => e.stopPropagation()}
                        className="flex w-full max-w-[520px] flex-col overflow-hidden border border-slate-200 bg-white shadow-[0_24px_48px_-12px_rgba(15,23,42,0.22)] max-h-[85dvh]"
                    >
                        <div className="flex h-12 shrink-0 items-center gap-2 border-b border-slate-200 px-4">
                            {step === "custom" && (
                                <button
                                    type="button"
                                    onClick={() => setStep("list")}
                                    aria-label="Back"
                                    className="-ml-1.5 inline-flex size-7 items-center justify-center text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900"
                                >
                                    <ArrowLeftIcon className="h-3.5 w-3.5" />
                                </button>
                            )}
                            <span className="text-[13px] font-medium text-slate-900">
                                {step === "list" ? "Insert personalization" : "Custom field"}
                            </span>
                            <button
                                type="button"
                                onClick={onClose}
                                aria-label="Close"
                                className="ml-auto inline-flex size-7 items-center justify-center text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900"
                            >
                                <XIcon className="h-3.5 w-3.5" />
                            </button>
                        </div>

                        <div className="relative min-h-0 flex-1 overflow-y-auto">
                            <AnimatePresence mode="wait" initial={false}>
                                {step === "list" ? (
                                    <motion.div
                                        key="list"
                                        initial={{ x: -16, opacity: 0 }}
                                        animate={{ x: 0, opacity: 1 }}
                                        exit={{ x: -16, opacity: 0 }}
                                        transition={{ duration: 0.14, ease: EASE }}
                                    >
                                        <p className="px-4 pt-3 text-[11.5px] text-slate-500">
                                            Replaced for each recipient when the email sends. Click a field to insert it.
                                        </p>

                                        <Group
                                            title="Recipient"
                                            aside="Fallback if blank"
                                        >
                                            {RECIPIENT_ROWS.map((r) => (
                                                <Row
                                                    key={r.id}
                                                    label={r.label}
                                                    desc={r.desc}
                                                    sample={r.sample}
                                                    onInsert={() => insert(r.build(fallbacks[r.id] ?? ""))}
                                                    end={
                                                        <TextInput
                                                            value={fallbacks[r.id] ?? ""}
                                                            onChange={(v) => setFallback(r.id, v)}
                                                            placeholder="None"
                                                            title="Used when the recipient's field is blank"
                                                            className="w-28"
                                                            onKeyDown={(e) => {
                                                                if (e.key === "Enter") {
                                                                    e.preventDefault();
                                                                    insert(r.build(fallbacks[r.id] ?? ""));
                                                                }
                                                            }}
                                                        />
                                                    }
                                                />
                                            ))}
                                        </Group>

                                        <Group title="Sender">
                                            {SENDER_VARS.map((v) => (
                                                <Row
                                                    key={v.key}
                                                    label={v.label.replace(/^Sender /, "").replace(/^\w/, (c) => c.toUpperCase())}
                                                    desc={v.desc}
                                                    sample={v.sample}
                                                    onInsert={() => insert(v.token)}
                                                />
                                            ))}
                                        </Group>

                                        <Group title="Other">
                                            <Row
                                                label="Custom field"
                                                desc="Any field from your contact list, by name"
                                                onInsert={() => setStep("custom")}
                                                end={<ChevronRightIcon className="h-3.5 w-3.5 text-slate-400" />}
                                                endInsideButton
                                            />
                                            {linkRows.map((v) => (
                                                <Row
                                                    key={v.key}
                                                    label={v.label}
                                                    desc={v.desc}
                                                    sample={SAMPLE[v.key]}
                                                    onInsert={() => insert(v.token)}
                                                />
                                            ))}
                                        </Group>
                                    </motion.div>
                                ) : (
                                    <motion.div
                                        key="custom"
                                        initial={{ x: 16, opacity: 0 }}
                                        animate={{ x: 0, opacity: 1 }}
                                        exit={{ x: 16, opacity: 0 }}
                                        transition={{ duration: 0.14, ease: EASE }}
                                        className="px-4 py-3"
                                    >
                                        <Label>Field name</Label>
                                        <TextInput
                                            autoFocus
                                            value={custom}
                                            onChange={setCustom}
                                            placeholder="e.g. role"
                                            className="w-full"
                                            invalid={shadowsStandard}
                                            onKeyDown={(e) => {
                                                if (e.key === "Enter") {
                                                    e.preventDefault();
                                                    insertCustom();
                                                }
                                            }}
                                        />
                                        {suggestions.length > 0 && (
                                            <div className="mt-1.5 flex flex-wrap gap-1">
                                                {suggestions.map((k) => (
                                                    <button
                                                        key={k}
                                                        type="button"
                                                        onClick={() => setCustom(k)}
                                                        className="inline-flex max-w-full items-center truncate border border-slate-200 bg-slate-50 px-2 py-0.5 text-[11px] text-slate-600 transition-colors hover:border-sky-300 hover:bg-sky-50 hover:text-sky-700"
                                                    >
                                                        {k}
                                                    </button>
                                                ))}
                                            </div>
                                        )}
                                        {shadowsStandard ? (
                                            <p className="mt-1.5 text-[11px] text-amber-600">
                                                A custom field named {customName} is shadowed by the built-in field of the
                                                same name and always uses that value. Pick it from the list instead.
                                            </p>
                                        ) : (
                                            <p className="mt-1.5 text-[11px] text-slate-400">
                                                Exact field name as it appears on your contacts. Blank when a contact lacks
                                                it, unless a fallback is set.
                                            </p>
                                        )}

                                        <div className="mt-3">
                                            <Label>Fallback if blank</Label>
                                            <TextInput
                                                value={customFallback}
                                                onChange={setCustomFallback}
                                                placeholder="Optional"
                                                className="w-full"
                                                onKeyDown={(e) => {
                                                    if (e.key === "Enter") {
                                                        e.preventDefault();
                                                        insertCustom();
                                                    }
                                                }}
                                            />
                                        </div>

                                        <div className="mt-4 flex items-center justify-between gap-2">
                                            <button
                                                type="button"
                                                onClick={() => setStep("list")}
                                                className="inline-flex h-7 items-center gap-1.5 px-2.5 text-[12px] font-medium text-slate-600 transition-colors hover:bg-slate-100 hover:text-slate-900"
                                            >
                                                <ArrowLeftIcon className="h-3.5 w-3.5" />
                                                Back
                                            </button>
                                            <button
                                                type="button"
                                                onClick={insertCustom}
                                                disabled={!customName}
                                                className="h-7 bg-sky-600 px-3 text-[12px] font-medium text-white transition-colors hover:bg-sky-700 disabled:opacity-50"
                                            >
                                                Insert
                                            </button>
                                        </div>
                                    </motion.div>
                                )}
                            </AnimatePresence>
                        </div>
                    </motion.div>
                </motion.div>
            )}
        </AnimatePresence>,
        document.body,
    );
}

function Group({ title, aside, children }: { title: string; aside?: string; children: React.ReactNode }) {
    return (
        <div className="pt-3">
            <div className="flex items-center justify-between px-4 pb-1">
                <span className="text-[10px] font-medium uppercase tracking-[0.14em] text-slate-400">{title}</span>
                {aside && <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400">{aside}</span>}
            </div>
            <div className="border-y border-slate-100">{children}</div>
        </div>
    );
}

// One field: the clickable label/description/sample, plus an optional
// trailing control (a fallback input, or an arrow drawn inside the button).
function Row({
    label,
    desc,
    sample,
    onInsert,
    end,
    endInsideButton = false,
}: {
    label: string;
    desc: string;
    sample?: string;
    onInsert: () => void;
    end?: React.ReactNode;
    endInsideButton?: boolean;
}) {
    return (
        <div className="flex items-center gap-2 border-b border-slate-100 last:border-b-0">
            <button
                type="button"
                onClick={onInsert}
                className="flex min-w-0 flex-1 items-center gap-3 px-4 py-2 text-left transition-colors hover:bg-slate-50"
            >
                <span className="min-w-0 flex-1">
                    <span className="block text-[12.5px] text-slate-800">{label}</span>
                    <span className="block truncate text-[11px] text-slate-400">{desc}</span>
                </span>
                {sample && <span className="max-w-[150px] shrink-0 truncate text-[11.5px] text-slate-400">{sample}</span>}
                {endInsideButton && end}
            </button>
            {!endInsideButton && end && <div className="shrink-0 pr-4">{end}</div>}
        </div>
    );
}
