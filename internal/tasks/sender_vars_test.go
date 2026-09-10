package tasks

import (
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

func TestSenderVarsSplitsNameOnFirstSpace(t *testing.T) {
	account := &models.Email{Name: "  Ana Maria Silva ", Email: "ana@acme.com"}
	v := senderVars(account, " Acme Inc ")
	want := map[string]string{
		SenderFirstNameVar: "Ana",
		SenderLastNameVar:  "Maria Silva",
		SenderNameVar:      "Ana Maria Silva",
		SenderEmailVar:     "ana@acme.com",
		SenderCompanyVar:   "Acme Inc",
	}
	for k, w := range want {
		if v[k] != w {
			t.Fatalf("%s = %q, want %q", k, v[k], w)
		}
	}

	// A single-word name is all first name; no mailbox leaves every field blank.
	if v := senderVars(&models.Email{Name: "Ana"}, ""); v[SenderFirstNameVar] != "Ana" || v[SenderLastNameVar] != "" {
		t.Fatalf("single-word name split wrong: %v", v)
	}
	if v := senderVars(nil, "Acme"); v[SenderNameVar] != "" || v[SenderCompanyVar] != "Acme" {
		t.Fatalf("nil account: %v", v)
	}
}

func TestSenderFieldsRenderAndOrFallbackValidates(t *testing.T) {
	extra := senderVars(&models.Email{Name: "Ana Silva", Email: "ana@acme.com"}, "Acme")
	contact := models.Contact{LastName: "Rivera"}

	tmpl := `Hi {{or .FirstName "there"}} {{.LastName}}, {{.SenderFirstName}} from {{.SenderCompany}} ({{.SenderEmail}})`
	if err := TemplateError(tmpl); err != nil {
		t.Fatalf("or-fallback template rejected: %v", err)
	}
	got := RenderTemplateWith(tmpl, contact, extra)
	if want := "Hi there Rivera, Ana from Acme (ana@acme.com)"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// The fallback yields to a real value, and an escaped quote survives.
	got = RenderTemplateWith(`{{or .FirstName "say \"hi\""}}`, models.Contact{FirstName: "Alex"}, nil)
	if got != "Alex" {
		t.Fatalf("fallback overrode a value: %q", got)
	}
	if got = RenderTemplateWith(`{{or .FirstName "say \"hi\""}}`, models.Contact{}, nil); got != `say "hi"` {
		t.Fatalf("escaped quote in fallback: %q", got)
	}

	// The preview carries the sender fields and never reports them unresolved.
	p := previewTemplatesExtra("{{.SenderName}}", "<p>{{.SenderLastName}}</p>", "", contact, extra)
	if p.Subject != "Ana Silva" || p.BodyHTML != "<p>Silva</p>" || len(p.Unresolved) != 0 || len(p.Errors) != 0 {
		t.Fatalf("preview with sender fields: %+v", p)
	}
}
