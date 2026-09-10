import type { DateRange } from "./CampaignAnalytics"

// One row of a send-method breakdown: how the sends made one way performed.
// send_method is <transport>_<provider>_<deployment>, for example
// smtp_google_cloud_run_worker or api_microsoft_cloud_vm; the empty string
// groups sends confirmed before the method was recorded.
export interface SendMethodStats {
    send_method: string
    // "text" (text/plain only) or "html" (multipart); empty before formats were recorded.
    send_format: string
    sent: number
    delivered: number
    opened: number
    replied: number
    bounced: number
    delivery_rate: number
    open_rate: number
    reply_rate: number
    bounce_rate: number
}

// GET /campaigns/:id/analytics/send-methods
export interface CampaignSendMethods {
    campaign_id: string
    methods: SendMethodStats[]
}

// GET /analytics/send-methods?period=7d|30d|90d
export default interface WorkspaceSendMethods {
    period: string
    date_range: DateRange
    methods: SendMethodStats[]
}
