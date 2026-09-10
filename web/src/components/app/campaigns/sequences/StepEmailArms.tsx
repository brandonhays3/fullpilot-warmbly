// The email section of the step pane. The copy itself is not edited here:
// the pane shows the split between the step's A/B arms, the selected arm's
// settings (name; pause, delete and results for a variant), and a compact
// summary of its email, with "Edit email" opening the full two-column dialog
// (EditEmailDialog). The Original is the step's own email, the control; its
// weight persists as an is_control variant row, created lazily.

import React from "react";
import { GitBranchIcon, Loader2Icon, MailIcon, PaperclipIcon, PauseIcon, PlayIcon, SplitIcon, Trash2Icon, TrophyIcon } from "@/components/icons";
import toast from "react-hot-toast";
import type Sequence from "@/lib/api/models/app/campaigns/sequences/Sequence";
import type ABVariant from "@/lib/api/models/app/campaigns/ABVariant";
import type { ABVariantStats } from "@/lib/api/models/app/campaigns/ABVariant";
import { Label, TextInput } from "@/components/ui/field";
import StepSplitAllocator from "./StepSplitAllocator";
import EditEmailDialog from "./EditEmailDialog";
import useStepArms, { ORIGINAL_ARM, variantLabel } from "./useStepArms";
import { htmlToPlain } from "./emailPreview";
import { useCampaignAttachments } from "@/lib/api/hooks/app/campaigns/useCampaignAttachments";
import { useUpdateABVariant } from "@/lib/api/hooks/app/campaigns/useCampaignABVariants";
import useUpdateSequence from "@/lib/api/hooks/app/campaigns/sequences/useUpdateSequence";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

export default function StepEmailArms({
    campaignId,
    sequence,
    index,
}: {
    campaignId: string;
    sequence: Sequence;
    index: number;
}) {
    const arms = useStepArms(campaignId, sequence);
    const { variants } = arms;

    const [selected, setSelected] = React.useState<string>(ORIGINAL_ARM);
    React.useEffect(() => {
        if (selected !== ORIGINAL_ARM && !variants.some((v) => v.id === selected)) setSelected(ORIGINAL_ARM);
    }, [variants, selected]);
    const [editing, setEditing] = React.useState(false);

    const variantIndex = variants.findIndex((v) => v.id === selected);
    const variant = variantIndex >= 0 ? variants[variantIndex] : null;

    const { data: allAttachments } = useCampaignAttachments(campaignId);
    const attachmentCount = (allAttachments ?? []).filter((a) => !a.step_id || a.step_id === sequence.id).length;

    const updateSequence = useUpdateSequence(campaignId, sequence.id);
    const updateVariant = useUpdateABVariant(campaignId);

    const addVariant = async () => {
        const id = await arms.addVariant();
        if (id) setSelected(id);
    };

    const subject = variant ? variant.subject : sequence.subject;
    const bodyPlain = htmlToPlain(variant ? variant.body_html : sequence.body_html).replace(/\s+/g, " ").trim();

    return (
        <div className="rounded-md border border-slate-200 bg-white">
            {variants.length > 0 && (
                <StepSplitAllocator
                    arms={arms.arms}
                    selectedKey={selected}
                    onSelect={setSelected}
                    onCommit={arms.commitWeights}
                    onAdd={() => void addVariant()}
                    onEven={arms.evenSplit}
                    canAdd={arms.canAdd}
                    adding={arms.adding}
                    busy={arms.busy}
                />
            )}

            <div className="space-y-3 p-3">
                {variant ? (
                    <VariantSettings
                        variant={variant}
                        label={variantLabel(variant, variantIndex)}
                        stats={arms.statsById.get(variant.id)}
                        isWinner={arms.winnerId === variant.id}
                        sharePct={variant.is_active ? arms.shareOf(variant.weight) : 0}
                        onRename={(name) => updateVariant.mutateAsync({ variantId: variant.id, input: { name } })}
                        onTogglePause={(active) => arms.togglePause(variant.id, active)}
                        onDelete={() => arms.deleteArm(variant.id, () => setSelected(ORIGINAL_ARM))}
                    />
                ) : (
                    <div>
                        <Label>Step name</Label>
                        <NameField
                            key={sequence.id}
                            value={sequence.name}
                            placeholder={`Step ${index + 1}`}
                            onCommit={(name) => updateSequence.mutateAsync({ name })}
                        />
                        <p className="mt-1.5 text-[10.5px] text-slate-400">Internal label only. Recipients never see it.</p>
                    </div>
                )}

                <div className="rounded-md border border-slate-200 bg-slate-50/60 px-3 py-2.5">
                    <div className="flex items-center gap-1.5">
                        <MailIcon className="w-3.5 h-3.5 text-slate-400" />
                        <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">
                            {variant ? variantLabel(variant, variantIndex) : variants.length > 0 ? "Original" : "Email"}
                        </span>
                    </div>
                    <div className={`mt-1.5 truncate text-[12.5px] font-medium ${subject ? "text-slate-900" : "text-slate-400"}`}>
                        {subject || (variant ? "Reuses the step's subject" : "No subject yet")}
                    </div>
                    <p className={`mt-1 line-clamp-2 text-[12px] leading-relaxed ${bodyPlain ? "text-slate-600" : "text-slate-400"}`}>
                        {bodyPlain || (variant ? "Reuses the step's body." : "Nothing written yet.")}
                    </p>
                    <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-slate-500">
                        <span className="inline-flex items-center gap-1">
                            <PaperclipIcon className="w-3 h-3 text-slate-400" />
                            {attachmentCount === 0 ? "No attachments" : `${attachmentCount} attachment${attachmentCount === 1 ? "" : "s"}`}
                        </span>
                        <span className="inline-flex items-center gap-1">
                            <SplitIcon className="w-3 h-3 text-slate-400" />
                            {variants.length === 0 ? "No A/B test" : `${arms.arms.length} arms`}
                        </span>
                    </div>
                </div>

                <div className="flex items-center gap-2">
                    <button
                        type="button"
                        onClick={() => setEditing(true)}
                        className="h-8 flex-1 px-3 inline-flex items-center justify-center gap-1.5 rounded-md bg-sky-600 text-[12.5px] font-medium text-white transition-colors hover:bg-sky-700"
                    >
                        <MailIcon className="w-4 h-4" />
                        Edit email
                    </button>
                    {variants.length === 0 && (
                        <button
                            type="button"
                            onClick={() => void addVariant()}
                            disabled={arms.adding}
                            title="Split this step's traffic between two or more versions"
                            className="h-8 px-2.5 inline-flex items-center gap-1.5 rounded-md border border-slate-200 bg-white text-[12px] font-medium text-slate-600 transition-colors hover:border-slate-300 hover:text-slate-900 disabled:opacity-50"
                        >
                            {arms.adding ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <SplitIcon className="w-3.5 h-3.5" />}
                            A/B test
                        </button>
                    )}
                </div>

                {index > 0 && (
                    <div className="flex items-start gap-2 rounded-md border border-slate-200 bg-slate-50/60 px-3 py-2.5">
                        <GitBranchIcon className="mt-0.5 w-3.5 h-3.5 shrink-0 text-slate-400" />
                        <p className="text-[11px] leading-relaxed text-slate-500">
                            Follow-ups thread on the previous step&apos;s subject. Change this subject and the follow-up
                            starts a new thread instead of replying in the existing one.
                        </p>
                    </div>
                )}
            </div>

            <EditEmailDialog
                open={editing}
                onClose={() => setEditing(false)}
                campaignId={campaignId}
                sequence={sequence}
                index={index}
                arms={arms}
                armKey={selected}
                onArmChange={setSelected}
            />
        </div>
    );
}

