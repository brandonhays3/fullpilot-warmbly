package advisor

import (
	"fmt"
	"math"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// campaignDetectors cover campaign shape and performance: whether a campaign is
// configured to succeed, and whether it is actually succeeding.
//
// A campaign has no daily limit of its own: what it can send in a day is the
// sum of its mailboxes' caps (SenderCapacity), which is the number every
// volume check below reasons from.
func campaignDetectors() []Detector {
	return []Detector{
		{
			Key:      "campaign_reply_rate_low",
			Category: models.AdvisorCategoryCampaign,
			About:    "A campaign's reply rate at volume. A very low reply rate on a large sample means the offer, the audience, or the copy is wrong, and sending more of it will cost sender reputation without producing results.",
			Run:      detectReplyRateLow,
		},
		{
			Key:      "campaign_step_dropoff",
			Category: models.AdvisorCategoryCampaign,
			About:    "One sequence step performing far worse than the campaign's best step. Because every step reaches the same audience, a step that underperforms the others by a wide margin is a copy problem isolated to that message.",
			Run:      detectStepDropoff,
		},
		{
			Key:      "campaign_no_senders",
			Category: models.AdvisorCategoryCampaign,
			About:    "A running campaign with no active mailbox resolving for it, usually after a tag was renamed or a mailbox disconnected. The campaign is live and sending nothing.",
			Run:      detectNoSenders,
		},
		{
			Key:      "campaign_list_exhaustion",
			Category: models.AdvisorCategoryCampaign,
			About:    "A running campaign about to run out of leads. Campaigns do not announce that they have finished their list; they just stop producing results.",
			Run:      detectListExhaustion,
		},
		{
			Key:      "campaign_no_ab_test",
			Category: models.AdvisorCategoryCampaign,
			About:    "A high-volume campaign running a single variant. At this volume an A/B test costs nothing and is the only way to know whether the copy is the constraint.",
			Run:      detectNoABTest,
		},
		{
			Key:      "campaign_narrow_window",
			Category: models.AdvisorCategoryCampaign,
			About:    "A sending window too short to fit the campaign's mailboxes' daily volume at safe pacing. The campaign either underdelivers or compresses its sends into a burst.",
			Run:      detectNarrowWindow,
		},
		{
			Key:      "campaign_duplicate_enrollment",
			Category: models.AdvisorCategoryCampaign,
			About:    "Contacts enrolled in more than one running campaign at the same time. To the recipient this is two unrelated strangers pitching in the same week, which is how a sender earns a complaint rather than a reply.",
			Run:      detectDuplicateEnrollment,
		},
	}
}

// runningCampaigns filters to campaigns that are actually sending.
func runningCampaigns(s *repository.AdvisorSnapshot) []repository.AdvisorCampaign {
	out := []repository.AdvisorCampaign{}
	for _, c := range s.Campaigns {
		if c.Status == "active" {
			out = append(out, c)
		}
	}
	return out
}

func detectReplyRateLow(s *repository.AdvisorSnapshot) []Finding {
	out := []Finding{}
	for _, c := range runningCampaigns(s) {
		if c.Sent < minSendsForEngagement {
			continue
		}
		r := rate(c.Replied, c.Sent)
		if r >= replyRateWeak {
			continue
		}

		severity := models.AdvisorMedium
		if r < replyRateVeryLow {
			severity = models.AdvisorHigh
		}

		out = append(out, Finding{
			Key:         "campaign_reply_rate_low",
			GroupTitle:  "{count} campaigns are getting almost no replies",
			Category:    models.AdvisorCategoryCampaign,
			Severity:    severity,
			Surface:     models.AdvisorSurfaceCampaigns,
			EntityType:  "campaign",
			EntityID:    ref(c.ID),
			EntityLabel: c.Name,
			Impact:      clampImpact(40 + c.Sent/100),
			Title:       fmt.Sprintf("%s is getting almost no replies", c.Name),
			Detail: fmt.Sprintf(
				"%s has sent %d emails in the last 30 days and received %s, a reply rate of %s. At this sample size that is not variance: the offer, the audience, or the opening line is not landing, and sending more of it costs sender reputation without producing results.",
				c.Name, c.Sent, plural(c.Replied, "reply", "replies"), pct(r)),
			Remedy: "Pause and change one thing at a time. Narrow the audience first (it is usually the audience), then rewrite the first email around a single specific problem those companies actually have.",
			Steps: []string{
				"Pause the campaign. Every further send at this rate costs reputation and teaches you nothing new.",
				"Look at the audience first. It is the audience far more often than the copy: check that the contacts really are the people who have the problem you are selling to.",
				"Narrow it. A list of 200 companies you can describe precisely outperforms 2,000 you cannot.",
				"Then rewrite the first email around one specific problem those companies have, in the words they would use for it.",
				"Change one thing at a time and restart at low volume. Changing audience and copy together tells you nothing about which one mattered.",
			},
			Evidence: map[string]any{
				"campaign":           c.Name,
				"sends_30d":          c.Sent,
				"replies_30d":        c.Replied,
				"reply_rate_percent": band(r),
				"bounced_30d":        c.Bounced,
				"leads":              c.LeadsTotal,
				"email_steps":        c.EmailStepCount,
			},
		})
	}
	return out
}

func detectStepDropoff(s *repository.AdvisorSnapshot) []Finding {
	// Group steps by campaign so each is compared against its own siblings:
	// absolute reply rates vary hugely by market, but within one campaign the
	// steps all reach the same audience.
	byCampaign := map[string][]repository.AdvisorStep{}
	for _, st := range s.Steps {
		if st.Kind != "email" || st.Sent < minStepSendsForDropoff {
			continue
		}
		byCampaign[st.CampaignID.String()] = append(byCampaign[st.CampaignID.String()], st)
	}

	names := map[string]string{}
	for _, c := range s.Campaigns {
		names[c.ID.String()] = c.Name
	}

	out := []Finding{}
	for cid, steps := range byCampaign {
		if len(steps) < 2 {
			continue
		}
		best := 0.0
		for _, st := range steps {
			if r := rate(st.Replied, st.Sent); r > best {
				best = r
			}
		}
		if best <= 0 {
			// No step replies at all: that is the campaign-level reply-rate
			// finding's job, not a per-step comparison.
			continue
		}

		for _, st := range steps {
			r := rate(st.Replied, st.Sent)
			if r*stepDropoffFactor >= best {
				continue
			}

			label := st.Name
			if label == "" {
				label = fmt.Sprintf("Step %d", st.Position+1)
			}
			out = append(out, Finding{
				Key:         "campaign_step_dropoff",
				GroupTitle:  "{count} sequence steps are far behind the rest of their campaign",
				Category:    models.AdvisorCategoryCampaign,
				Severity:    models.AdvisorMedium,
				Surface:     models.AdvisorSurfaceCampaigns,
				EntityType:  "step",
				EntityID:    ref(st.ID),
				EntityLabel: fmt.Sprintf("%s / %s", names[cid], label),
				ParentType:  "campaign",
				ParentID:    ref(st.CampaignID),
				Impact:      clampImpact(30 + st.Sent/50),
				Title:       fmt.Sprintf("%s is the weak step in %s", label, names[cid]),
				Detail: fmt.Sprintf(
					"%s replies at %s across %d sends, against %s for the best step in the same campaign. Every step reaches the same audience, so a gap this wide is a problem with this message rather than the list.",
					label, pct(r), st.Sent, pct(best)),
				Remedy: "Rewrite this step. Follow-ups that only say \"just bumping this\" reliably land here; one that adds a new, specific reason to reply does not.",
				Evidence: map[string]any{
					"campaign":                names[cid],
					"step":                    label,
					"step_position":           st.Position + 1,
					"step_sends":              st.Sent,
					"step_reply_rate_percent": band(r),
					"best_step_rate_percent":  band(best),
					"subject":                 st.Subject,
				},
			})
		}
	}
	return out
}

func detectNoSenders(s *repository.AdvisorSnapshot) []Finding {
	out := []Finding{}
	for _, camp := range runningCampaigns(s) {
		if camp.SenderCount > 0 {
			continue
		}

		out = append(out, Finding{
			Key:         "campaign_no_senders",
			GroupTitle:  "{count} running campaigns have no mailbox to send from",
			Category:    models.AdvisorCategoryCampaign,
			Severity:    models.AdvisorCritical,
			Surface:     models.AdvisorSurfaceCampaigns,
			EntityType:  "campaign",
			EntityID:    ref(camp.ID),
			EntityLabel: camp.Name,
			Impact:      90,
			Title:       fmt.Sprintf("%s is running but has no mailbox to send from", camp.Name),
			Detail: fmt.Sprintf(
				"%s is active, but no connected active mailbox resolves for it, so it is sending nothing at all. This usually happens after a tag is renamed or the mailbox it depended on was disconnected.",
				camp.Name),
			Remedy: "Attach a mailbox to the campaign, or fix the tag it selects senders by. Nothing else in the campaign matters until this is resolved.",
			Steps: []string{
				"Open the campaign's sender settings.",
				"If it selects senders by tag, check that the tag still exists and still has mailboxes on it. A renamed tag is the usual cause.",
				"If it names mailboxes directly, check they are still connected and active. A disconnected mailbox drops out of the pool silently.",
				"Attach at least one healthy mailbox and save.",
				"Sending resumes on the next scheduling pass. Nothing queued was lost.",
			},
			Evidence: map[string]any{
				"campaign":        camp.Name,
				"sender_strategy": camp.SenderStrategy,
				"sender_count":    0,
				"leads_remaining": camp.LeadsRemaining,
			},
		})
	}
	return out
}

func detectListExhaustion(s *repository.AdvisorSnapshot) []Finding {
	out := []Finding{}
	for _, camp := range runningCampaigns(s) {
		// The campaign drains its list at the pace its mailboxes allow.
		perDay := camp.SenderCapacity
		if perDay <= 0 || camp.LeadsRemaining <= 0 {
			continue
		}
		daysLeft := int(math.Floor(float64(camp.LeadsRemaining) / float64(perDay)))
		if daysLeft > listExhaustionDays {
			continue
		}

		out = append(out, Finding{
			Key:         "campaign_list_exhaustion",
			GroupTitle:  "{count} campaigns are about to run out of leads",
			Category:    models.AdvisorCategoryCampaign,
			Severity:    models.AdvisorLow,
			Surface:     models.AdvisorSurfaceCampaigns,
			EntityType:  "campaign",
			EntityID:    ref(camp.ID),
			EntityLabel: camp.Name,
			Impact:      clampImpact(25 + (listExhaustionDays-daysLeft)*3),
			Title:       fmt.Sprintf("%s is %s out of leads", camp.Name, whenExhausted(daysLeft)),
			Detail: fmt.Sprintf(
				"%s has %s left to reach at %d a day across its mailboxes, so it is %s out. Campaigns do not announce that they have finished their list; they just stop producing results while still looking active.",
				camp.Name, plural(camp.LeadsRemaining, "lead", "leads"), perDay, whenExhausted(daysLeft)),
			Remedy: "Add more contacts now if the campaign is working, or let it finish and read the numbers before building the next list.",
			Steps: []string{
				"Decide whether this campaign is working before you feed it. Check its reply rate on the campaign page.",
				"If it is working, add contacts to the same list and they get picked up on the next scheduling pass. No restart needed.",
				"If it is not, let it finish. Topping up a campaign that is not landing just spends more reputation on the same result.",
				"Either way, build the next list while this one drains, so there is no gap where the mailboxes sit idle.",
			},
			Evidence: map[string]any{
				"campaign":         camp.Name,
				"leads_remaining":  camp.LeadsRemaining,
				"leads_total":      camp.LeadsTotal,
				"mailbox_capacity": perDay,
				"days_remaining":   daysLeft,
				"replies_30d":      camp.Replied,
			},
		})
	}
	return out
}

// whenExhausted phrases the runway in words rather than rendering "about 0
// days", which is the kind of arithmetic-shaped sentence nobody writes.
func whenExhausted(daysLeft int) string {
	switch {
	case daysLeft <= 0:
		return "about to run"
	case daysLeft == 1:
		return "a day from running"
	default:
		return fmt.Sprintf("about %d days from running", daysLeft)
	}
}

func detectNoABTest(s *repository.AdvisorSnapshot) []Finding {
	out := []Finding{}
	for _, camp := range runningCampaigns(s) {
		if camp.VariantCount >= 2 || camp.Sent < minSendsForEngagement*2 {
			continue
		}

		out = append(out, Finding{
			Key:         "campaign_no_ab_test",
			GroupTitle:  "{count} high-volume campaigns are running one version of the copy",
			Category:    models.AdvisorCategoryCampaign,
			Severity:    models.AdvisorLow,
			Surface:     models.AdvisorSurfaceCampaigns,
			EntityType:  "campaign",
			EntityID:    ref(camp.ID),
			EntityLabel: camp.Name,
			Impact:      clampImpact(20 + camp.Sent/200),
			Title:       fmt.Sprintf("%s is running one version of the copy", camp.Name),
			Detail: fmt.Sprintf(
				"%s has sent %d emails on a single variant. At this volume an A/B test costs nothing extra and is the only way to know whether the copy is what is holding the campaign back.",
				camp.Name, camp.Sent),
			Remedy: "Add a second variant that changes one thing: the subject line, or the opening sentence. Changing both at once tells you nothing about which mattered.",
			Steps: []string{
				"Open the first email step and add a variant.",
				"Change exactly one thing in it. The subject line is the usual first test, because it decides whether anything else gets read.",
				"Leave everything else identical, including the call to action. Two variants that differ in three ways cannot tell you which one moved the number.",
				"Let it run until each variant has a few hundred sends before reading the result.",
			},
			Evidence: map[string]any{
				"campaign":           camp.Name,
				"sends_30d":          camp.Sent,
				"variants":           camp.VariantCount,
				"reply_rate_percent": band(rate(camp.Replied, camp.Sent)),
			},
		})
	}
	return out
}

func detectNarrowWindow(s *repository.AdvisorSnapshot) []Finding {
	out := []Finding{}
	for _, camp := range runningCampaigns(s) {
		// What the campaign wants to send in a day is what its mailboxes allow.
		volume := camp.SenderCapacity
		if volume <= 0 || camp.SenderCount == 0 {
			continue
		}
		windowMinutes := minutesBetween(camp.StartTime, camp.EndTime)
		if windowMinutes <= 0 {
			continue
		}
		// Safe pacing: each mailbox holds its own minimum gap, so the campaign's
		// achievable volume is (window / gap) per mailbox.
		gap := defaultMinGap
		for _, m := range s.Mailboxes {
			if m.InActiveCampaign && m.MinWaitTime > 0 && m.MinWaitTime < gap {
				gap = m.MinWaitTime
			}
		}
		achievable := (windowMinutes * 60 / gap) * camp.SenderCount
		if achievable <= 0 || volume <= achievable {
			continue
		}

		out = append(out, Finding{
			Key:         "campaign_narrow_window",
			GroupTitle:  "{count} campaigns cannot fit their daily volume into their sending window",
			Category:    models.AdvisorCategoryCampaign,
			Severity:    models.AdvisorMedium,
			Surface:     models.AdvisorSurfaceCampaigns,
			EntityType:  "campaign",
			EntityID:    ref(camp.ID),
			EntityLabel: camp.Name,
			Impact:      clampImpact(25 + (volume-achievable)/5),
			Title:       fmt.Sprintf("%s cannot fit its daily volume into its sending window", camp.Name),
			Detail: fmt.Sprintf(
				"%s's mailboxes allow %d emails a day but its %s window fits about %d at safe pacing across %s. The campaign will either underdeliver or compress its sends into a burst, and bursts are what filters notice.",
				camp.Name, volume, humanMinutes(windowMinutes), achievable, plural(camp.SenderCount, "mailbox", "mailboxes")),
			Remedy: "Widen the sending window, add mailboxes, or lower the mailboxes' daily caps. Widening the window is free and changes nothing about how the mail looks.",
			Steps: []string{
				"Open the campaign's schedule and widen the sending window. A full working day fits the volume at the mailboxes' own gaps.",
				"If the window has to stay short, add mailboxes so each one carries less of the day.",
				"Otherwise lower the daily cap on the mailboxes attached to this campaign until the window fits; the cap is on each mailbox's settings.",
			},
			Evidence: map[string]any{
				"campaign":         camp.Name,
				"mailbox_capacity": volume,
				"window":           fmt.Sprintf("%s-%s", camp.StartTime, camp.EndTime),
				"window_minutes":   windowMinutes,
				"achievable_sends": achievable,
				"sender_count":     camp.SenderCount,
				"min_gap_seconds":  gap,
			},
		})
	}
	return out
}

func detectDuplicateEnrollment(s *repository.AdvisorSnapshot) []Finding {
	if s.Org.DuplicateContacts == 0 || s.Org.RunningCampaigns < 2 {
		return nil
	}

	severity := models.AdvisorLow
	if s.Org.DuplicateContacts >= 100 {
		severity = models.AdvisorMedium
	}

	return []Finding{{
		Key:      "campaign_duplicate_enrollment",
		Category: models.AdvisorCategoryCampaign,
		Severity: severity,
		Surface:  models.AdvisorSurfaceContacts,
		Impact:   clampImpact(20 + s.Org.DuplicateContacts/10),
		Title:    fmt.Sprintf("%s in more than one running campaign", plural(s.Org.DuplicateContacts, "contact is", "contacts are")),
		Detail: fmt.Sprintf(
			"%s enrolled in two or more of your %d running campaigns at once. To the recipient that is two unrelated strangers pitching in the same week, which is how a sender earns a complaint instead of a reply.",
			plural(s.Org.DuplicateContacts, "contact is", "contacts are"), s.Org.RunningCampaigns),
		Remedy: "Pick one campaign per contact. Deduplicate the overlapping lists, or split the audience by a field that keeps them apart.",
		Steps: []string{
			"Open Contacts and find the contacts enrolled more than once.",
			"Pick which campaign each one should be in. Usually the more specific campaign wins.",
			"Remove them from the others. To the recipient, two of your campaigns is two unrelated strangers pitching in the same week.",
			"Then stop it recurring: split the audiences on a field that keeps them apart, like industry or company size, rather than rebuilding overlapping lists by hand each time.",
		},
		Evidence: map[string]any{
			"duplicate_contacts": s.Org.DuplicateContacts,
			"running_campaigns":  s.Org.RunningCampaigns,
		},
	}}
}

// minutesBetween parses two "HH:MM" strings into a window length in minutes,
// returning 0 for an unparseable or inverted window (which the campaign
// preflight already reports as an error).
func minutesBetween(start, end string) int {
	s, ok1 := parseHHMM(start)
	e, ok2 := parseHHMM(end)
	if !ok1 || !ok2 || e <= s {
		return 0
	}
	return e - s
}

func parseHHMM(v string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(v, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func humanMinutes(mins int) string {
	if mins%60 == 0 {
		return plural(mins/60, "hour", "hour")
	}
	return fmt.Sprintf("%dh%02dm", mins/60, mins%60)
}
