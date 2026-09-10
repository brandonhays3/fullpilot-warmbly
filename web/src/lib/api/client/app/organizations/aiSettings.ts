import type { AIModel, AISettings, UpdateAISettingsRequest } from "@/lib/api/models/app/organizations/AISettings";
import Request from "../../Request";

// GET /organization/current/ai: whether a key is set (and its last four), plus
// the workspace default model.
export async function getAISettings(): Promise<AISettings> {
    return await Request<AISettings>({
        method: "GET",
        url: "/organization/current/ai",
        authorization: true,
    });
}

// PUT /organization/current/ai: replace the key and/or the default model.
// Retry-safe: the same body lands the same state.
export async function updateAISettings(body: UpdateAISettingsRequest): Promise<AISettings> {
    return await Request<AISettings>({
        method: "PUT",
        url: "/organization/current/ai",
        data: body,
        authorization: true,
    });
}

// DELETE /organization/current/ai: remove the key; the model choice stays.
export async function removeAIKey(): Promise<AISettings> {
    return await Request<AISettings>({
        method: "DELETE",
        url: "/organization/current/ai",
        authorization: true,
    });
}

// GET /organization/current/ai/models: every model the workspace key can route
// to, with pricing. 409 ai_key_missing without a key.
export async function getAIModels(): Promise<AIModel[]> {
    const res = await Request<{ data: AIModel[] }>({
        method: "GET",
        url: "/organization/current/ai/models",
        authorization: true,
    });
    return res?.data ?? [];
}
