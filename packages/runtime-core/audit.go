package runtimecore

import (
	"context"
	"errors"
	"reflect"
	"sync"

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
