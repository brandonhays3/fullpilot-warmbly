// The step's email, edited in a large two-column dialog: the full composer on
// the left (arm tabs, subject, body, attachments) and, on the right, the email
// as the recipient will see it, re-rendered as you type. Every change
// autosaves to the arm it belongs to, so closing never loses work; "Done" just
// closes. The footer mails a test of the saved step through any active mailbox
// in the workspace, not only the campaign's own senders.

import React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import toast from "react-hot-toast";
import {
    AlertCircleIcon,
    EyeIcon,
    Loader2Icon,
    MailIcon,
    PaperclipIcon,
    PencilLineIcon,
    PlusIcon,
    SendIcon,
    XIcon,
    TrashIcon,
    EyeOffIcon,
} from "@/components/icons";
import type Sequence from "@/lib/api/models/app/campaigns/sequences/Sequence";
import type Contact from "@/lib/api/models/app/contacts/Contact";
import type Inbox from "@/lib/api/models/app/emails/Inbox";
import type { TemplatePreview } from "@/lib/api/client/app/campaigns/previewTemplate";
import { useTemplatePreview } from "@/lib/api/hooks/app/campaigns/useTemplatePreview";
import { useCampaignAttachments } from "@/lib/api/hooks/app/campaigns/useCampaignAttachments";
import useUpdateSequence from "@/lib/api/hooks/app/campaigns/sequences/useUpdateSequence";
import useSendTestEmail from "@/lib/api/hooks/app/campaigns/useSendTestEmail";
import useEmails from "@/lib/api/hooks/app/emails/useEmails";
import { useUserProfile } from "@/hooks/context/user";
import { useConfirm } from "@/hooks/context/confirm";
import { TextInput } from "@/components/ui/field";
import {
    PopoverMenu,
    PopoverMenuContent,
    PopoverMenuItem,
    PopoverMenuLabel,
    PopoverMenuTrigger,
    SelectButton,
} from "@/components/ui/popover-menu";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import formatBytes from "@/lib/helper/formatBytes";
import { SAMPLE } from "@/lib/templateVars";
import EmailContentEditor from "./EmailContentEditor";
import StepAttachments from "./StepAttachments";
import { PreviewContactPicker } from "./PreviewControls";
import { SAMPLE_CONTACT_LABEL, contactLabel, useCampaignSenderInboxes } from "./previewContext";
import { htmlToPlain, linkifyUnsubscribe, renderPreview } from "./emailPreview";
import { ORIGINAL_ARM, type StepArms } from "./useStepArms";
import { useUpdateABVariant } from "@/lib/api/hooks/app/campaigns/useCampaignABVariants";

type Draft = { subject: string; bodyHtml: string };
const sameDraft = (a: Draft, b: Draft) => a.subject === b.subject && a.bodyHtml === b.bodyHtml;
const AUTOSAVE_MS = 800;
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function EditEmailDialog({
    open,
    onClose,
    campaignId,
    sequence,
    index,
    arms,
    armKey,
    onArmChange,
}: {
    open: boolean;
    onClose: () => void;
    campaignId: string;
    sequence: Sequence;
    index: number;
    arms: StepArms;
    // "original" or a variant id; owned by the step pane so both agree.
    armKey: string;
    onArmChange: (key: string) => void;
}) {
    return (
        <AnimatePresence>
            {open && (
                <DialogBody
                    onClose={onClose}
                    campaignId={campaignId}
                    sequence={sequence}
                    index={index}
                    arms={arms}
                    armKey={armKey}
                    onArmChange={onArmChange}
                />
            )}
        </AnimatePresence>
    );
}

