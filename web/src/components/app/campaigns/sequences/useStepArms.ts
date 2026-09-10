// The data and mutations behind a step's A/B arms, shared by the step pane
// (which shows the split and the selected arm's settings) and the email
// dialog (which edits the selected arm's copy). The Original is the step's own
// email; its weight persists as an is_control variant row, created lazily the
// first time its share moves off the default.

import React from "react";
import toast from "react-hot-toast";
import type Sequence from "@/lib/api/models/app/campaigns/sequences/Sequence";
import type ABVariant from "@/lib/api/models/app/campaigns/ABVariant";
import type { ABVariantStats } from "@/lib/api/models/app/campaigns/ABVariant";
import type { SplitArm } from "./StepSplitAllocator";
import { htmlToPlain } from "./emailPreview";
import {
    useCampaignABVariants,
    useCampaignABAnalysis,
    useCreateABVariant,
    useUpdateABVariant,
    useDeleteABVariant,
} from "@/lib/api/hooks/app/campaigns/useCampaignABVariants";
import useUpdateSequence from "@/lib/api/hooks/app/campaigns/sequences/useUpdateSequence";
import { useConfirm } from "@/hooks/context/confirm";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

export const ORIGINAL_ARM = "original";
const LETTERS = ["B", "C", "D", "E", "F"];
// Must match abControlWeight in internal/app/advanced/service.go.
const CONTROL_WEIGHT = 100;
const MAX_VARIANTS = 5;

const clampW = (w: number) => Math.min(100, Math.max(1, Math.round(w)));
const err = (e: unknown) => toast.error(buildError(e as unknown as AppError));

export function variantLabel(v: ABVariant, i: number): string {
    return v.name || `Variant ${LETTERS[i] ?? i + 1}`;
}

export interface StepArms {
    arms: SplitArm[];
    variants: ABVariant[];
    controlRow: ABVariant | null;
    statsById: Map<string, ABVariantStats>;
    winnerId: string | null;
    busy: boolean;
    adding: boolean;
    canAdd: boolean;
    commitWeights: (next: Record<string, number>) => void;
    evenSplit: () => void;
    togglePause: (variantId: string, active: boolean) => void;
    toggleOriginal: (active: boolean) => void;
    renameOriginal: (name: string) => void;
    deleteOriginal: (after?: () => void) => void;
    deleteArm: (variantId: string, after?: () => void) => void;
    // Resolves to the new variant's id, or null when creation failed.
    addVariant: () => Promise<string | null>;
    shareOf: (weight: number) => number;
}

