import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getAIModels, getAISettings, removeAIKey, updateAISettings } from "@/lib/api/client/app/organizations/aiSettings";
import type { AISettings, UpdateAISettingsRequest } from "@/lib/api/models/app/organizations/AISettings";

// Keyed under ["organizations", "current"] so the audit spine's `settings`
// entry refreshes it when a teammate changes the key or model.
export const AI_SETTINGS_KEY = ["organizations", "current", "ai"] as const;
export const AI_MODELS_KEY = ["organizations", "current", "ai", "models"] as const;

export function useAISettings(enabled = true) {
    return useQuery<AISettings>({
        queryKey: AI_SETTINGS_KEY,
        queryFn: getAISettings,
        enabled,
        staleTime: 60_000,
    });
}

export function useUpdateAISettings() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (body: UpdateAISettingsRequest) => updateAISettings(body),
        onSuccess: (settings, body) => {
            qc.setQueryData(AI_SETTINGS_KEY, settings);
            // A new key unlocks the model list and resumes campaigns parked on
            // paused_ai_key, so both views move with it.
            if (body.api_key) {
                void qc.invalidateQueries({ queryKey: AI_MODELS_KEY });
                void qc.invalidateQueries({ queryKey: ["campaigns"] });
            }
        },
    });
}

export function useRemoveAIKey() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: removeAIKey,
        onSuccess: (settings) => {
            qc.setQueryData(AI_SETTINGS_KEY, settings);
            qc.removeQueries({ queryKey: AI_MODELS_KEY });
        },
    });
}

// The model catalog is the same for every workspace and the backend caches it
// for ten minutes, so the client keeps it for as long.
export function useAIModels(enabled: boolean) {
    return useQuery({
        queryKey: AI_MODELS_KEY,
        queryFn: getAIModels,
        enabled,
        staleTime: 10 * 60_000,
        retry: false,
    });
}
