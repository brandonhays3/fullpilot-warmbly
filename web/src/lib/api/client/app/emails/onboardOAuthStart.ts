import Request from "../../Request";

export interface OAuthStartResponse {
    url: string;
    state: string;
    // Set when the provider will send the consent window to localhost
    // (desktop-type OAuth client). The user pastes the landing address back.
    manual_redirect?: boolean;
}

// client picks an alternate OAuth client for the provider. "google_desktop" is
// the desktop-type Gmail client; it redirects to localhost and the user pastes
// the landing address back (manual_redirect in the response).
export type OAuthClientChoice = "default" | "google_desktop";

export default async function onboardOAuthStart(
    provider: "gmail" | "outlook",
    client: OAuthClientChoice = "default",
): Promise<OAuthStartResponse> {
    return await Request<OAuthStartResponse>({
        method: "POST",
        url: `/emails/onboarding/oauth/start`,
        data: { provider, client },
        authorization: true,
    });
}
