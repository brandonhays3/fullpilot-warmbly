// AI settings: the workspace's own OpenRouter key and the default model its AI
// blocks run on. The key is write-only: once saved the page shows only its last
// four characters, with Replace and Remove. Everything generated for the
// workspace is billed by OpenRouter to that key, never as Fullpilot credits.

import React from "react";
import toast from "react-hot-toast";
import { ExternalLinkIcon, KeyRoundIcon, Loader2Icon } from "@/components/icons";
import { NoAccess } from "@/components/layout/NoAccess";
import { usePermission } from "@/hooks/usePermission";
import { useConfirm } from "@/hooks/context/confirm";
import { TextInput } from "@/components/ui/field";
import ModelPicker from "@/components/app/ai/ModelPicker";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { useAIModels, useAISettings, useRemoveAIKey, useUpdateAISettings } from "@/lib/api/hooks/app/organizations/useAISettings";
import { OPENROUTER_KEYS_URL } from "@/lib/api/models/app/organizations/AISettings";
import { Row, Section, SectionShell } from "../_components/SectionShell";

export default function AISettingsPage() {
    const canManage = usePermission("MANAGE_SETTINGS");
    if (!canManage) return <NoAccess feature="AI" permissionLabel="Manage settings" />;
    return <AISettings />;
}

function AISettings() {
    const settings = useAISettings();
    const update = useUpdateAISettings();
    const remove = useRemoveAIKey();
    const confirm = useConfirm();

    const hasKey = !!settings.data?.has_key;
    const models = useAIModels(hasKey);

    // Key entry is shown when there is no key yet, or after Replace.
    const [editing, setEditing] = React.useState(false);
    const [key, setKey] = React.useState("");
    const entering = editing || (settings.isSuccess && !hasKey);

    const saveKey = async () => {
        const trimmed = key.trim();
        if (!trimmed || update.isPending) return;
        try {
            await update.mutateAsync({ api_key: trimmed });
            setKey("");
            setEditing(false);
            toast.success("OpenRouter key saved. Campaigns paused for it resume on their own.");
        } catch (e) {
            toast.error(buildError(e as AppError));
        }
    };

    const removeKey = () => {
        confirm?.show(
            "Remove the OpenRouter key? Every AI feature stops working for this workspace, and a campaign that reaches an AI block pauses until a key is added again.",
            async () => {
                try {
                    await remove.mutateAsync();
                    toast.success("OpenRouter key removed");
                } catch (e) {
                    toast.error(buildError(e as AppError));
                }
            },
        );
    };

    const setModel = async (model: string) => {
        try {
            await update.mutateAsync({ model });
            toast.success(model ? "Default model updated" : "Default model reset");
        } catch (e) {
            toast.error(buildError(e as AppError));
        }
    };

    return (
        <SectionShell
            title="AI"
            description="Every AI feature this workspace uses runs on its own OpenRouter key: AI blocks in campaign copy, the writing assistant, reply and compose drafts. OpenRouter bills the workspace directly, so nothing here spends Fullpilot credits."
        >
            <Section
                eyebrow="OpenRouter key"
                description="Create or copy a key on OpenRouter, paste it here, and it is stored encrypted. It is never shown again in full."
                actions={
                    <a
                        href={OPENROUTER_KEYS_URL}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 text-[12px] font-medium text-sky-600 hover:text-sky-700"
                    >
                        openrouter.ai/keys
                        <ExternalLinkIcon className="w-3 h-3" />
                    </a>
                }
            >
                {settings.isPending ? (
                    <div className="h-9 bg-slate-100 animate-pulse" />
                ) : entering ? (
                    <div className="flex flex-col sm:flex-row gap-2">
                        <TextInput
                            value={key}
                            onChange={setKey}
                            type="password"
                            autoComplete="off"
                            placeholder="sk-or-v1-…"
                            className="flex-1 h-8"
                            onKeyDown={(e) => {
                                if (e.key === "Enter") void saveKey();
                            }}
                            autoFocus={editing}
                        />
                        <div className="flex items-center gap-1.5 shrink-0">
                            {hasKey && (
                                <button
                                    type="button"
                                    onClick={() => {
                                        setEditing(false);
                                        setKey("");
                                    }}
                                    className="h-8 px-3 border border-slate-200 bg-white text-[12.5px] font-medium text-slate-700 transition-colors hover:border-slate-300"
                                >
                                    Cancel
                                </button>
                            )}
                            <button
                                type="button"
                                onClick={() => void saveKey()}
                                disabled={!key.trim() || update.isPending}
                                className="h-8 px-3.5 inline-flex items-center gap-1.5 bg-sky-600 text-[12.5px] font-medium text-white transition-colors hover:bg-sky-700 disabled:opacity-50"
                            >
                                {update.isPending && <Loader2Icon className="w-3.5 h-3.5 animate-spin" />}
                                {hasKey ? "Save new key" : "Save key"}
                            </button>
                        </div>
                    </div>
                ) : (
                    <Row
                        label={
                            <span className="inline-flex items-center gap-2">
                                <KeyRoundIcon className="w-3.5 h-3.5 text-slate-400" />
                                <span>Key ending in •••• {settings.data?.key_last4}</span>
                            </span>
                        }
                        description="Set and encrypted. Replace it to rotate, or remove it to turn AI features off for this workspace."
                    >
                        <div className="flex items-center gap-1.5">
                            <button
                                type="button"
                                onClick={() => setEditing(true)}
                                className="h-7 px-2.5 border border-slate-200 bg-white text-[12px] font-medium text-slate-700 transition-colors hover:border-slate-300 hover:text-slate-900"
                            >
                                Replace
                            </button>
                            <button
                                type="button"
                                onClick={removeKey}
                                disabled={remove.isPending}
                                className="h-7 px-2.5 border border-slate-200 bg-white text-[12px] font-medium text-slate-700 transition-colors hover:border-rose-300 hover:text-rose-700 disabled:opacity-60"
                            >
                                Remove
                            </button>
                        </div>
                    </Row>
                )}
            </Section>

            <Section
                eyebrow="Default model"
                description="The model AI blocks and the writing assistant use unless a block picks its own. The list is everything your key can route to on OpenRouter, with prices per million tokens."
            >
                {hasKey ? (
                    <div className="max-w-md">
                        <ModelPicker
                            value={settings.data?.model ?? ""}
                            onChange={(m) => void setModel(m)}
                            models={models.data ?? []}
                            loading={models.isPending}
                            disabled={update.isPending}
                            fullWidth
                        />
                        {models.isError && (
                            <p className="mt-1.5 text-[11.5px] text-rose-600">{buildError(models.error as unknown as AppError)}</p>
                        )}
                    </div>
                ) : (
                    <p className="text-[12px] text-slate-500">Save a key first: the model list comes from OpenRouter.</p>
                )}
            </Section>

            <Section eyebrow="How it is used">
                <ul className="space-y-1.5 text-[12px] leading-relaxed text-slate-600 list-disc pl-4">
                    <li>An AI block in a campaign email is written for each recipient at send time with this key. A campaign that carries one cannot start without a key, and one already running pauses until a key is saved.</li>
                    <li>Each AI block may pick its own model; otherwise it follows the default above.</li>
                    <li>Reply classification and platform features keep running on Fullpilot's own model and do not use this key.</li>
                </ul>
            </Section>
        </SectionShell>
    );
}
