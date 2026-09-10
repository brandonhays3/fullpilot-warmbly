import type { CampaignSendMethods } from "@/lib/api/models/app/analytics/SendMethods";
import Request from "../../Request";

// GET /campaigns/:id/analytics/send-methods — a bare object (no envelope).
export default async function getCampaignSendMethods(id: string): Promise<CampaignSendMethods> {
    return await Request<CampaignSendMethods>({
        method: "GET",
        url: `/campaigns/${id}/analytics/send-methods`,
        authorization: true,
    })
}
