import type WorkspaceSendMethods from "@/lib/api/models/app/analytics/SendMethods";
import Request from "../../Request";

// GET /analytics/send-methods?period=7d|30d|90d — a bare object (no envelope).
export default async function getSendMethods(period: string = "7d"): Promise<WorkspaceSendMethods> {
    return await Request<WorkspaceSendMethods>({
        method: "GET",
        url: `/analytics/send-methods?period=${encodeURIComponent(period)}`,
        authorization: true,
    })
}
