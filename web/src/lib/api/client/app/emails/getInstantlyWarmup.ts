import type InstantlyWarmup from "@/lib/api/models/app/emails/InstantlyWarmup";
import Request from "../../Request";

// Reads the Instantly side of a mailbox's warmup: whether the integration is
// on, whether this mailbox is warmed there, and the status Instantly reports.
export default async function getInstantlyWarmup(id: string): Promise<InstantlyWarmup> {
    return await Request<InstantlyWarmup>({
        method: "GET",
        url: `/emails/${id}/warmup/instantly`,
        authorization: true,
    });
}
