package runtimecore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

type AuditLogReader interface {
	AuditEntries() []pluginsdk.AuditEntry
}

type AuditRecorder interface {
	RecordAudit(entry pluginsdk.AuditEntry) error
}

type InMemoryAuditLog struct {
	mu      sync.RWMutex
	entries []pluginsdk.AuditEntry
}

func NewInMemoryAuditLog() *InMemoryAuditLog {
	return &InMemoryAuditLog{}
}

type JoinedAuditLog struct {
	recorder AuditRecorder
	reader   AuditLogReader
}

func NewJoinedAuditLog(recorder AuditRecorder, reader AuditLogReader) *JoinedAuditLog {
	return &JoinedAuditLog{recorder: recorder, reader: reader}
}

func (l *JoinedAuditLog) RecordAudit(entry pluginsdk.AuditEntry) error {
	if l == nil || l.recorder == nil {
		return nil
	}
	return l.recorder.RecordAudit(entry)
}

func (l *JoinedAuditLog) AuditEntries() []pluginsdk.AuditEntry {
	if l == nil || l.reader == nil {
		return nil
	}
	return l.reader.AuditEntries()
}

func (l *InMemoryAuditLog) RecordAudit(entry pluginsdk.AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
	return nil
}

func (l *InMemoryAuditLog) AuditEntries() []pluginsdk.AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]pluginsdk.AuditEntry(nil), l.entries...)
}

func ApplyAuditExecutionContext(entry *pluginsdk.AuditEntry, ctx eventmodel.ExecutionContext) {
	if entry == nil {
		return
	}
	ctx = normalizeExecutionContextObservability(ctx)
	if strings.TrimSpace(entry.TraceID) == "" {
		entry.TraceID = strings.TrimSpace(ctx.TraceID)
	}
	if strings.TrimSpace(entry.EventID) == "" {
		entry.EventID = strings.TrimSpace(ctx.EventID)
	}
	if strings.TrimSpace(entry.PluginID) == "" {
		entry.PluginID = strings.TrimSpace(ctx.PluginID)
	}
	if strings.TrimSpace(entry.RunID) == "" {
		entry.RunID = strings.TrimSpace(ctx.RunID)
	}
	if strings.TrimSpace(entry.CorrelationID) == "" {
		entry.CorrelationID = strings.TrimSpace(ctx.CorrelationID)
	}
}

type PostgresAuditRecorder struct {
	ctx   context.Context
	store *PostgresStore
}

func NewPostgresAuditRecorder(ctx context.Context, store *PostgresStore) *PostgresAuditRecorder {
	return &PostgresAuditRecorder{ctx: ctx, store: store}
}

func (r *PostgresAuditRecorder) RecordAudit(entry pluginsdk.AuditEntry) error {
	if r == nil || r.store == nil {
		return errors.New("postgres audit store is required")
	}
	ctx := r.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return r.store.SaveAudit(ctx, entry)
}

type MultiAuditRecorder struct {
	recorders []AuditRecorder
}

func NewMultiAuditRecorder(recorders ...AuditRecorder) *MultiAuditRecorder {
	filtered := make([]AuditRecorder, 0, len(recorders))
	for _, recorder := range recorders {
		if recorder != nil {
			filtered = append(filtered, recorder)
		}
	}
	return &MultiAuditRecorder{recorders: filtered}
}

func (m *MultiAuditRecorder) RecordAudit(entry pluginsdk.AuditEntry) error {
	var errs []error
	for _, recorder := range m.recorders {
		if err := recorder.RecordAudit(entry); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func setAuditEntryReason(entry *pluginsdk.AuditEntry, reason string) {
	if entry == nil || reason == "" {
		return
	}
	value := reflect.ValueOf(entry).Elem()
	field := value.FieldByName("Reason")
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
		field.SetString(reason)
	}
}

func auditEntryReason(entry pluginsdk.AuditEntry) string {
	value := reflect.ValueOf(entry)
	field := value.FieldByName("Reason")
	if field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}
	return ""
}

func parseAuditOccurredAt(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("audit occurred_at is required")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}
