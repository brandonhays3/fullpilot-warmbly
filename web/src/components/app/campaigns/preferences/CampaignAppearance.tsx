// Standard campaign settings, split into the sections rendered on the
// single-scroll preferences page: identity, sending accounts, and the
// deliverability toggles. Each export returns ONLY its controls — the page's
// SettingsSection wrapper supplies the heading, icon and anchor.
// On-theme: slate/sky, rounded-md, 12.5px base.

import type Campaign from "@/lib/api/models/app/campaigns/Campaign";
import { Label, TextInput } from "@/components/ui/field";
import SenderSelector from "./SenderSelector";
import { SettingRow, Toggle } from "./components/CampaignPreferenceBoolBox";
import { SelectMenu, type SelectOption } from "@/components/ui/select-menu";
import { useOutreachSettings } from "@/lib/api/hooks/app/outreach/useOutreachSettings";

const UNSUB_MODES: SelectOption[] = [
    { value: "inherit", label: "Workspace default" },
    { value: "text", label: "Reply to opt out (text line)" },
    { value: "link", label: "Unsubscribe link" },
    { value: "off", label: "Nothing" },
];

type SetCampaign = React.Dispatch<React.SetStateAction<Campaign>>;

/** General — campaign name + description. */
export function GeneralSection({
    campaign,
    newCampaign,
    setNewCampaign,
}: {
    campaign: Campaign;
    newCampaign: Campaign;
    setNewCampaign: SetCampaign;
}) {
    return (
        <div className="space-y-4">
            <div>
                <Label>Campaign name</Label>
                <TextInput
                    value={newCampaign.name}
                    placeholder={campaign.name}
                    onChange={(v) => setNewCampaign((bef) => ({ ...bef, name: v }))}
                    className="w-full max-w-[420px]"
                />
            </div>
            <div>
                <Label>Description</Label>
                <TextInput
                    value={newCampaign.description}
                    placeholder={campaign.description || "Optional — what this targets"}
                    onChange={(v) => setNewCampaign((bef) => ({ ...bef, description: v }))}
                    className="w-full max-w-[420px]"
                />
            </div>
        </div>
    );
}

/** Sending accounts — the unified tag/mailbox picker. The daily cap is a
 * mailbox setting, so there is no per-campaign limit here. */
export function SendingAccountsSection({
    newCampaign,
    setNewCampaign,
    explicitAccounts,
    setExplicitAccounts,
}: {
    newCampaign: Campaign;
    setNewCampaign: SetCampaign;
    explicitAccounts: string[];
    setExplicitAccounts: React.Dispatch<React.SetStateAction<string[]>>;
}) {
    return (
        <div className="space-y-4">
            <div>
                <Label>Sending accounts</Label>
                <SenderSelector
                    selectedTags={newCampaign.email_tags}
                    onTagsChange={(next) => setNewCampaign((bef) => ({ ...bef, email_tags: next }))}
                    selectedAccounts={explicitAccounts}
                    onAccountsChange={setExplicitAccounts}
                />
                <p className="text-[11px] text-slate-400 mt-1.5">
                    Pick tags, specific mailboxes, or both. Volume is split evenly across the resolved pool, and each
                    mailbox sends up to its own daily cap, set on the mailbox. Leave empty to send from every active
                    mailbox.
                </p>
            </div>
        </div>
    );
}

