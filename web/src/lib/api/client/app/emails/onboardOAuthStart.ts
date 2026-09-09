import Request from "../../Request";

export interface OAuthStartResponse {
    url: string;
    state: string;
    // Set when the provider will send the consent window to localhost
    // (desktop-type OAuth client). The user pastes the landing address back.
    manual_redirect?: boolean;
}

export default async function onboardOAuthStart(provider: "gmail" | "outlook"): Promise<OAuthStartResponse> {
    return await Request<OAuthStartResponse>({
        method: "POST",
        url: `/emails/onboarding/oauth/start`,
        data: { provider },
        authorization: true,
    });
}
