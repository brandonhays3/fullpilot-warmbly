// Non-component helpers for the step preview's context (who it renders for,
// which mailbox signs it). Kept apart from PreviewControls.tsx so that file
// exports only components and stays fast-refresh friendly.

import React from "react";
import type Contact from "@/lib/api/models/app/contacts/Contact";
import type Inbox from "@/lib/api/models/app/emails/Inbox";
import useCampaign from "@/lib/api/hooks/app/campaigns/useCampaign";
import useCampaignSenders from "@/lib/api/hooks/app/campaigns/useCampaignSenders";
import useEmails from "@/lib/api/hooks/app/emails/useEmails";
import { SAMPLE } from "@/lib/templateVars";

export const SAMPLE_CONTACT_LABEL = `${SAMPLE.FirstName} ${SAMPLE.LastName} (sample)`;

export function contactLabel(c: Contact): string {
    const name = `${c.first_name ?? ""} ${c.last_name ?? ""}`.trim();
    return name || c.email;
}

// Resolves the mailboxes the campaign will send from to mailbox rows, so
// pickers can label a sender by address. Mirrors the server's sender
// resolution: an explicit pool lists its enabled senders (a paused one never
// sends this step); the default tag strategy takes every mailbox carrying one
// of the campaign's tags, and with no tags at all, every mailbox.
export function useCampaignSenderInboxes(campaignId: string): { inboxes: Inbox[]; loading: boolean } {
    const campaign = useCampaign(campaignId);
    const explicit = campaign.data?.sender_strategy === "explicit";
    const senders = useCampaignSenders(campaignId, !!campaignId && explicit);
    const emails = useEmails({ query: "", tag: "", limit: 200, enabled: !!campaignId });
    const ids = new Set((senders.data ?? []).filter((s) => s.enabled).map((s) => s.email_account_id));
    const tagIds = new Set(campaign.data?.email_tags ?? []);
    const inboxes = React.useMemo(() => {
        if (!campaign.data) return [];
        if (explicit) return emails.emails.filter((e) => ids.has(e.id));
        if (tagIds.size === 0) return emails.emails;
        return emails.emails.filter((e) => (e.tags ?? []).some((t) => tagIds.has(t)));
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [campaign.data, explicit, emails.emails, [...ids].join(","), [...tagIds].join(",")]);

    // A workspace can hold more mailboxes than one page, and a sender sitting on
    // a later one would otherwise be missing from the picker. An explicit pool
    // knows how many it is waiting for; a tag match can only be sure once every
    // page is in.
    const incomplete = explicit ? inboxes.length < ids.size : true;
    const { hasNextPage, isFetchingNextPage, fetchNextPage } = emails;
    React.useEffect(() => {
        if (incomplete && hasNextPage && !isFetchingNextPage) fetchNextPage();
    }, [incomplete, hasNextPage, isFetchingNextPage, fetchNextPage]);

    return {
        inboxes,
        loading: campaign.isLoading || senders.isLoading || emails.isLoading || (incomplete && !!hasNextPage),
    };
}
