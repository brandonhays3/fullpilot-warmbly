package maildomain

import "testing"

func TestPublicMailDomain(t *testing.T) {
	public := []string{
		"gmail.com", "GMAIL.COM", "gmail.com.", " googlemail.com ",
		"you@gmail.com", "Someone@Outlook.com",
		"outlook.com", "hotmail.com", "live.com", "msn.com",
		"hotmail.co.uk", "outlook.fr", "live.de", "yahoo.co.jp",
		"yahoo.com", "icloud.com", "me.com", "aol.com",
		"proton.me", "protonmail.com", "gmx.de", "gmx.co.uk", "mail.com",
		"zoho.com", "yandex.ru", "yandex.kz",
	}
	for _, d := range public {
		if !PublicMailDomain(d) {
			t.Errorf("%q should be a public mail domain", d)
		}
	}

	private := []string{
		"", "@", "example.com", "acme.io", "mail.acme.io",
		// A customer's own subdomain that happens to reuse a provider label.
		"outlook.example.com", "gmail.acme.io", "yahoo.corp.example.org",
		// Bare labels and suffixes are not domains anyone sends from.
		"gmail", "co.uk",
	}
	for _, d := range private {
		if PublicMailDomain(d) {
			t.Errorf("%q should not be a public mail domain", d)
		}
	}
}
