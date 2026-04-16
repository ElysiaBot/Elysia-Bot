package adapterwebhook

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
	runtimecore "github.com/ohmyopencode/bot-platform/packages/runtime-core"
)

type Dispatcher interface {
	DispatchEvent(context.Context, eventmodel.Event) error
}

type SecretResolver interface {
	Resolve(context.Context, string, string) (string, error)
}

type WebhookPayload struct {
	EventType string         `json:"event_type"`
	Source    string         `json:"source"`
	ActorID   string         `json:"actor_id"`
	Text      string         `json:"text"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Adapter struct {
	Token      string
	TokenRef   string
	Secrets    SecretResolver
	Dispatcher Dispatcher
	Logger     *runtimecore.Logger
	Tracer     *runtimecore.TraceRecorder
	Audits     runtimecore.AuditRecorder
	Now        func() time.Time
}

const (
	dispatchFailureCode           = "dispatch_failed"
	dispatchFailureMessage        = "event dispatch failed"
	dispatchFailureReasonGeneric  = "dispatch_failed"
	dispatchFailureReasonCanceled = "dispatch_canceled"
	secretResolutionFailureCode   = "secret_resolution_failed"
	secretResolutionMessage       = "webhook secret resolution failed"
	webhookUnauthorizedCode       = "webhook_unauthorized"
	webhookUnauthorizedMessage    = "unauthorized"
	webhookInvalidPayloadCode     = "webhook_invalid_payload"
	webhookInvalidPayloadMessage  = "invalid payload"
	webhookInvalidEventCode       = "webhook_invalid_event"
)

func New(token string, dispatcher Dispatcher, logger *runtimecore.Logger, audits runtimecore.AuditRecorder) *Adapter {
	return &Adapter{Token: token, Dispatcher: dispatcher, Logger: logger, Tracer: runtimecore.NewTraceRecorder(), Audits: audits, Now: time.Now().UTC}
}

func NewWithSecretRef(tokenRef string, secrets SecretResolver, dispatcher Dispatcher, logger *runtimecore.Logger, audits runtimecore.AuditRecorder) (*Adapter, error) {
	if err := runtimecore.ValidateSecretRef(tokenRef); err != nil {
		return nil, err
	}
	return &Adapter{TokenRef: tokenRef, Secrets: secrets, Dispatcher: dispatcher, Logger: logger, Tracer: runtimecore.NewTraceRecorder(), Audits: audits, Now: time.Now().UTC}, nil
}

func (a *Adapter) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	expectedToken, err := a.expectedToken(r.Context())
	if err != nil {
		traceID := newWebhookID("trace")
		failureReason := runtimecore.SecretResolutionFailureReason(err)
		if a.Tracer != nil {
			finishFailureSpan := a.Tracer.StartSpan(traceID, "adapter.ingress.secret_resolution.failure", "", "", "", "", map[string]any{"path": r.URL.Path, "failure_code": secretResolutionFailureCode, "failure_reason": failureReason})
			finishFailureSpan()
		}
		if a.Logger != nil {
			_ = a.Logger.Log("error", "webhook secret resolution failed", runtimecore.LogContext{TraceID: traceID}, map[string]any{"path": r.URL.Path, "failure_code": secretResolutionFailureCode, "failure_reason": failureReason, "error": err.Error()})
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "code": secretResolutionFailureCode, "message": secretResolutionMessage, "trace_id": traceID})
		return
	}
	if expectedToken != "" && r.Header.Get("X-Webhook-Token") != expectedToken {
		traceID := newWebhookID("trace")
		a.recordAudit("webhook.reject.unauthorized", r.URL.Path, requestActor(r))
		writeErrorJSON(w, http.StatusUnauthorized, webhookUnauthorizedCode, webhookUnauthorizedMessage, traceID)
		return
	}

	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		traceID := newWebhookID("trace")
		a.recordAudit("webhook.reject.invalid_payload", r.URL.Path, requestActor(r))
		writeErrorJSON(w, http.StatusBadRequest, webhookInvalidPayloadCode, webhookInvalidPayloadMessage, traceID)
		return
	}

	event, err := a.MapPayload(payload)
	if err != nil {
		traceID := newWebhookID("trace")
		a.recordAudit("webhook.reject.invalid_event", payload.Source, payload.ActorID)
		writeErrorJSON(w, http.StatusBadRequest, webhookInvalidEventCode, err.Error(), traceID)
		return
	}
	finishSpan := func() {}
	if a.Tracer != nil {
		finishSpan = a.Tracer.StartSpan(event.TraceID, "adapter.ingress", event.EventID, "", "", event.IdempotencyKey, map[string]any{"source": event.Source, "type": event.Type})
	}
	defer finishSpan()
	if a.Logger != nil {
		_ = a.Logger.Log("info", "webhook ingress mapped to standard event", runtimecore.LogContext{TraceID: event.TraceID, EventID: event.EventID, CorrelationID: event.IdempotencyKey}, map[string]any{"source": event.Source, "type": event.Type})
	}
	if err := a.Dispatcher.DispatchEvent(r.Context(), event); err != nil {
		failureReason := webhookDispatchFailureReason(err)
		if a.Tracer != nil {
			finishFailureSpan := a.Tracer.StartSpan(event.TraceID, "adapter.ingress.failure", event.EventID, "", "", event.IdempotencyKey, map[string]any{"source": event.Source, "type": event.Type, "failure_code": dispatchFailureCode, "failure_reason": failureReason})
			finishFailureSpan()
		}
		if a.Logger != nil {
			_ = a.Logger.Log("error", "webhook ingress dispatch failed", runtimecore.LogContext{TraceID: event.TraceID, EventID: event.EventID, CorrelationID: event.IdempotencyKey}, map[string]any{"source": event.Source, "type": event.Type, "failure_code": dispatchFailureCode, "failure_reason": failureReason, "error": err.Error()})
		}
		writeJSON(w, http.StatusBadGateway, map[string]any{"status": "error", "code": dispatchFailureCode, "message": dispatchFailureMessage, "event_id": event.EventID, "trace_id": event.TraceID})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "event_id": event.EventID, "trace_id": event.TraceID})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErrorJSON(w http.ResponseWriter, statusCode int, code, message, traceID string) {
	writeJSON(w, statusCode, map[string]any{"status": "error", "code": code, "message": message, "trace_id": traceID})
}

func (a *Adapter) expectedToken(ctx context.Context) (string, error) {
	if a.TokenRef == "" {
		return a.Token, nil
	}
	if a.Secrets == nil {
		return "", errors.New("secret resolver is required")
	}
	return a.Secrets.Resolve(ctx, a.TokenRef, "adapter-webhook")
}

func (a *Adapter) MapPayload(payload WebhookPayload) (eventmodel.Event, error) {
	if payload.EventType == "" {
		return eventmodel.Event{}, errors.New("event_type is required")
	}
	if payload.Source == "" {
		return eventmodel.Event{}, errors.New("source is required")
	}

	eventID := newWebhookID("evt")
	event := eventmodel.Event{
		EventID:        eventID,
		TraceID:        newWebhookID("trace"),
		Source:         payload.Source,
		Type:           payload.EventType,
		Timestamp:      a.Now(),
		Actor:          &eventmodel.Actor{ID: payload.ActorID, Type: "service"},
		Message:        &eventmodel.Message{Text: payload.Text},
		Metadata:       payload.Metadata,
		IdempotencyKey: fmt.Sprintf("webhook:%s:%s:%s", payload.Source, payload.EventType, eventID),
	}
	return event, event.Validate()
}

func webhookDispatchFailureReason(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return dispatchFailureReasonCanceled
	}
	return dispatchFailureReasonGeneric
}

func newWebhookID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "-fallback"
	}
	return prefix + "-" + hex.EncodeToString(buf)
}

func (a *Adapter) recordAudit(action, target, actor string) {
	if a == nil || a.Audits == nil {
		return
	}
	if actor == "" {
		actor = "unknown"
	}
	_ = a.Audits.RecordAudit(runtimecoreAuditEntry(actor, action, target, a.Now()))
}

func requestActor(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.RemoteAddr
}

func runtimecoreAuditEntry(actor, action, target string, now time.Time) pluginsdk.AuditEntry {
	return pluginsdk.AuditEntry{
		Actor:      actor,
		Action:     action,
		Target:     target,
		Allowed:    false,
		Reason:     webhookAuditReason(action),
		OccurredAt: now.Format(time.RFC3339),
	}
}

func webhookAuditReason(action string) string {
	switch action {
	case "webhook.reject.unauthorized":
		return "webhook_unauthorized"
	case "webhook.reject.invalid_payload":
		return "webhook_invalid_payload"
	case "webhook.reject.invalid_event":
		return "webhook_invalid_event"
	default:
		return ""
	}
}
