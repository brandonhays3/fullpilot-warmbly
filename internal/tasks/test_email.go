package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/aisettings"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/generation"
)

// GetCampaignSequences returns the sequences for a campaign ordered by position
func (s *tasksService) GetCampaignSequences(ctx context.Context, campaignID uuid.UUID) ([]models.Sequence, error) {
	return s.campaignRepo.GetSequencesByCampaignID(ctx, campaignID)
}

// testContact stands in when no real contact is chosen for a test send.
func testContact(recipient string) models.Contact {
	return models.Contact{
		ID:        uuid.New(),
		FirstName: "Test",
		LastName:  "Recipient",
		Email:     recipient,
		Company:   "Test Company",
	}
}

// SendTestEmail renders a campaign step as the send path would and mails it to
// recipient through one of the organization's mailboxes. contact, when given,
// is the real contact the copy is rendered for; nil uses a placeholder. The
// message carries the campaign's attachments, the mailbox signature and the
// opt-out footer, so what lands in the tester's inbox is what a lead gets.
func (s *tasksService) SendTestEmail(ctx context.Context, orgID uuid.UUID, accountID uuid.UUID, recipient string, campaign *models.Campaign, sequence *models.Sequence, contact *models.Contact) *errx.Error {
	// Any member allowed to send may test from any active mailbox of the
	// organization: not only the ones they connected, and not only the
	// campaign's own sender pool. GetByID is the full row (the org-scoped Get
	// omits worker_id, which the send needs).
	account, err := s.emailRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || account.OrganizationID == nil || *account.OrganizationID != orgID {
		return errx.New(errx.NotFound, "email account not found")
	}
	// A paused or revoked mailbox has no worker to send through; say so
	// instead of failing the dispatch.
	if account.Status != "active" {
		return errx.New(errx.BadRequest, "email account is not active")
	}

	renderFor := testContact(recipient)
	if contact != nil {
		renderFor = *contact
	}

	// A step with AI blocks needs the workspace key before anything is
	// rendered: the same gate that refuses a campaign start, so a test never
	// ships raw [[ai:...]] tokens or fails halfway.
	hasAI := HasAIVariables(sequence.BodyHTML)
	if hasAI {
		if s.aiSettings == nil {
			return errx.ErrCampaignAIKeyMissing
		}
		hasKey, kerr := s.aiSettings.HasKey(ctx, orgID)
		if kerr != nil {
			return errx.InternalError()
		}
		if !hasKey {
			return errx.ErrCampaignAIKeyMissing
		}
	}

	// A test send carries the real opt-out footer so the sender sees exactly
	// what a recipient will, but its link names no contact (uuid.Nil), so
	// clicking it can never suppress anyone.
	optOut := s.resolveOptOut(ctx, orgID, campaign)
	var unsubscribeURL string
	if s.unsubLinks != nil && s.unsubLinks.Enabled() {
		unsubscribeURL = s.unsubLinks.URL(orgID, campaign.ID, uuid.Nil, time.Now())
	}

	rendered := previewTemplatesExtra(sequence.Subject, sequence.BodyHTML, sequence.BodyPlain, renderFor, s.sendExtra(ctx, orgID, account, unsubscribeURL))
	renderedSubject, renderedHTML, renderedPlain := rendered.Subject, rendered.BodyHTML, rendered.BodyPlain
	if hasAI {
		// Written fresh for the tester's contact, exactly as a send would, so
		// the test shows real AI output rather than the block's token.
		var aerr error
		renderedSubject, renderedHTML, renderedPlain, aerr = s.ResolveAIVariablesForTest(ctx, orgID, &renderFor, renderedSubject, renderedHTML, renderedPlain)
		if aerr != nil {
			switch {
			case errors.Is(aerr, aisettings.ErrKeyMissing):
				return errx.ErrCampaignAIKeyMissing
			case errors.Is(aerr, generation.ErrProviderAuth):
				return errx.ErrAIProviderRejected
			}
			return errx.New(errx.ServiceUnavailable, fmt.Sprintf("could not write the AI blocks for this test: %v", aerr))
		}
	}
	bodyHTML, bodyPlain := finishBody(renderedHTML, renderedPlain, campaign.TextOnly, account, &optOut, unsubscribeURL)
	subject := "[TEST] " + renderedSubject

	// Never a List-Unsubscribe header, on a test send as on a real one.
	headerURL := ""

	// Tracking is deliberately off: a test open or click must not count.
	emailMsg := EmailMessage{
		From:           account.Email,
		To:             []string{recipient},
		Subject:        subject,
		BodyHTML:       bodyHTML,
		BodyPlain:      bodyPlain,
		MessageID:      generateMessageID(account.Email),
		IsWarmup:       false,
		UnsubscribeURL: headerURL,
		Attachments:    s.campaignAttachmentRefs(ctx, campaign.ID, sequence.ID),
	}

	taskID := uuid.New()
	if err := s.emailSender.Send(ctx, taskID, emailMsg, *account); err != nil {
		return errx.New(errx.Internal, fmt.Sprintf("failed to send test email: %v", err))
	}

	return nil
}
