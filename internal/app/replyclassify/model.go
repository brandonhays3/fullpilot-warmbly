package replyclassify

import (
	"context"
	"strings"
	"sync"
	"time"
)

// The model classifier rides the platform LLM provider wired in from the app
// mains via SetModelClassifier, on the cheap model AI_MODEL_CLASSIFY selects,
// so it uses the same self-hostable backend as every other AI feature. It is
// platform-paid: this path never charges org credits. When no provider is
// wired (no AI_PROVIDER) it is a pure no-op and the deterministic layers
// decide WITHOUT any network call.
//
// The model answers with one of the six reply classes. Its verdict is taken
// as-is; the header and lexicon layers only run when the model is off, the
// call fails, or the answer is not a known label.

// modelTimeout bounds the single completion.
const modelTimeout = 8 * time.Second

// modelBodyLimit caps the body the model reads, in runes: a reply's intent
// sits in its first lines, and the quoted history below only costs tokens.
const modelBodyLimit = 4000

const modelSystemPrompt = "You classify a reply to a cold sales email. " +
	"Answer with exactly one lowercase label and nothing else:\n" +
	"positive: interested, wants to talk, asks for details, pricing or a meeting\n" +
	"negative: not interested, a rejection, wrong person with no referral\n" +
	"neutral: a question, a deferral, a referral, or anything unclear\n" +
	"auto_reply: an automatic message such as a delivery report, bounce, ticket acknowledgement or list notice\n" +
	"out_of_office: a vacation, leave or away autoresponder\n" +
	"unsubscribe: asks to stop, be removed, or opt out\n" +
	"Judge the reply itself, not the quoted history below it. Do not explain."

// modelHeaders are the headers handed to the model alongside the text: the
// machine-reply markers the header layer keys on, so the model can tell an
// autoresponder or a bounce from a human.
var modelHeaders = []string{
	"From", "Auto-Submitted", "Precedence", "X-Autoreply", "X-Autorespond",
	"X-Auto-Response-Suppress", "Return-Path", "Content-Type",
}

// ModelClassifyFunc runs one platform LLM completion: given the system + user
// prompt it returns the model's raw text. The app mains adapt
// generation.Provider.Complete to this shape and wire it with
// SetModelClassifier, keeping this low-level package free of a direct provider
// dependency. nil means the model layer is disabled.
type ModelClassifyFunc func(ctx context.Context, system, user string) (string, error)

var (
	modelMu       sync.RWMutex
	modelClassify ModelClassifyFunc
)

// SetModelClassifier wires (or clears, with nil) the platform provider that
// backs the model layer. Safe to call once at startup; guarded for concurrent
// reads.
func SetModelClassifier(fn ModelClassifyFunc) {
	modelMu.Lock()
	modelClassify = fn
	modelMu.Unlock()
}

// ModelEnabled reports whether a provider is wired for the model layer.
func ModelEnabled() bool {
	modelMu.RLock()
	defer modelMu.RUnlock()
	return modelClassify != nil
}

// classifyModel runs the model when a provider is wired. Returns (zero, false)
// when unconfigured, on any error, or on an answer outside the label set, so
// the caller falls back to the deterministic layers and NEVER hard-errors on
// a classification miss.
func classifyModel(ctx context.Context, in Input) (Result, bool) {
	modelMu.RLock()
	fn := modelClassify
	modelMu.RUnlock()
	if fn == nil {
		return Result{}, false
	}

	user := modelUserPrompt(in)
	if user == "" {
		return Result{}, false
	}

	cctx, cancel := context.WithTimeout(ctx, modelTimeout)
	defer cancel()

	out, err := fn(cctx, modelSystemPrompt, user)
	if err != nil {
		return Result{}, false
	}

	switch normalizeModelLabel(out) {
	case ClassPositive:
		return Result{Class: ClassPositive, Confidence: 0.8, Source: SourceModel}, true
	case ClassNegative:
		return Result{Class: ClassNegative, Confidence: 0.8, Source: SourceModel}, true
	case ClassNeutral:
		return Result{Class: ClassNeutral, Confidence: 0.7, Source: SourceModel}, true
	case ClassAutoReply:
		return Result{Class: ClassAutoReply, Confidence: 0.85, Source: SourceModel}, true
	case ClassOutOfOffice:
		return Result{Class: ClassOutOfOffice, Confidence: 0.85, Source: SourceModel}, true
	case ClassUnsubscribe:
		return Result{Class: ClassUnsubscribe, Confidence: 0.85, Source: SourceModel}, true
	default:
		return Result{}, false
	}
}

// modelUserPrompt renders the reply for the model: the machine-marker headers
// that are present, the subject, and a bounded body. Empty when there is
// nothing to judge.
func modelUserPrompt(in Input) string {
	h := newHeaderLookup(in.Headers)
	var b strings.Builder
	for _, name := range modelHeaders {
		if v := h.first(name); v != "" {
			b.WriteString(name)
			b.WriteString(": ")
			b.WriteString(v)
			b.WriteString("\n")
		}
	}
	b.WriteString("Subject: ")
	b.WriteString(strings.TrimSpace(in.Subject))
	b.WriteString("\n\n")
	body := strings.TrimSpace(in.BodyText)
	if r := []rune(body); len(r) > modelBodyLimit {
		body = string(r[:modelBodyLimit])
	}
	b.WriteString(body)
	if strings.TrimSpace(in.Subject) == "" && body == "" {
		return ""
	}
	return strings.TrimSpace(b.String())
}

// normalizeModelLabel reduces the model's free text to one of the six labels,
// tolerating stray punctuation, whitespace and the spelled-out forms
// ("out of office", "auto reply"). Anything else is rejected so the caller
// falls back to the deterministic layers.
func normalizeModelLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, ".\"'`* \n\t")
	s = strings.NewReplacer("-", "_", " ", "_").Replace(s)
	switch {
	case strings.HasPrefix(s, "out_of_office"), strings.HasPrefix(s, "out_of_the_office"):
		return ClassOutOfOffice
	case strings.HasPrefix(s, "auto_reply"), strings.HasPrefix(s, "autoreply"), strings.HasPrefix(s, "auto_replied"):
		return ClassAutoReply
	case strings.HasPrefix(s, "unsubscribe"), strings.HasPrefix(s, "opt_out"):
		return ClassUnsubscribe
	case strings.HasPrefix(s, ClassPositive):
		return ClassPositive
	case strings.HasPrefix(s, ClassNegative):
		return ClassNegative
	case strings.HasPrefix(s, ClassNeutral):
		return ClassNeutral
	default:
		return ""
	}
}
