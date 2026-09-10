package models

import "strings"

// Send method providers, the middle segment of a send method label. They
// name the party whose infrastructure carried the message, not the mailbox
// type as stored (google rather than gmail), so the label reads the same
// across the dashboard, the API and an export.
const (
	SendMethodProviderGoogle    = "google"
	SendMethodProviderMicrosoft = "microsoft"
	SendMethodProviderSMTPIMAP  = "smtpimap"
)

// SendMethodProvider maps a mailbox provider to its send method segment.
func SendMethodProvider(p InboxProvider) string {
	switch p {
	case InboxProviderGoogle:
		return SendMethodProviderGoogle
	case InboxProviderOutlook:
		return SendMethodProviderMicrosoft
	default:
		return SendMethodProviderSMTPIMAP
	}
}

// SendMethodLabel is the string stamped on a send: <transport>_<provider>_
// <deployment>, for example smtp_google_cloud_run_worker or
// api_microsoft_cloud_vm. Deployment is the worker's WORKER_DEPLOYMENT and
// may itself contain underscores, so a reader splits on the first two only.
func SendMethodLabel(transport MailTransport, provider InboxProvider, deployment string) string {
	deployment = strings.TrimSpace(deployment)
	if deployment == "" {
		deployment = "unknown"
	}
	return string(transport) + "_" + SendMethodProvider(provider) + "_" + deployment
}