// A text field that writes on blur (or Enter) when its value changed.
function NameField({
    value,
    placeholder,
    onCommit,
}: {
    value: string;
    placeholder?: string;
    onCommit: (v: string) => Promise<unknown>;
}) {
    const [draft, setDraft] = React.useState(value);
    React.useEffect(() => setDraft(value), [value]);
    const commit = () => {
        const next = draft.trim();
        if (next === value) return;
        onCommit(next).catch((e) => {
            toast.error(buildError(e as AppError));
            setDraft(value);
        });
    };
    return (
        <TextInput
            value={draft}
            onChange={setDraft}
            placeholder={placeholder}
            onBlur={commit}
            onKeyDown={(e) => {
                if (e.key === "Enter") (e.target as HTMLInputElement).blur();
            }}
        />
    );
}

function VariantSettings({
    variant,
    label,
    stats,
    isWinner,
    sharePct,
    onRename,
    onTogglePause,
    onDelete,
}: {
    variant: ABVariant;
    label: string;
    stats?: ABVariantStats;
    isWinner: boolean;
    sharePct: number;
    onRename: (name: string) => Promise<unknown>;
    onTogglePause: (active: boolean) => void;
    onDelete: () => void;
}) {
    return (
        <div className="space-y-2">
            <div className="flex items-end gap-2">
                <div className="min-w-0 flex-1">
                    <Label>Variant name</Label>
                    <NameField key={variant.id} value={variant.name} placeholder={label} onCommit={onRename} />
                </div>
                <span
                    className={`h-7 shrink-0 inline-flex items-center rounded-md px-2 text-[11px] font-medium tabular-nums ${
                        variant.is_active ? "bg-sky-50 text-sky-700" : "bg-slate-100 text-slate-400"
                    }`}
                >
                    {variant.is_active ? `${sharePct}% of contacts` : "Paused"}
                </span>
                <button
                    type="button"
                    onClick={() => onTogglePause(!variant.is_active)}
                    title={variant.is_active ? "Pause this variant" : "Resume this variant"}
                    className="size-7 shrink-0 inline-flex items-center justify-center rounded-md text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                >
                    {variant.is_active ? <PauseIcon className="w-3.5 h-3.5" /> : <PlayIcon className="w-3.5 h-3.5" />}
                </button>
                <button
                    type="button"
                    onClick={onDelete}
                    title="Delete variant"
                    className="size-7 shrink-0 inline-flex items-center justify-center rounded-md text-slate-400 transition-colors hover:bg-rose-50 hover:text-rose-600"
                >
                    <Trash2Icon className="w-3.5 h-3.5" />
                </button>
            </div>
            {stats && stats.total_sent > 0 && (
                <div className="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-md bg-slate-50 px-2.5 py-1.5 text-[11px]">
                    {isWinner && (
                        <span className="inline-flex items-center gap-1 text-amber-700 font-medium">
                            <TrophyIcon className="w-3 h-3" /> Winner
                        </span>
                    )}
                    <Metric label="Sent" value={stats.total_sent.toLocaleString()} />
                    <Metric label="Open" value={`${stats.open_rate.toFixed(1)}%`} tone="text-emerald-600" />
                    <Metric label="Reply" value={`${stats.reply_rate.toFixed(1)}%`} tone="text-sky-600" />
                    <Metric label="Bounce" value={`${stats.bounce_rate.toFixed(1)}%`} tone="text-rose-600" />
                </div>
            )}
        </div>
    );
}

function Metric({ label, value, tone }: { label: string; value: string; tone?: string }) {
    return (
        <span className="inline-flex items-center gap-1 tabular-nums">
            <span className="text-slate-400">{label}</span>
            <span className={`font-medium ${tone ?? "text-slate-700"}`}>{value}</span>
        </span>
    );
}
