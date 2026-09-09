package instantly

// TODO: once config.WarmupDefaults(provider) lands in
// internal/config/constants.go, derive these from it instead of keeping a
// private copy here. The numbers must agree with that table.

// Warmbly provider strings as stored in email_accounts.provider.
const (
	providerGmail    = "gmail"
	providerOutlook  = "outlook"
	providerSMTPIMAP = "smtp_imap"
)

// WarmupDefaults is the per-provider warmup profile applied to every mailbox
// Warmbly enrolls in Instantly: +2 a day, 65% replies, read emulation on,
// every day of the week, 63% opens, 100% spam rescue, 34% marked important.
// The daily ceiling is the only number that differs: 30 for Gmail and custom
// IMAP/SMTP, 16 for Outlook.
func WarmupDefaults(provider string) WarmupSettings {
	limit := 30
	if provider == providerOutlook {
		limit = 16
	}
	return WarmupSettings{
		Limit:     limit,
		Increment: "2",
		ReplyRate: 0.65,
		Advanced: &WarmupAdvanced{
			OpenRate:      0.63,
			ImportantRate: 0.34,
			ReadEmulation: true,
			SpamSaveRate:  1.0,
			WeekdayOnly:   false,
		},
	}
}

// ProviderCodeFor maps a Warmbly provider to Instantly's provider_code.
func ProviderCodeFor(provider string) int {
	switch provider {
	case providerGmail:
		return ProviderCodeGoogle
	case providerOutlook:
		return ProviderCodeMicrosoft
	case providerSMTPIMAP:
		return ProviderCodeCustomIMAPSMTP
	}
	return ProviderCodeCustomIMAPSMTP
}