/** Deliverability — the per-campaign send/tracking toggles. */
export function DeliverabilitySection({
    newCampaign,
    setNewCampaign,
}: {
    newCampaign: Campaign;
    setNewCampaign: SetCampaign;
}) {
    // The warning below is only true when the opt-out this campaign actually
    // sends is a link, so "inherit" has to be resolved against the workspace
    // default rather than assumed.
    const { data: outreach } = useOutreachSettings();
    const mode = newCampaign.unsubscribe_mode ?? "inherit";
    const effectiveMode = mode === "inherit" || !mode ? (outreach?.unsubscribe?.mode ?? "off") : mode;
    const plainTextLinkOptOut = newCampaign.text_only && effectiveMode === "link";

    return (
        <div className="space-y-5">
            <SettingRow
                title="Stop on reply"
                description="Pause follow-ups for a contact once they respond."
                control={
                    <Toggle
                        id="campaign-pref-stop-on-reply"
                        value={newCampaign.stop_on_reply}
                        onChange={(v) => setNewCampaign((bef) => ({ ...bef, stop_on_reply: v }))}
                    />
                }
            />
            <SettingRow
                title="Plain text only"
                description="Send as simple text for the best deliverability (disables tracking)."
                control={
                    <Toggle
                        id="campaign-pref-text"
                        value={newCampaign.text_only}
                        onChange={(v) => setNewCampaign((bef) => ({ ...bef, text_only: v }))}
                    />
                }
            />
            <SettingRow
                title="Open tracking"
                description="Track email opens, but may slightly reduce deliverability."
                control={
                    <Toggle
                        id="campaign-pref-open-tracking"
                        value={newCampaign.open_tracking}
                        disabled={newCampaign.text_only}
                        onChange={(v) => setNewCampaign((bef) => ({ ...bef, open_tracking: v }))}
                    />
                }
            />
            <SettingRow
                title="Link tracking"
                description="Track clicks on links to measure engagement (click-through rate). Each link is tracked on its own, so a contact's activity shows exactly which link was clicked."
                control={
                    <Toggle
                        id="campaign-pref-link-tracking"
                        value={newCampaign.link_tracking}
                        disabled={newCampaign.text_only}
                        onChange={(v) => setNewCampaign((bef) => ({ ...bef, link_tracking: v }))}
                    />
                }
            />
            <SettingRow
                title="UTM parameters"
                description="Tag every link with utm_source, utm_medium, utm_campaign and utm_content (the link's own text) so clicks show up attributed in your web analytics. Links that already carry a UTM value keep it."
                control={
                    <Toggle
                        id="campaign-pref-utm-tracking"
                        value={newCampaign.utm_tracking}
                        disabled={newCampaign.text_only}
                        onChange={(v) => setNewCampaign((bef) => ({ ...bef, utm_tracking: v }))}
                    />
                }
            />
            {newCampaign.utm_tracking && !newCampaign.text_only && (
                <UTMFields campaign={newCampaign} setNewCampaign={setNewCampaign} />
            )}
            <SettingRow
                title="Opt-out line"
                description={
                    <>
                        What this campaign appends after the signature, if anything. Nothing is the default: a reply
                        that asks to stop is detected and honoured automatically either way. Reply to opt out adds a
                        sentence that reads as a personal email; a link is for lists that need one. The workspace
                        default is set under Settings &gt; Sending.
                        {plainTextLinkOptOut && (
                            <span className="mt-1 block text-amber-700">
                                This campaign sends plain text only, where a link has nowhere to hide its address: the
                                recipient reads the full unsubscribe URL. Prefer the reply line here.
                            </span>
                        )}
                    </>
                }
                control={
                    <SelectMenu
                        value={newCampaign.unsubscribe_mode ?? "inherit"}
                        onChange={(v) =>
                            setNewCampaign((bef) => ({ ...bef, unsubscribe_mode: v as Campaign["unsubscribe_mode"] }))
                        }
                        options={UNSUB_MODES}
                        aria-label="Opt-out line"
                        minWidth={240}
                        align="end"
                    />
                }
            />
        </div>
    );
}

/** The three campaign-level UTM values; utm_content is per link and not editable. */
export function UTMFields({
    campaign,
    setNewCampaign,
}: {
    campaign: Campaign;
    setNewCampaign: SetCampaign;
}) {
    return (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pl-0">
            <div>
                <Label>utm_source</Label>
                <TextInput
                    value={campaign.utm_source}
                    placeholder="fullpilot"
                    onChange={(v) => setNewCampaign((bef) => ({ ...bef, utm_source: v }))}
                    className="w-full"
                />
            </div>
            <div>
                <Label>utm_medium</Label>
                <TextInput
                    value={campaign.utm_medium}
                    placeholder="email"
                    onChange={(v) => setNewCampaign((bef) => ({ ...bef, utm_medium: v }))}
                    className="w-full"
                />
            </div>
            <div>
                <Label>utm_campaign</Label>
                <TextInput
                    value={campaign.utm_campaign}
                    placeholder={utmSlug(campaign.name) || "campaign"}
                    onChange={(v) => setNewCampaign((bef) => ({ ...bef, utm_campaign: v }))}
                    className="w-full"
                />
            </div>
            <p className="sm:col-span-3 text-[11px] text-slate-400 -mt-1">
                Leave a field empty to use the placeholder default. utm_content is set per link from its text
                (for example <span className="font-mono">pricing</span>), so each link is attributed on its own.
            </p>
        </div>
    );
}

// Mirrors the backend's slug: lowercase words joined with underscores.
function utmSlug(s: string): string {
    return s
        .toLowerCase()
        .trim()
        .split(/[^\p{L}\p{N}]+/u)
        .filter(Boolean)
        .join("_")
        .slice(0, 64);
}
