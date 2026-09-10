import { useQuery } from "@tanstack/react-query";
import getCampaignSendMethods from "@/lib/api/client/app/analytics/getCampaignSendMethods";

export default function useCampaignSendMethods(id: string) {
    return useQuery({
        queryKey: ["analytics", "campaigns", id, "send-methods"],
        queryFn: () => getCampaignSendMethods(id),
        enabled: !!id,
    })
}
