package runtimecore

import (
	"errors"
	"strings"
	"testing"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

type manifestMapReader map[string]pluginsdk.PluginManifest

func (r manifestMapReader) LoadPluginManifest(pluginID string) (pluginsdk.PluginManifest, error) {
	manifest, ok := r[pluginID]
	if !ok {
		return pluginsdk.PluginManifest{}, errPluginManifestNotFound
	}
	return manifest, nil
}

func TestRolloutManagerPreparesCompatibleCandidate(t *testing.T) {
	t.Parallel()

	manager := NewInMemoryRolloutManager(
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.2.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
	)
	record, err := manager.Prepare("plugin-echo")
	if err != nil {
		t.Fatalf("prepare rollout: %v", err)
	}
	if record.Status != pluginsdk.RolloutStatusPrepared || record.CurrentVersion != "0.1.0" || record.CandidateVersion != "0.2.0" {
		t.Fatalf("unexpected rollout record %+v", record)
	}
}

func TestRolloutManagerRejectsIncompatibleCandidate(t *testing.T) {
	t.Parallel()

	manager := NewInMemoryRolloutManager(
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.2.0", APIVersion: "v9", Mode: pluginsdk.ModeSubprocess}},
	)
	record, err := manager.Prepare("plugin-echo")
	if err == nil || !strings.Contains(err.Error(), "rollout preflight rejected") {
		t.Fatalf("expected rollout rejection, got %v", err)
	}
	if record.Status != pluginsdk.RolloutStatusRejected || !strings.Contains(record.Reason, "apiVersion mismatch") {
		t.Fatalf("unexpected rejected rollout record %+v", record)
	}
}

func TestRolloutManagerActivatesPreparedRecord(t *testing.T) {
	t.Parallel()

	manager := NewInMemoryRolloutManager(
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.2.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
	)
	if _, err := manager.Prepare("plugin-echo"); err != nil {
		t.Fatalf("prepare rollout: %v", err)
	}
	record, err := manager.Activate("plugin-echo")
	if err != nil {
		t.Fatalf("activate rollout: %v", err)
	}
	if record.Status != pluginsdk.RolloutStatusActivated {
		t.Fatalf("expected activated rollout record, got %+v", record)
	}
}

func TestRolloutManagerRejectsCandidateManifestIDMismatch(t *testing.T) {
	t.Parallel()

	manager := NewInMemoryRolloutManager(
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
		manifestMapReader{"plugin-echo": {ID: "plugin-ai-chat", Version: "0.2.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
	)
	record, err := manager.Prepare("plugin-echo")
	if err == nil || !strings.Contains(err.Error(), "rollout preflight rejected") {
		t.Fatalf("expected rollout rejection, got %v", err)
	}
	if record.Status != pluginsdk.RolloutStatusRejected || !strings.Contains(record.Reason, "candidate manifest id mismatch") {
		t.Fatalf("unexpected rejected rollout record %+v", record)
	}
}

func TestRolloutManagerRejectsCurrentManifestIDMismatch(t *testing.T) {
	t.Parallel()

	manager := NewInMemoryRolloutManager(
		manifestMapReader{"plugin-echo": {ID: "plugin-admin", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.2.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
	)
	record, err := manager.Prepare("plugin-echo")
	if err == nil || !strings.Contains(err.Error(), "rollout preflight rejected") {
		t.Fatalf("expected rollout rejection, got %v", err)
	}
	if record.Status != pluginsdk.RolloutStatusRejected || !strings.Contains(record.Reason, "current manifest id mismatch") {
		t.Fatalf("unexpected rejected rollout record %+v", record)
	}
}

func TestRolloutManagerDoesNotActivateWithoutPreparedRecord(t *testing.T) {
	t.Parallel()

	manager := NewInMemoryRolloutManager(
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
		manifestMapReader{"plugin-echo": {ID: "plugin-ai-chat", Version: "0.2.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
	)
	if _, err := manager.Activate("plugin-echo"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected activate without prepare to fail, got %v", err)
	}
	if _, err := manager.Prepare("plugin-echo"); err == nil {
		t.Fatal("expected prepare with id mismatch to be rejected")
	}
	if _, err := manager.Activate("plugin-echo"); err == nil || !strings.Contains(err.Error(), "not prepared") {
		t.Fatalf("expected rejected rollout record not to activate, got %v", err)
	}
}

func TestRolloutManagerRejectsActivateWhenCandidateDriftsAfterPrepare(t *testing.T) {
	t.Parallel()

	current := manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}}
	candidate := manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.2.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}}
	manager := NewInMemoryRolloutManager(current, candidate)
	if _, err := manager.Prepare("plugin-echo"); err != nil {
		t.Fatalf("prepare rollout: %v", err)
	}
	candidate["plugin-echo"] = pluginsdk.PluginManifest{ID: "plugin-echo", Version: "0.3.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}
	_, err := manager.Activate("plugin-echo")
	if err == nil || !strings.Contains(err.Error(), "drifted after prepare") {
		t.Fatalf("expected activate drift rejection, got %v", err)
	}
	record, ok := manager.Record("plugin-echo")
	if !ok || record.Status != pluginsdk.RolloutStatusPrepared || record.CandidateVersion != "0.3.0" {
		t.Fatalf("expected rollout record to refresh to latest prepared candidate, got %+v ok=%v", record, ok)
	}
}

func TestRolloutManagerRejectsActivateWhenCompatibilityDriftsAfterPrepare(t *testing.T) {
	t.Parallel()

	current := manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}}
	candidate := manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.2.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}}
	manager := NewInMemoryRolloutManager(current, candidate)
	if _, err := manager.Prepare("plugin-echo"); err != nil {
		t.Fatalf("prepare rollout: %v", err)
	}
	candidate["plugin-echo"] = pluginsdk.PluginManifest{ID: "plugin-echo", Version: "0.2.0", APIVersion: "v9", Mode: pluginsdk.ModeSubprocess}
	_, err := manager.Activate("plugin-echo")
	if err == nil || !strings.Contains(err.Error(), "drifted after prepare") || !strings.Contains(err.Error(), "apiVersion mismatch") {
		t.Fatalf("expected compatibility drift rejection, got %v", err)
	}
	record, ok := manager.Record("plugin-echo")
	if !ok || record.Status != pluginsdk.RolloutStatusRejected || !strings.Contains(record.Reason, "apiVersion mismatch") {
		t.Fatalf("expected rollout record to refresh to rejected drift state, got %+v ok=%v", record, ok)
	}
}

