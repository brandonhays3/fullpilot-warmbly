package tasks

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// Sender merge fields: the mailbox a send goes out through and the workspace
// it belongs to. Resolved per send, like the unsubscribe link, so the same
// step reads right from every mailbox in the pool. Keep the names in sync
// with SENDER_VARS in web/src/lib/templateVars.ts.
const (
	SenderFirstNameVar = "SenderFirstName"
	SenderLastNameVar  = "SenderLastName"
	SenderNameVar      = "SenderName"
	SenderEmailVar     = "SenderEmail"
	SenderCompanyVar   = "SenderCompany"
)

// splitName divides a display name on its first space: "Ana Maria Silva" is
// first name "Ana", last name "Maria Silva".
func splitName(name string) (first, last string) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "", ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// senderVars is the sender's merge fields for one mailbox. A nil account
// leaves every field blank, so a template still renders.
func senderVars(account *models.Email, orgName string) map[string]string {
	vars := map[string]string{
		SenderFirstNameVar: "",
		SenderLastNameVar:  "",
		SenderNameVar:      "",
		SenderEmailVar:     "",
		SenderCompanyVar:   strings.TrimSpace(orgName),
	}
	if account == nil {
		return vars
	}
	name := strings.TrimSpace(account.Name)
	first, last := splitName(name)
	vars[SenderFirstNameVar] = first
	vars[SenderLastNameVar] = last
	vars[SenderNameVar] = name
	vars[SenderEmailVar] = account.Email
	return vars
}

// sendExtra assembles the per-send values that are not contact fields: the
// recipient's unsubscribe link and the sender fields for account. The
// organization name is looked up when a repository is wired; without one the
// sender company is blank rather than the send failing.
func (s *tasksService) sendExtra(ctx context.Context, orgID uuid.UUID, account *models.Email, unsubscribeURL string) map[string]string {
	extra := senderVars(account, s.organizationName(ctx, orgID))
	extra[UnsubscribeLinkVar] = unsubscribeURL
	return extra
}

func (s *tasksService) organizationName(ctx context.Context, orgID uuid.UUID) string {
	if s.orgRepo == nil || orgID == uuid.Nil {
		return ""
	}
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil || org == nil {
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID.String()).Msg("sender company: load organization failed")
		}
		return ""
	}
	return org.Name
}

// WireOrganizations attaches the organization lookup the sender company field
// reads. Kept off the constructor so the service stays constructible in tests.
func (s *tasksService) WireOrganizations(r repository.OrganizationRepository) {
	s.orgRepo = r
}

// OrganizationAware is the optional capability the caller uses to attach it.
type OrganizationAware interface {
	WireOrganizations(r repository.OrganizationRepository)
}

// previewTemplatesExtra renders subject/html/plain against contact EXACTLY as
// the send path does (template render + spintax) with the given per-send
// values, and reports parse errors plus any tokens that did not resolve.
func previewTemplatesExtra(subject, bodyHTML, bodyPlain string, contact models.Contact, extra map[string]string) TemplatePreview {
	p := TemplatePreview{
		Subject:   expandSpintax(RenderTemplateWith(subject, contact, extra)),
		BodyHTML:  expandSpintax(RenderTemplateWith(bodyHTML, contact, extra)),
		BodyPlain: expandSpintax(RenderTemplateWith(bodyPlain, contact, extra)),
	}
	for _, f := range []struct{ name, raw string }{{"subject", subject}, {"body", bodyHTML}, {"plain text", bodyPlain}} {
		if err := TemplateError(f.raw); err != nil {
			p.Errors = append(p.Errors, f.name+": "+err.Error())
		}
	}
	seen := map[string]bool{}
	for _, out := range []string{p.Subject, p.BodyHTML, p.BodyPlain} {
		for _, tok := range unresolvedToken.FindAllString(out, -1) {
			if !seen[tok] {
				seen[tok] = true
				p.Unresolved = append(p.Unresolved, tok)
			}
		}
	}
	return p
}
