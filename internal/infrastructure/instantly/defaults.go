package instantly

import (
	"strconv"

	"github.com/warmbly/warmbly/internal/config"
)

// Warmbly provider strings as stored in email_accounts.provider.
const (
	providerGmail    = "gmail"
	providerOutlook  = "outlook"
	providerSMTPIMAP = "smtp_imap"
)

// WarmupDefaults is the per-provider warmup profile applied to every mailbox
// Fullpilot enrolls in Instantly, derived from config.WarmupDefaults. Rates
// are sent as fractions, which is what Instantly's API examples use.
func WarmupDefaults(provider string) WarmupSettings {
	d := config.WarmupDefaults(provider)
	return WarmupSettings{
		Limit:     d.DailyLimit,
		Increment: strconv.Itoa(d.IncreasePerDay),
		ReplyRate: float64(d.ReplyRate) / 100,
		Advanced: &WarmupAdvanced{
			OpenRate:      float64(d.OpenRate) / 100,
			ImportantRate: float64(d.MarkImportant) / 100,
			ReadEmulation: d.ReadEmulation,
			SpamSaveRate:  float64(d.SpamProtection) / 100,
			WeekdayOnly:   d.WeekdaysOnly,
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
