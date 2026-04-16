package runtimecore

import (
	"errors"
	"strings"
	"testing"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

type failingAuditRecorder struct{}

func (failingAuditRecorder) RecordAudit(entry pluginsdk.AuditEntry) error {
	return errors.New("sink failed")
}

func TestInMemoryAuditLogStoresEntriesInOrder(t *testing.T) {
	t.Parallel()

	log := NewInMemoryAuditLog()
	entries := []pluginsdk.AuditEntry{
		{Actor: "admin-user", Action: "enable", Target: "plugin-echo", Allowed: true, OccurredAt: "2026-04-03T12:00:00Z"},
		{Actor: "guest-user", Action: "admin", Target: "/admin enable plugin-echo", Allowed: false, OccurredAt: "2026-04-03T12:01:00Z"},
	}
	setAuditEntryReason(&entries[1], "permission_denied")
	for _, entry := range entries {
		if err := log.RecordAudit(entry); err != nil {
			t.Fatalf("record audit entry: %v", err)
		}
	}

	stored := log.AuditEntries()
	if len(stored) != 2 || stored[0].Actor != "admin-user" || stored[1].Allowed || auditEntryReason(stored[1]) != "permission_denied" {
		t.Fatalf("unexpected stored audit entries %+v", stored)
	}
	stored[0].Actor = "mutated"
	if log.AuditEntries()[0].Actor != "admin-user" {
		t.Fatal("expected audit entries to be returned as a copy")
	}
}

func TestMultiAuditRecorderReturnsJoinedErrors(t *testing.T) {
	t.Parallel()

	recorder := NewMultiAuditRecorder(NewInMemoryAuditLog(), failingAuditRecorder{})
	err := recorder.RecordAudit(pluginsdk.AuditEntry{Actor: "admin-user", Action: "enable", Target: "plugin-echo", Allowed: true, OccurredAt: "2026-04-03T12:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "sink failed") {
		t.Fatalf("expected joined audit recorder failure, got %v", err)
	}
}