export default function useStepArms(campaignId: string, sequence: Sequence): StepArms {
    const { data: all } = useCampaignABVariants(campaignId);
    const stepRows = React.useMemo(() => (all ?? []).filter((v) => v.step_id === sequence.id), [all, sequence.id]);
    const controlRow = stepRows.find((v) => v.is_control) ?? null;
    const variants = React.useMemo(() => stepRows.filter((v) => !v.is_control), [stepRows]);

    const create = useCreateABVariant(campaignId);
    const update = useUpdateABVariant(campaignId);
    const del = useDeleteABVariant(campaignId);
    const updateSequence = useUpdateSequence(campaignId, sequence.id);
    const confirm = useConfirm();
    const busy = create.isPending || update.isPending || del.isPending;

    const { data: analysis } = useCampaignABAnalysis(campaignId, (all ?? []).length > 0);
    const statsById = React.useMemo(() => {
        const m = new Map<string, ABVariantStats>();
        for (const s of analysis?.variants ?? []) m.set(s.variant_id, s);
        return m;
    }, [analysis]);
    const winnerId = analysis?.winner_id ?? null;

    const originalWeight = controlRow ? controlRow.weight : CONTROL_WEIGHT;
    const arms: SplitArm[] = [
        { key: ORIGINAL_ARM, name: controlRow?.name?.trim() || "Original", weight: originalWeight, active: controlRow ? controlRow.is_active : true, isOriginal: true },
        ...variants.map((v, i) => ({
            key: v.id,
            name: variantLabel(v, i),
            weight: v.weight,
            active: v.is_active,
            isOriginal: false,
            winner: winnerId === v.id,
        })),
    ];

    // Approximate share of the active split, for the editor chip.
    const activeWeightSum = arms.filter((a) => a.active).reduce((s, a) => s + Math.max(a.weight, 1), 0);
    const shareOf = (w: number) => (activeWeightSum > 0 ? Math.round((Math.max(w, 1) / activeWeightSum) * 100) : 0);

    // Persist a new split: the Original maps to its control row (created lazily),
    // each variant to its own weight. Only changed arms are written.
    const commitWeights = (next: Record<string, number>) => {
        const tasks: Promise<unknown>[] = [];
        if (next[ORIGINAL_ARM] != null) {
            const ow = clampW(next[ORIGINAL_ARM]);
            if (controlRow) {
                if (controlRow.weight !== ow) {
                    tasks.push(update.mutateAsync({ variantId: controlRow.id, input: { weight: ow } }));
                }
            } else if (ow !== CONTROL_WEIGHT) {
                tasks.push(
                    create.mutateAsync({
                        name: "Original",
                        step_id: sequence.id,
                        weight: ow,
                        is_control: true,
                        is_active: true,
                    }),
                );
            }
        }
        for (const v of variants) {
            const raw = next[v.id];
            if (raw == null) continue;
            const w = clampW(raw);
            if (w !== v.weight) tasks.push(update.mutateAsync({ variantId: v.id, input: { weight: w } }));
        }
        if (tasks.length) Promise.all(tasks).catch(err);
    };

    const evenSplit = () => {
        const keys = [ORIGINAL_ARM, ...variants.filter((v) => v.is_active).map((v) => v.id)];
        const n = keys.length;
        if (n < 2) return;
        const base = Math.floor(100 / n);
        const rem = 100 - base * n;
        const next: Record<string, number> = {};
        keys.forEach((k, i) => (next[k] = base + (i < rem ? 1 : 0)));
        commitWeights(next);
    };

    const togglePause = (variantId: string, active: boolean) => {
        const v = variants.find((x) => x.id === variantId);
        const label = v ? v.name : "this variant";
        confirm.show(
            active
                ? `Enable "${label}"? It gets its share of traffic again.`
                : `Disable "${label}"? Its copy stays, but it sends none of the traffic until enabled.`,
            async () => {
                await update.mutateAsync({ variantId, input: { is_active: active } });
            },
        );
    };

    // The Original is the step's own content. Its is_control row carries the
    // share and the on/off state; it is created on demand.
    const toggleOriginal = (active: boolean) => {
        confirm.show(
            active
                ? "Enable the Original? It gets its share of traffic again."
                : "Disable the Original? The step's own copy stays, but only the variants send until it is enabled.",
            async () => {
                if (controlRow) {
                    await update.mutateAsync({ variantId: controlRow.id, input: { is_active: active } });
                } else {
                    await create.mutateAsync({
                        name: "Original",
                        step_id: sequence.id,
                        weight: CONTROL_WEIGHT,
                        is_control: true,
                        is_active: active,
                    });
                }
            },
        );
    };

    // The Original's name lives on its control row, created on demand.
    const renameOriginal = (name: string) => {
        const clean = name.trim();
        if (!clean) return;
        if (controlRow) {
            if (clean !== controlRow.name) update.mutate({ variantId: controlRow.id, input: { name: clean } }, { onError: err });
        } else if (clean !== "Original") {
            create.mutate(
                { name: clean, step_id: sequence.id, weight: CONTROL_WEIGHT, is_control: true, is_active: true },
                { onError: err },
            );
        }
    };

    // Deleting the Original promotes a variant: its copy becomes the step's own
    // content and the variant row goes away.
    const deleteOriginal = (after?: () => void) => {
        const promoted = variants.find((v) => v.is_active) ?? variants[0];
        if (!promoted) return;
        confirm.show(
            `Delete the Original? "${promoted.name}" becomes the step's email and the Original's copy is removed.`,
            async () => {
                await updateSequence.mutateAsync({ subject: promoted.subject, body_html: promoted.body_html });
                await del.mutateAsync(promoted.id);
                if (variants.length === 1 && controlRow) {
                    try {
                        await del.mutateAsync(controlRow.id);
                    } catch {
                        /* harmless */
                    }
                }
                after?.();
            },
        );
    };

    const deleteArm = (variantId: string, after?: () => void) => {
        const v = variants.find((x) => x.id === variantId);
        if (!v) return;
        confirm.show(`Delete "${v.name}"? Its content will be removed from this step.`, async () => {
            await del.mutateAsync(v.id);
            // Removing the last variant reverts the step to a single arm: drop the
            // control row too so the step stops A/B splitting entirely.
            if (variants.length === 1 && controlRow) {
                try {
                    await del.mutateAsync(controlRow.id);
                } catch {
                    /* harmless: a lone control row still sends the original */
                }
            }
            after?.();
            toast.success("Variant removed.");
        });
    };

    const addVariant = async (): Promise<string | null> => {
        try {
            const v = await create.mutateAsync({
                name: `Variant ${LETTERS[variants.length] ?? variants.length + 1}`,
                step_id: sequence.id,
                weight: CONTROL_WEIGHT,
                is_active: true,
                subject: sequence.subject,
                body_html: sequence.body_html,
                body_plain: htmlToPlain(sequence.body_html ?? ""),
            });
            return v?.id ?? null;
        } catch (e) {
            err(e);
            return null;
        }
    };

    return {
        arms,
        variants,
        controlRow,
        statsById,
        winnerId,
        busy,
        adding: create.isPending,
        canAdd: variants.length < MAX_VARIANTS,
        commitWeights,
        evenSplit,
        togglePause,
        toggleOriginal,
        renameOriginal,
        deleteOriginal,
        deleteArm,
        addVariant,
        shareOf,
    };
}
