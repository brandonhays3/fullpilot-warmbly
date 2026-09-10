import { useQuery } from "@tanstack/react-query";
import getSendMethods from "@/lib/api/client/app/analytics/getSendMethods";

export default function useSendMethods(period: string = "7d") {
    return useQuery({
        queryKey: ["analytics", "send-methods", period],
        queryFn: () => getSendMethods(period),
    })
}