func TestRolloutManagerCanActivateChecksPreparedStateWithoutMutatingRecord(t *testing.T) {
	t.Parallel()

	manager := NewInMemoryRolloutManager(
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
		manifestMapReader{"plugin-echo": {ID: "plugin-echo", Version: "0.2.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}},
	)
	if _, err := manager.Prepare("plugin-echo"); err != nil {
		t.Fatalf("prepare rollout: %v", err)
	}
	record, err := manager.CanActivate("plugin-echo")
	if err != nil {
		t.Fatalf("can activate rollout: %v", err)
	}
	if record.Status != pluginsdk.RolloutStatusPrepared {
		t.Fatalf("expected can-activate to keep prepared state, got %+v", record)
	}
	stored, ok := manager.Record("plugin-echo")
	if !ok || stored.Status != pluginsdk.RolloutStatusPrepared {
		t.Fatalf("expected can-activate not to mutate stored record, got %+v ok=%v", stored, ok)
	}
}

func TestRolloutPolicyDeclarationMatchesCurrentRuntimeBoundaries(t *testing.T) {
	t.Parallel()

	policy := RolloutPolicy()
	if policy.RecordStore != "in-memory-per-runtime-process" {
		t.Fatalf("expected in-memory rollout record store, got %+v", policy)
	}
	for _, expected := range []string{"/admin prepare <plugin-id>", "/admin activate <plugin-id>", "manifest.id-match", "manifest.mode-match", "manifest.api-version-match", "manifest.version-changed", "prepared-record-required", "prepare-time-drift-recheck", "lifecycle-enable-before-activated-mark", "rollout_prepared", "rollout_activated", "rollout_drifted", "rollout_failed", "rollback", "staged-rollout", "persisted-rollout-history"} {
		if !strings.Contains(policy.Summary, "manual /admin prepare|activate only") && expected == "/admin prepare <plugin-id>" {
			t.Fatalf("expected rollout summary to describe manual prepare/activate boundary, got %+v", policy)
		}
		joined := strings.Join(append(append(append(append(append(append(append([]string{}, policy.EntryPoints...), policy.PreflightChecks...), policy.ActivationChecks...), policy.AuditReasons...), policy.SupportedModes...), policy.UnsupportedModes...), policy.Facts...), "\n")
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected rollout policy declaration to contain %q, got %+v", expected, policy)
		}
	}
	if !strings.Contains(policy.Summary, "no rollback or persisted rollout history") {
		t.Fatalf("expected rollout summary to describe unsupported boundaries, got %+v", policy)
	}
}

var errPluginManifestNotFound = errors.New("plugin manifest not found")