function DialogBody({
    onClose,
    campaignId,
    sequence,
    index,
    arms,
    armKey,
    onArmChange,
}: {
    onClose: () => void;
    campaignId: string;
    sequence: Sequence;
    index: number;
    arms: StepArms;
    armKey: string;
    onArmChange: (key: string) => void;
}) {
    const confirm = useConfirm();
    const { user } = useUserProfile();

    const variant = armKey === ORIGINAL_ARM ? null : (arms.variants.find((v) => v.id === armKey) ?? null);
    // A variant deleted from under the dialog falls back to the Original.
    React.useEffect(() => {
        if (armKey !== ORIGINAL_ARM && !variant) onArmChange(ORIGINAL_ARM);
    }, [armKey, variant, onArmChange]);
    const saved: Draft = variant
        ? { subject: variant.subject, bodyHtml: variant.body_html }
        : { subject: sequence.subject, bodyHtml: sequence.body_html };

    // ── Draft + autosave ──────────────────────────────────────────────────
    // Dirty is measured against what this dialog last wrote (not the prop),
    // so a server-side normalisation of the HTML cannot loop the autosave.
    const [draft, setDraft] = React.useState<Draft>(saved);
    const [lastSaved, setLastSaved] = React.useState<Draft>(saved);
    const draftRef = React.useRef(draft);
    draftRef.current = draft;
    const lastSavedRef = React.useRef(lastSaved);
    lastSavedRef.current = lastSaved;
    const dirty = !sameDraft(draft, lastSaved);

    // Switching arms (the previous one was flushed first) swaps the draft in
    // the same render, so the editor never shows the old arm's copy.
    const [savedKey, setSavedKey] = React.useState(armKey);
    if (savedKey !== armKey) {
        setSavedKey(armKey);
        setDraft(saved);
        setLastSaved(saved);
    }

    const updateSequence = useUpdateSequence(campaignId, sequence.id);
    const updateVariant = useUpdateABVariant(campaignId);
    const saving = updateSequence.isPending || updateVariant.isPending;
    const inflightRef = React.useRef<Promise<boolean> | null>(null);
    const timerRef = React.useRef<number | null>(null);
    const clearTimer = () => {
        if (timerRef.current) window.clearTimeout(timerRef.current);
        timerRef.current = null;
    };

    // Writes the draft to its arm. Serialised, so two writes cannot race and
    // land out of order. Resolves false when the save failed (toasted).
    const variantId = variant?.id ?? null;
    const saveVariant = updateVariant.mutateAsync;
    const saveSequence = updateSequence.mutateAsync;
    const saveNow = React.useCallback(async (): Promise<boolean> => {
        while (inflightRef.current) await inflightRef.current;
        clearTimer();
        const d = draftRef.current;
        if (sameDraft(d, lastSavedRef.current)) return true;
        const body = { subject: d.subject, body_html: d.bodyHtml, body_plain: htmlToPlain(d.bodyHtml) };
        const p = (async () => {
            try {
                if (variantId) await saveVariant({ variantId, input: body });
                else await saveSequence(body);
                setLastSaved(d);
                return true;
            } catch (e) {
                toast.error(buildError(e as AppError));
                return false;
            } finally {
                inflightRef.current = null;
            }
        })();
        inflightRef.current = p;
        return p;
    }, [variantId, saveVariant, saveSequence]);

    React.useEffect(() => {
        if (!dirty || savedKey !== armKey) return;
        timerRef.current = window.setTimeout(() => void saveNow(), AUTOSAVE_MS);
        return clearTimer;
    }, [draft, dirty, lastSaved, saveNow, savedKey, armKey]);

    const [closing, setClosing] = React.useState(false);
    async function done() {
        setClosing(true);
        const ok = await saveNow();
        setClosing(false);
        if (ok) onClose();
    }
    const requestClose = React.useCallback(() => {
        if (closing) return;
        if (!dirty) {
            onClose();
            return;
        }
        confirm.show("Discard unsaved changes to this email?", async () => {
            clearTimer();
            onClose();
        });
    }, [closing, dirty, onClose, confirm]);

    React.useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (e.key !== "Escape") return;
            // An open picker or the confirm dialog owns this Escape.
            if (document.querySelector("[data-floating], [role='alertdialog']")) return;
            requestClose();
        };
        document.addEventListener("keydown", onKey);
        return () => document.removeEventListener("keydown", onKey);
    }, [requestClose]);

    async function switchArm(key: string) {
        if (key === armKey) return;
        if (!(await saveNow())) return;
        onArmChange(key);
    }
    async function addVariant() {
        if (!(await saveNow())) return;
        const id = await arms.addVariant();
        if (id) onArmChange(id);
    }

    // ── Preview ───────────────────────────────────────────────────────────
    const [previewContact, setPreviewContact] = React.useState<Contact | null>(null);
    const [mobileTab, setMobileTab] = React.useState<"edit" | "preview">("edit");

    // Every active mailbox in the workspace, so a test can go out from any of
    // them. Pages are pulled until the list is complete.
    const emails = useEmails({ query: "", tag: "", limit: 200 });
    const { hasNextPage, isFetchingNextPage, fetchNextPage } = emails;
    React.useEffect(() => {
        if (hasNextPage && !isFetchingNextPage) fetchNextPage();
    }, [hasNextPage, isFetchingNextPage, fetchNextPage]);
    const mailboxes = React.useMemo(() => emails.emails.filter((e) => e.status === "active"), [emails.emails]);
    const senders = useCampaignSenderInboxes(campaignId);
    const [mailboxId, setMailboxId] = React.useState<string | null>(null);
    const mailboxIds = mailboxes.map((m) => m.id).join(",");
    const senderIds = senders.inboxes.map((m) => m.id).join(",");
    React.useEffect(() => {
        if (mailboxId && mailboxes.some((m) => m.id === mailboxId)) return;
        // Default to one of the campaign's own senders, else the first mailbox.
        const preferred = senders.inboxes.find((s) => mailboxes.some((m) => m.id === s.id)) ?? mailboxes[0];
        setMailboxId(preferred?.id ?? null);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [mailboxIds, senderIds]);
    const mailbox = mailboxes.find((m) => m.id === mailboxId) ?? null;

    // A blank variant field reuses the step's, as the send path does.
    const effective: Draft = {
        subject: variant && !draft.subject.trim() ? sequence.subject : draft.subject,
        bodyHtml: variant && !htmlToPlain(draft.bodyHtml).trim() ? sequence.body_html : draft.bodyHtml,
    };

    // The server render is the truth (signature, opt-out footer, functions);
    // while it is on its way the local render shows the keystroke at once.
    const previewMut = useTemplatePreview();
    const runPreview = previewMut.mutateAsync;
    const previewKey = JSON.stringify([effective.subject, effective.bodyHtml, previewContact?.id ?? "", mailboxId ?? ""]);
    const [serverPreview, setServerPreview] = React.useState<{ key: string; data: TemplatePreview } | null>(null);
    React.useEffect(() => {
        let active = true;
        const t = window.setTimeout(() => {
            runPreview({
                subject: effective.subject,
                body_html: effective.bodyHtml,
                body_plain: htmlToPlain(effective.bodyHtml),
                campaign_id: campaignId,
                step_id: sequence.id,
                ...(previewContact ? { contact_id: previewContact.id } : {}),
                ...(mailboxId ? { account_id: mailboxId } : {}),
            })
                .then((data) => {
                    if (active) setServerPreview({ key: previewKey, data });
                })
                .catch(() => {
                    /* the local render stays up */
                });
        }, 300);
        return () => {
            active = false;
            window.clearTimeout(t);
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [previewKey]);
    const live = serverPreview?.key === previewKey ? serverPreview.data : null;
    const ctx = React.useMemo(() => contactContext(previewContact), [previewContact]);
    const shownSubject = live?.subject ?? renderPreview(effective.subject, ctx);
    const shownBody = live?.body_html ?? linkifyUnsubscribe(renderPreview(effective.bodyHtml, ctx));
    const shownPlain = live?.body_plain ?? "";
    const from = live?.from ?? (mailbox ? { name: mailbox.name, email: mailbox.email } : null);
    const toLabel = previewContact
        ? `${contactLabel(previewContact)} <${previewContact.email}>`
        : `${SAMPLE.FirstName} ${SAMPLE.LastName} <${SAMPLE.Email}>`;

    const { data: allAttachments } = useCampaignAttachments(campaignId);
    const attachments = (allAttachments ?? []).filter((a) => !a.step_id || a.step_id === sequence.id);

    // ── Test send ─────────────────────────────────────────────────────────
    const [recipient, setRecipient] = React.useState(user.email ?? "");
    const [testOpen, setTestOpen] = React.useState(false);
    const [variantName, setVariantName] = React.useState(variant?.name ?? "");
    React.useEffect(() => {
        setVariantName(variant?.name ?? "");
    }, [variant?.id, variant?.name]);
    const commitVariantName = () => {
        const name = variantName.trim();
        if (variant && name && name !== variant.name) void updateVariant.mutateAsync({ variantId: variant.id, input: { name } });
        else if (variant) setVariantName(variant.name);
    };
    // The selected arm's share as typed; committed as a weight on blur.
    const selectedArm = arms.arms.find((a) => a.key === armKey);
    const [shareDraft, setShareDraft] = React.useState(String(selectedArm ? arms.shareOf(selectedArm.weight) : 0));
    React.useEffect(() => {
        setShareDraft(String(selectedArm ? arms.shareOf(selectedArm.weight) : 0));
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [armKey, selectedArm?.weight, arms.arms.length]);
    const commitShare = (key: string) => {
        const pct = Math.max(1, Math.min(100, Number(shareDraft) || 0));
        if (!selectedArm || pct === arms.shareOf(selectedArm.weight)) {
            setShareDraft(String(selectedArm ? arms.shareOf(selectedArm.weight) : 0));
            return;
        }
        arms.commitWeights({ [key]: pct });
    };
    const send = useSendTestEmail(campaignId);
    const recipientOk = EMAIL_RE.test(recipient.trim());
    const testBlocked = variant
        ? "Tests send the Original. Switch to it to send one."
        : !mailbox
          ? emails.isLoading
              ? "Loading mailboxes…"
              : "No active mailbox in this workspace. Connect one under Mailboxes."
          : null;
    async function sendTest() {
        if (testBlocked || !mailbox || !recipientOk) return;
        // The test mails the saved step, so the latest keystrokes go first.
        if (!(await saveNow())) return;
        await toast.promise(
            send.mutateAsync({
                account_id: mailbox.id,
                recipient: recipient.trim(),
                step_id: sequence.id,
                ...(previewContact ? { contact_id: previewContact.id } : {}),
            }),
            {
                loading: "Sending test…",
                success: `Test sent to ${recipient.trim()} from ${mailbox.email}.`,
                error: (e: AppError) => buildError(e),
            },
        );
    }

    const saveState = saving ? "Saving…" : dirty ? "Unsaved changes" : "Saved";

    return createPortal(
        <motion.div
            key="overlay"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            onMouseDown={requestClose}
            className="fixed inset-0 z-[120] flex items-center justify-center bg-slate-900/30 px-3"
        >
            <motion.div
                key="card"
                role="dialog"
                aria-modal="true"
                aria-label={`Edit email, step ${index + 1}`}
                initial={{ y: 8, opacity: 0, scale: 0.985 }}
                animate={{ y: 0, opacity: 1, scale: 1 }}
                exit={{ y: 8, opacity: 0, scale: 0.985 }}
                transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                onMouseDown={(e) => e.stopPropagation()}
                className="relative flex h-[88vh] w-[92vw] max-w-[1280px] flex-col overflow-hidden border border-slate-200 bg-white shadow-[0_24px_48px_-12px_rgba(15,23,42,0.18),0_8px_16px_-8px_rgba(15,23,42,0.1)]"
            >
                <header className="flex h-12 shrink-0 items-center gap-2.5 border-b border-slate-200 px-4">
                    <MailIcon className="w-4 h-4 text-slate-600" />
                    <span className="text-[12.5px] font-medium text-slate-900">Edit email</span>
                    <span className="text-slate-300">·</span>
                    <span className="truncate text-[12.5px] text-slate-600">Step {index + 1}</span>
                    <span
                        className={`ml-2 hidden sm:inline-flex items-center gap-1.5 text-[11px] ${
                            saving ? "text-slate-500" : dirty ? "text-amber-600" : "text-slate-400"
                        }`}
                    >
                        {saving && <Loader2Icon className="w-3 h-3 animate-spin" />}
                        {saveState}
                    </span>
                    <button
                        type="button"
                        onClick={() => setTestOpen(true)}
                        title={testBlocked ?? "Send a test of this step"}
                        className="ml-auto h-7 px-2.5 inline-flex items-center gap-1.5 border border-slate-200 bg-white text-[12px] font-medium text-slate-700 transition-colors hover:bg-slate-50 hover:text-slate-900"
                    >
                        <SendIcon className="w-3.5 h-3.5" />
                        Send test
                    </button>
                    <button
                        type="button"
                        onClick={requestClose}
                        aria-label="Close"
                        className="inline-flex size-7 items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900"
                    >
                        <XIcon className="w-3.5 h-3.5" />
                    </button>
                </header>

                {/* Below md the two columns become tabs. */}
                <div className="flex shrink-0 items-center gap-1 border-b border-slate-200 px-3 md:hidden">
                    <MobileTab active={mobileTab === "edit"} onClick={() => setMobileTab("edit")} icon={<PencilLineIcon className="w-3.5 h-3.5" />}>
                        Edit
                    </MobileTab>
                    <MobileTab active={mobileTab === "preview"} onClick={() => setMobileTab("preview")} icon={<EyeIcon className="w-3.5 h-3.5" />}>
                        Preview
                    </MobileTab>
                </div>

                <div className="grid min-h-0 flex-1 md:grid-cols-2">
                    <div
                        className={`${mobileTab === "edit" ? "flex" : "hidden"} min-h-0 flex-col overflow-y-auto md:flex md:border-r md:border-slate-200`}
                    >
                        {arms.variants.length > 0 ? (
                            <div className="shrink-0 border-b border-slate-200">
                                {/* Neutral arm tabs: name and share, selected is dark. */}
                                {/* Tabs: hairline strip, each arm a tab; the selected one is white and
                                    open at the bottom so it joins the editor. "+" is the last tab. */}
                                <div className="flex items-end overflow-x-auto border-b border-slate-200 px-3 pt-3 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                                    {arms.arms.map((a) =>
                                        a.key === armKey ? (
                                            <div
                                                key={a.key}
                                                className="-mb-px h-9 shrink-0 px-3.5 inline-flex items-center gap-2 border border-slate-200 border-b-white bg-white text-slate-900"
                                            >
                                                {!a.active && (
                                                    <span className="h-5 px-1.5 inline-flex items-center bg-slate-900 text-[10px] font-medium uppercase tracking-[0.1em] text-white">Off</span>
                                                )}
                                                {a.isOriginal ? (
                                                    <span className={`text-[12.5px] font-medium ${a.active ? "" : "line-through text-slate-400"}`}>{a.name}</span>
                                                ) : (
                                                    <input
                                                        value={variantName}
                                                        onChange={(e) => setVariantName(e.target.value)}
                                                        onBlur={commitVariantName}
                                                        onKeyDown={(e) => {
                                                            if (e.key === "Enter") e.currentTarget.blur();
                                                        }}
                                                        aria-label="Variant name"
                                                        className={`h-6 min-w-[3ch] bg-transparent text-[12.5px] font-medium outline-none [field-sizing:content] focus:bg-slate-100 ${a.active ? "text-slate-900" : "line-through text-slate-400"}`}
                                                    />
                                                )}
                                                <input
                                                    value={shareDraft}
                                                    onChange={(e) => setShareDraft(e.target.value.replace(/[^0-9]/g, "").slice(0, 3))}
                                                    onBlur={() => commitShare(a.key)}
                                                    onKeyDown={(e) => {
                                                        if (e.key === "Enter") e.currentTarget.blur();
                                                    }}
                                                    inputMode="numeric"
                                                    aria-label="Traffic share"
                                                    className="h-6 min-w-[2ch] bg-transparent text-right text-[11px] tabular-nums text-slate-500 outline-none [field-sizing:content] focus:bg-slate-100 focus:text-slate-900"
                                                />
                                                <span className="-ml-2 text-[11px] text-slate-400">%</span>
                                                <span className="mx-1 h-4 w-px bg-slate-200" />
                                                <button
                                                    type="button"
                                                    onClick={() =>
                                                        a.isOriginal
                                                            ? arms.toggleOriginal(!a.active)
                                                            : variant && arms.togglePause(variant.id, !variant.is_active)
                                                    }
                                                    disabled={arms.busy}
                                                    aria-label={a.active ? "Disable" : "Enable"}
                                                    title={a.active ? "Disable: keeps the copy, sends none of the traffic" : "Enable"}
                                                    className="inline-flex size-6 items-center justify-center text-slate-400 hover:text-slate-900"
                                                >
                                                    {a.active ? <EyeOffIcon className="w-3.5 h-3.5" /> : <EyeIcon className="w-3.5 h-3.5" />}
                                                </button>
                                                {(!a.isOriginal || arms.variants.length > 0) && (
                                                    <button
                                                        type="button"
                                                        onClick={() =>
                                                            a.isOriginal
                                                                ? arms.deleteOriginal(() => onArmChange(ORIGINAL_ARM))
                                                                : variant && arms.deleteArm(variant.id, () => onArmChange(ORIGINAL_ARM))
                                                        }
                                                        disabled={arms.busy}
                                                        aria-label="Delete"
                                                        title={a.isOriginal ? "Delete the Original: a variant becomes the step's email" : "Delete this variant"}
                                                        className="inline-flex size-6 items-center justify-center text-slate-400 hover:text-rose-600"
                                                    >
                                                        <TrashIcon className="w-3.5 h-3.5" />
                                                    </button>
                                                )}
                                            </div>
                                        ) : (
                                            <button
                                                key={a.key}
                                                type="button"
                                                onClick={() => void switchArm(a.key)}
                                                className="h-9 shrink-0 px-3.5 inline-flex items-center gap-2 border border-transparent bg-slate-50 text-[12.5px] text-slate-600 transition-colors hover:bg-slate-100 hover:text-slate-900"
                                            >
                                                {!a.active && (
                                                    <span className="h-5 px-1.5 inline-flex items-center bg-slate-300 text-[10px] font-medium uppercase tracking-[0.1em] text-white">Off</span>
                                                )}
                                                <span className={a.active ? "" : "line-through text-slate-400"}>{a.name}</span>
                                                <span className="tabular-nums text-[11px] text-slate-400">{arms.shareOf(a.active ? a.weight : 0)}%</span>
                                            </button>
                                        ),
                                    )}
                                    {arms.canAdd && (
                                        <button
                                            type="button"
                                            onClick={() => void addVariant()}
                                            disabled={arms.adding}
                                            aria-label="Add a variant"
                                            title="Add a variant"
                                            className="h-9 shrink-0 px-3 inline-flex items-center border border-transparent text-slate-500 transition-colors hover:text-slate-900 disabled:opacity-60"
                                        >
                                            {arms.adding ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <PlusIcon className="w-3.5 h-3.5" />}
                                        </button>
                                    )}
                                </div>
                            </div>
                        ) : (
                            <div className="flex shrink-0 items-center gap-2 border-b border-slate-200 px-3 py-2">
                                <span className="text-[12px] text-slate-500">One version of this email.</span>
                                {arms.canAdd && (
                                    <button
                                        type="button"
                                        onClick={() => void addVariant()}
                                        disabled={arms.adding}
                                        title="Split this step's traffic between two or more versions"
                                        className="ml-auto h-7 px-2.5 inline-flex items-center gap-1.5 border border-slate-200 bg-white text-[12px] font-medium text-slate-700 hover:bg-slate-50"
                                    >
                                        {arms.adding ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <PlusIcon className="w-3.5 h-3.5" />}
                                        A/B test
                                    </button>
                                )}
                            </div>
                        )}
                        <div className="space-y-4 p-4">
                            <EmailContentEditor
                                key={armKey}
                                subject={draft.subject}
                                onSubjectChange={(v) => setDraft((d) => ({ ...d, subject: v }))}
                                bodyHtml={draft.bodyHtml}
                                onBodyChange={(html) => setDraft((d) => ({ ...d, bodyHtml: html }))}
                                subjectPlaceholder={variant ? "Leave blank to reuse the step's subject" : undefined}
                                bodyPlaceholder={variant ? "Leave blank to reuse the step's body" : undefined}
                                campaignId={campaignId}
                                stepId={sequence.id}
                                previewTab={false}
                                bodyMinHeight={360}
                            />
                            <StepAttachments campaignId={campaignId} sequenceId={sequence.id} />
                        </div>
                    </div>

                    <div
                        className={`${mobileTab === "preview" ? "flex" : "hidden"} min-h-0 flex-col overflow-y-auto bg-slate-50 md:flex`}
                    >
                        <div className="flex shrink-0 flex-wrap items-center gap-1.5 border-b border-slate-200 bg-white px-3 py-2">
                            <span className="px-1 text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Preview as</span>
                            <PreviewContactPicker campaignId={campaignId} value={previewContact} onChange={setPreviewContact} />
                            {previewMut.isPending && (
                                <span className="ml-auto inline-flex items-center gap-1 text-[11px] text-slate-400">
                                    <Loader2Icon className="w-3 h-3 animate-spin" /> Rendering
                                </span>
                            )}
                        </div>
                        <div className="p-4">
                            <div className="rounded-md border border-slate-200 bg-white">
                                <div className="space-y-1 border-b border-slate-200 px-4 py-3 text-[12.5px]">
                                    <div className="text-[14px] font-medium text-slate-900">{shownSubject || "(no subject)"}</div>
                                    <div className="flex flex-wrap gap-x-1 text-slate-600">
                                        <span className="text-slate-400">From</span>
                                        <span>{from ? (from.name ? `${from.name} <${from.email}>` : from.email) : "No mailbox selected"}</span>
                                    </div>
                                    <div className="flex flex-wrap gap-x-1 text-slate-600">
                                        <span className="text-slate-400">To</span>
                                        <span>{toLabel}</span>
                                    </div>
                                </div>
                                <div
                                    className="tiptap-body min-h-[240px] px-4 py-3 text-[13px] leading-relaxed text-slate-800"
                                    dangerouslySetInnerHTML={{
                                        __html:
                                            shownBody ||
                                            (shownPlain
                                                ? `<pre class="whitespace-pre-wrap font-sans">${escapeHtml(shownPlain)}</pre>`
                                                : '<p class="text-slate-300">Nothing to preview yet.</p>'),
                                    }}
                                />
                                {attachments.length > 0 && (
                                    <ul className="flex flex-wrap gap-1.5 border-t border-slate-200 px-4 py-2.5">
                                        {attachments.map((a) => (
                                            <li
                                                key={a.id}
                                                title={a.mime_type}
                                                className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-slate-50 px-2 py-0.5 text-[11px] text-slate-700"
                                            >
                                                <PaperclipIcon className="w-3 h-3 text-slate-400" />
                                                <span className="max-w-[200px] truncate">{a.filename}</span>
                                                <span className="text-slate-400">{formatBytes(a.size)}</span>
                                            </li>
                                        ))}
                                    </ul>
                                )}
                            </div>
                            {(live?.errors?.length ?? 0) > 0 && (
                                <p className="mt-2 flex items-start gap-1.5 text-[11px] text-rose-600">
                                    <AlertCircleIcon className="mt-px w-3.5 h-3.5 shrink-0" />
                                    <span>{live!.errors!.join(" · ")}. Fix before sending.</span>
                                </p>
                            )}
                            {(live?.unresolved?.length ?? 0) > 0 && (
                                <p className="mt-2 flex items-start gap-1.5 text-[11px] text-amber-600">
                                    <AlertCircleIcon className="mt-px w-3.5 h-3.5 shrink-0" />
                                    <span>These won&apos;t resolve and would send literally: {live!.unresolved!.join(", ")}</span>
                                </p>
                            )}
                            <p className="mt-2 text-[10.5px] leading-relaxed text-slate-400">
                                Rendered with the real send engine for{" "}
                                {previewContact ? contactLabel(previewContact) : SAMPLE_CONTACT_LABEL}
                                {mailbox ? `, with ${mailbox.email}'s signature` : ""} and the campaign&apos;s opt-out footer.
                                Tracking links are not rewritten here.
                            </p>
                        </div>
                    </div>
                </div>

                <footer className="flex shrink-0 items-center justify-end gap-2 border-t border-slate-200 bg-slate-50/30 px-3 py-2">
                    <button
                        type="button"
                        onClick={() => void done()}
                        disabled={closing}
                        className="h-7 px-3 inline-flex items-center gap-1.5 bg-sky-600 text-[12px] font-medium text-white transition-colors hover:bg-sky-700 disabled:opacity-60"
                    >
                        {closing && <Loader2Icon className="w-3 h-3 animate-spin" />}
                        Done
                    </button>
                </footer>

                {/* Test send lives in its own small dialog over the editor, so the
                    mailbox and recipient never have to squeeze into a bar. */}
                {testOpen && (
                    <div
                        data-floating
                        className="absolute inset-0 z-10 flex items-center justify-center bg-slate-900/30"
                        onMouseDown={(e) => {
                            if (e.target === e.currentTarget) setTestOpen(false);
                        }}
                        onKeyDown={(e) => {
                            if (e.key === "Escape") {
                                e.stopPropagation();
                                setTestOpen(false);
                            }
                        }}
                    >
                        <div
                            role="dialog"
                            aria-label="Send test email"
                            className="w-[min(440px,92vw)] border border-slate-200 bg-white shadow-lg"
                            onMouseDown={(e) => e.stopPropagation()}
                        >
                            <div className="flex h-11 items-center gap-2 border-b border-slate-200 px-4">
                                <SendIcon className="w-3.5 h-3.5 text-slate-500" />
                                <span className="text-[12.5px] font-medium text-slate-900">Send test email</span>
                                <button
                                    type="button"
                                    onClick={() => setTestOpen(false)}
                                    aria-label="Close"
                                    className="ml-auto inline-flex size-7 items-center justify-center text-slate-500 hover:bg-slate-100 hover:text-slate-900"
                                >
                                    <XIcon className="w-3.5 h-3.5" />
                                </button>
                            </div>
                            <div className="space-y-3 px-4 py-4">
                                <div className="space-y-1">
                                    <div className="text-[11px] font-medium text-slate-500">From mailbox</div>
                                    <TestMailboxPicker mailboxes={mailboxes} value={mailbox} onChange={(m) => setMailboxId(m.id)} loading={emails.isLoading} />
                                </div>
                                <div className="space-y-1">
                                    <div className="text-[11px] font-medium text-slate-500">Send to</div>
                                    <TextInput
                                        type="email"
                                        value={recipient}
                                        onChange={setRecipient}
                                        placeholder="you@company.com"
                                        invalid={recipient.length > 0 && !recipientOk}
                                        className="w-full"
                                        autoFocus
                                    />
                                </div>
                                <p className="text-[11.5px] text-slate-500">
                                    Sends the saved step through a real worker with the mailbox signature and the opt-out footer. Tracking is off.
                                </p>
                                {testBlocked && <p className="text-[11.5px] text-amber-600">{testBlocked}</p>}
                            </div>
                            <div className="flex items-center justify-end gap-2 border-t border-slate-200 px-4 py-2.5">
                                <button
                                    type="button"
                                    onClick={() => setTestOpen(false)}
                                    className="h-7 px-3 inline-flex items-center text-[12px] font-medium text-slate-600 hover:text-slate-900"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="button"
                                    onClick={() => void sendTest().then(() => setTestOpen(false))}
                                    disabled={!!testBlocked || !recipientOk || send.isPending}
                                    className="h-7 px-3 inline-flex items-center gap-1.5 bg-sky-600 text-[12px] font-medium text-white transition-colors hover:bg-sky-700 disabled:opacity-60"
                                >
                                    {send.isPending ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <SendIcon className="w-3.5 h-3.5" />}
                                    Send
                                </button>
                            </div>
                        </div>
                    </div>
                )}
            </motion.div>
        </motion.div>,
        document.body,
    );
}

// Lists every active mailbox in the workspace by name and address.
function TestMailboxPicker({
    mailboxes,
    value,
    onChange,
    loading,
}: {
    mailboxes: Inbox[];
    value: Inbox | null;
    onChange: (m: Inbox) => void;
    loading: boolean;
}) {
    return (
        <PopoverMenu>
            <PopoverMenuTrigger asChild>
                <SelectButton
                    icon={<MailIcon className="w-3.5 h-3.5" />}
                    label={value ? value.email : loading ? "Loading…" : "No active mailbox"}
                    title="Mailbox the test is sent from"
                />
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={300} className="max-h-72 overflow-y-auto p-1">
                <PopoverMenuLabel>All active mailboxes</PopoverMenuLabel>
                {mailboxes.length === 0 ? (
                    <div className="px-3 py-2 text-[11.5px] text-slate-400">
                        {loading ? "Loading…" : "No active mailbox in this workspace yet."}
                    </div>
                ) : (
                    mailboxes.map((m) => (
                        <PopoverMenuItem key={m.id} selected={value?.id === m.id} onSelect={() => onChange(m)}>
                            <span className="flex min-w-0 flex-col">
                                <span className="truncate text-slate-800">{m.name || m.email}</span>
                                {m.name && <span className="truncate text-[11px] text-slate-400">{m.email}</span>}
                            </span>
                        </PopoverMenuItem>
                    ))
                )}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}

function MobileTab({
    active,
    onClick,
    icon,
    children,
}: {
    active: boolean;
    onClick: () => void;
    icon: React.ReactNode;
    children: React.ReactNode;
}) {
    return (
        <button
            type="button"
            onClick={onClick}
            className={`relative h-10 px-2.5 inline-flex items-center gap-1.5 text-[12.5px] ${
                active ? "text-slate-900 font-medium" : "text-slate-500 hover:text-slate-900"
            }`}
        >
            {icon}
            {children}
            {active && <span className="absolute inset-x-0 bottom-0 h-0.5 bg-sky-600" />}
        </button>
    );
}

// Merge-field values for the local (instant) render: the chosen contact's
// standard fields over the sample ones.
function contactContext(c: Contact | null): Record<string, string> {
    if (!c) return SAMPLE;
    return {
        ...SAMPLE,
        FirstName: c.first_name ?? "",
        LastName: c.last_name ?? "",
        Email: c.email ?? "",
        Company: c.company ?? "",
        Phone: c.phone ?? "",
    };
}

function escapeHtml(s: string): string {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
