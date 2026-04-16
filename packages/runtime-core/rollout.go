package runtimecore

import (
	"fmt"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

type PluginManifestReader interface {
	LoadPluginManifest(pluginID string) (pluginsdk.PluginManifest, error)
}

type InMemoryRolloutManager struct {
	current   PluginManifestReader
	candidate PluginManifestReader
	records   map[string]pluginsdk.RolloutRecord
}

type RolloutPolicyDeclaration struct {
	EntryPoints           []string `json:"entryPoints,omitempty"`
	PreflightChecks       []string `json:"preflightChecks,omitempty"`
	ActivationChecks      []string `json:"activationChecks,omitempty"`
	AuditReasons          []string `json:"auditReasons,omitempty"`
	RecordStore           string   `json:"recordStore,omitempty"`
	SupportedModes        []string `json:"supportedModes,omitempty"`
	UnsupportedModes      []string `json:"unsupportedModes,omitempty"`
	VerificationEndpoints []string `json:"verificationEndpoints,omitempty"`
	Facts                 []string `json:"facts,omitempty"`
	Summary               string   `json:"summary,omitempty"`
}

func RolloutPolicy() RolloutPolicyDeclaration {
	declaration := RolloutPolicyDeclaration{
		EntryPoints:      []string{"/admin prepare <plugin-id>", "/admin activate <plugin-id>"},
		PreflightChecks:  []string{"manifest.id-match", "manifest.mode-match", "manifest.api-version-match", "manifest.version-changed"},
		ActivationChecks: []string{"prepared-record-required", "prepare-time-drift-recheck", "lifecycle-enable-before-activated-mark"},
		AuditReasons:     []string{"rollout_prepared", "rollout_activated", "rollout_drifted", "rollout_failed"},
		RecordStore:      "in-memory-per-runtime-process",
		SupportedModes:   []string{"manual-prepare-activate", "manifest-preflight", "activate-time-drift-recheck", "minimal-audit-reasons"},
		UnsupportedModes: []string{"rollback", "staged-rollout", "health-check-gate", "automatic-rollback", "persisted-rollout-history", "approval-workflow", "schema-migration-gate"},
		VerificationEndpoints: []string{
			"GET /api/console",
			"go test ./packages/runtime-core ./plugins/plugin-admin ./apps/runtime -run Rollout",
		},
		Facts: []string{
			"rollout is currently a manual admin chain limited to /admin prepare <plugin-id> and /admin activate <plugin-id>",
			"prepare records only minimal manifest compatibility facts for ID, Mode, APIVersion, and Version change",
			"activate re-checks current and candidate manifests before lifecycle enable so prepare-time drift is rejected",
			"rollout records and status remain in-memory only for the current runtime process and are not persisted across restart",
		},
	}
	declaration.Summary = "manual /admin prepare|activate only; preflight checks ID|Mode|APIVersion|Version; activate re-checks drift before lifecycle enable; audit reasons are minimal; no rollback or persisted rollout history"
	return declaration
}

func NewInMemoryRolloutManager(current, candidate PluginManifestReader) *InMemoryRolloutManager {
	return &InMemoryRolloutManager{current: current, candidate: candidate, records: map[string]pluginsdk.RolloutRecord{}}
}

func (m *InMemoryRolloutManager) Prepare(pluginID string) (pluginsdk.RolloutRecord, error) {
	record, err := m.evaluate(pluginID)
	if err != nil {
		return pluginsdk.RolloutRecord{}, err
	}
	m.records[pluginID] = record
	if record.Status == pluginsdk.RolloutStatusRejected {
		return record, fmt.Errorf("rollout preflight rejected: %s", record.Reason)
	}
	return record, nil
}

func (m *InMemoryRolloutManager) Activate(pluginID string) (pluginsdk.RolloutRecord, error) {
	record, err := m.CanActivate(pluginID)
	if err != nil {
		return pluginsdk.RolloutRecord{}, err
	}
	record.Status = pluginsdk.RolloutStatusActivated
	m.records[pluginID] = record
	return record, nil
}

func (m *InMemoryRolloutManager) CanActivate(pluginID string) (pluginsdk.RolloutRecord, error) {
	if m == nil {
		return pluginsdk.RolloutRecord{}, fmt.Errorf("rollout manager is required")
	}
	record, ok := m.records[pluginID]
	if !ok {
		return pluginsdk.RolloutRecord{}, fmt.Errorf("rollout record for %q not found", pluginID)
	}
	if record.Status != pluginsdk.RolloutStatusPrepared {
		return pluginsdk.RolloutRecord{}, fmt.Errorf("rollout record for %q is not prepared", pluginID)
	}
	currentRecord, err := m.evaluate(pluginID)
	if err != nil {
		return pluginsdk.RolloutRecord{}, err
	}
	if currentRecord.Status != pluginsdk.RolloutStatusPrepared {
		m.records[pluginID] = currentRecord
		return pluginsdk.RolloutRecord{}, fmt.Errorf("rollout record for %q drifted after prepare: %s", pluginID, currentRecord.Reason)
	}
	if currentRecord.CurrentVersion != record.CurrentVersion || currentRecord.CandidateVersion != record.CandidateVersion {
		m.records[pluginID] = currentRecord
		return pluginsdk.RolloutRecord{}, fmt.Errorf("rollout record for %q drifted after prepare: current=%s/%s candidate=%s/%s", pluginID, record.CurrentVersion, currentRecord.CurrentVersion, record.CandidateVersion, currentRecord.CandidateVersion)
	}
	return record, nil
}

func (m *InMemoryRolloutManager) evaluate(pluginID string) (pluginsdk.RolloutRecord, error) {
	if m == nil || m.current == nil || m.candidate == nil {
		return pluginsdk.RolloutRecord{}, fmt.Errorf("rollout manifest readers are required")
	}
	current, err := m.current.LoadPluginManifest(pluginID)
	if err != nil {
		return pluginsdk.RolloutRecord{}, err
	}
	candidate, err := m.candidate.LoadPluginManifest(pluginID)
	if err != nil {
		return pluginsdk.RolloutRecord{}, err
	}
	record := pluginsdk.RolloutRecord{PluginID: pluginID, CurrentVersion: current.Version, CandidateVersion: candidate.Version}
	switch {
	case current.ID != pluginID:
		record.Status = pluginsdk.RolloutStatusRejected
		record.Reason = fmt.Sprintf("current manifest id mismatch: expected=%s actual=%s", pluginID, current.ID)
	case candidate.ID != pluginID:
		record.Status = pluginsdk.RolloutStatusRejected
		record.Reason = fmt.Sprintf("candidate manifest id mismatch: expected=%s actual=%s", pluginID, candidate.ID)
	case current.Mode != candidate.Mode:
		record.Status = pluginsdk.RolloutStatusRejected
		record.Reason = fmt.Sprintf("mode mismatch: current=%s candidate=%s", current.Mode, candidate.Mode)
	case candidate.APIVersion == "":
		record.Status = pluginsdk.RolloutStatusRejected
		record.Reason = "candidate apiVersion is required"
	case current.APIVersion != candidate.APIVersion:
		record.Status = pluginsdk.RolloutStatusRejected
		record.Reason = fmt.Sprintf("apiVersion mismatch: current=%s candidate=%s", current.APIVersion, candidate.APIVersion)
	case current.Version == candidate.Version:
		record.Status = pluginsdk.RolloutStatusRejected
		record.Reason = "candidate version must differ from current version"
	default:
		record.Status = pluginsdk.RolloutStatusPrepared
	}
	return record, nil
}

func (m *InMemoryRolloutManager) Record(pluginID string) (pluginsdk.RolloutRecord, bool) {
	if m == nil {
		return pluginsdk.RolloutRecord{}, false
	}
	record, ok := m.records[pluginID]
	return record, ok
}
