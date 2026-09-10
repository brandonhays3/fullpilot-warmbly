import { MailIcon } from "@/components/icons";

/** Provider mark for a mailbox: Google's G, Microsoft's squares, or a plain
 *  envelope for SMTP/IMAP. Sized by className like any icon. */
export function ProviderLogo({ provider, className = "w-3.5 h-3.5" }: { provider?: string; className?: string }) {
    if (provider === "gmail") {
        return (
            <svg viewBox="0 0 48 48" className={className} aria-hidden>
                <path fill="#EA4335" d="M24 9.5c3.5 0 6.6 1.2 9.1 3.6l6.8-6.8C35.7 2.5 30.2 0 24 0 14.6 0 6.5 5.4 2.6 13.3l7.9 6.1C12.4 13.7 17.7 9.5 24 9.5z" />
                <path fill="#4285F4" d="M46.5 24.5c0-1.6-.1-3.1-.4-4.5H24v9h12.7c-.6 2.9-2.2 5.4-4.7 7.1l7.6 5.9c4.4-4.1 6.9-10.1 6.9-17.5z" />
                <path fill="#FBBC05" d="M10.5 28.6A14.5 14.5 0 0 1 9.5 24c0-1.6.3-3.2.8-4.6l-7.9-6.1A24 24 0 0 0 0 24c0 3.9.9 7.5 2.6 10.7l7.9-6.1z" />
                <path fill="#34A853" d="M24 48c6.5 0 11.9-2.1 15.9-5.8l-7.6-5.9c-2.1 1.4-4.9 2.3-8.3 2.3-6.3 0-11.6-4.2-13.5-9.9l-7.9 6.1C6.5 42.6 14.6 48 24 48z" />
            </svg>
        );
    }
    if (provider === "outlook") {
        return (
            <svg viewBox="0 0 23 23" className={className} aria-hidden>
                <rect x="1" y="1" width="10" height="10" fill="#F25022" />
                <rect x="12" y="1" width="10" height="10" fill="#7FBA00" />
                <rect x="1" y="12" width="10" height="10" fill="#00A4EF" />
                <rect x="12" y="12" width="10" height="10" fill="#FFB900" />
            </svg>
        );
    }
    return <MailIcon className={className} />;
}

export const PROVIDER_LABELS: Record<string, string> = {
    gmail: "Google",
    outlook: "Microsoft",
    smtp_imap: "SMTP / IMAP",
};
