import { useQuery } from "@tanstack/react-query";
import getInstantlyWarmup from "@/lib/api/client/app/emails/getInstantlyWarmup";

// Instantly warmup status for a mailbox. Keyed under ["analytics", "accounts",
// id] so the existing warmup/account realtime branch refreshes it too. The
// backend answers this from Instantly's API, so it is kept fresh for a minute
// rather than refetched on every focus.
export default function useInstantlyWarmup(id: string, enabled = true) {
    return useQuery({
        queryKey: ["analytics", "accounts", id, "instantly"],
        queryFn: () => getInstantlyWarmup(id),
        enabled: !!id && enabled,
        staleTime: 60_000,
    });
}
