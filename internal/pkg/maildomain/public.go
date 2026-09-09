// Package maildomain holds facts about mail domains that several layers need
// to agree on. It depends on nothing inside the repository so the models,
// the config, the DNS checker, the sweep and the advisor can all import it.
package maildomain

import (
	"strings"

	"golang.org/x/net/publicsuffix"
)

// publicMailDomains are consumer mail providers whose DNS the customer does
// not own. SPF, DKIM and DMARC on gmail.com are Google's business, so a
// sending-domain authentication verdict is meaningless for a mailbox there:
// nothing the owner can do at a registrar changes it, and the provider signs
// its own outbound mail. Every check, gate and finding about domain
// authentication skips these.
var publicMailDomains = map[string]struct{}{
	// Google
	"gmail.com":      {},
	"googlemail.com": {},
	// Microsoft
	"outlook.com": {},
	"hotmail.com": {},
	"live.com":    {},
	"msn.com":     {},
	// Yahoo
	"yahoo.com":      {},
	"ymail.com":      {},
	"rocketmail.com": {},
	// Apple
	"icloud.com": {},
	"me.com":     {},
	"mac.com":    {},
	// AOL
	"aol.com": {},
	// Proton
	"proton.me":      {},
	"protonmail.com": {},
	"protonmail.ch":  {},
	"pm.me":          {},
	// GMX / mail.com (1&1)
	"gmx.com":  {},
	"gmx.net":  {},
	"gmx.de":   {},
	"gmx.at":   {},
	"gmx.ch":   {},
	"mail.com": {},
	// Zoho
	"zoho.com":     {},
	"zohomail.com": {},
	// Yandex
	"yandex.com": {},
	"yandex.ru":  {},
	// Others
	"fastmail.com": {},
	"hey.com":      {},
	"tutanota.com": {},
	"tuta.io":      {},
}

// publicMailLabels are registrable-domain labels that are the same consumer
// provider under every country suffix: hotmail.co.uk, outlook.fr, yahoo.co.jp,
// live.de, gmx.co.uk, yandex.kz. Matching on the label keeps the list from
// having to enumerate every country each provider serves.
var publicMailLabels = map[string]struct{}{
	"hotmail": {},
	"outlook": {},
	"live":    {},
	"yahoo":   {},
	"ymail":   {},
	"gmx":     {},
	"yandex":  {},
	"aol":     {},
}

// PublicMailDomain reports whether domain belongs to a consumer mail provider
// rather than to the customer. Matching is case-insensitive, tolerates a
// trailing dot and surrounding whitespace, and takes "you@gmail.com" as well
// as "gmail.com".
func PublicMailDomain(domain string) bool {
	d := strings.ToLower(strings.TrimSpace(domain))
	if at := strings.LastIndex(d, "@"); at >= 0 {
		d = d[at+1:]
	}
	d = strings.TrimSuffix(d, ".")
	if d == "" {
		return false
	}
	if _, ok := publicMailDomains[d]; ok {
		return true
	}
	// The label rule only applies to a registrable domain (eTLD+1), so
	// "hotmail.co.uk" matches while a customer's own "outlook.example.com"
	// does not: its registrable domain is example.com.
	registrable, err := publicsuffix.EffectiveTLDPlusOne(d)
	if err != nil || registrable != d {
		return false
	}
	label, _, found := strings.Cut(d, ".")
	if !found {
		return false
	}
	_, ok := publicMailLabels[label]
	return ok
}
