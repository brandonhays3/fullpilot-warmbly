// The email section of the step pane. The copy itself is not edited here:
// the pane shows the split between the step's A/B arms, the selected arm's
// settings (name; pause, delete and results for a variant), and a compact
// summary of its email, with "Edit email" opening the full two-column dialog
// (EditEmailDialog). The Original is the step's own email, the control; its
// weight persists as an is_control variant row, created lazily.

import React from "react";
import { GitBranchIcon, MailIcon, PaperclipIcon, SplitIcon } from "@/components/icons";
import toast from "react-hot-toast";
import type Sequence from "@/lib/api/models/app/campaigns/sequences/Sequence";
import { Label, TextInput } from "@/components/ui/field";
import EditEmailDialog from "./EditEmailDialog";
import useStepArms, { ORIGINAL_ARM } from "./useStepArms";
import { htmlToPlain } from "./emailPreview";
import { useCampaignAttachments } from "@/lib/api/hooks/app/campaigns/useCampaignAttachments";
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


    const { data: allAttachments } = useCampaignAttachments(campaignId);
    const attachmentCount = (allAttachments ?? []).filter((a) => !a.step_id || a.step_id === sequence.id).length;

    const updateSequence = useUpdateSequence(campaignId, sequence.id);


    const subject = sequence.subject;
    const bodyPlain = htmlToPlain(sequence.body_html).replace(/\s+/g, " ").trim();

    return (
        <div className="rounded-md border border-slate-200 bg-white">

            <div className="space-y-3 p-3">
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

                <div className="rounded-md border border-slate-200 bg-slate-50/60 px-3 py-2.5">
                    <div className="flex items-center gap-1.5">
                        <MailIcon className="w-3.5 h-3.5 text-slate-400" />
                        <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">
                            Email
                        </span>
                    </div>
                    <div className={`mt-1.5 truncate text-[12.5px] font-medium ${subject ? "text-slate-900" : "text-slate-400"}`}>
                        {subject || "No subject yet"}
                    </div>
                    <p className={`mt-1 line-clamp-2 text-[12px] leading-relaxed ${bodyPlain ? "text-slate-600" : "text-slate-400"}`}>
                        {bodyPlain || "Nothing written yet."}
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

