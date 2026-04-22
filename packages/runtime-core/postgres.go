package runtimecore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

const PostgresSchemaV0 = `
CREATE TABLE IF NOT EXISTS event_log (
  event_id TEXT PRIMARY KEY,
  trace_id TEXT NOT NULL,
  source TEXT NOT NULL,
  type TEXT NOT NULL,
  payload_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS job_state (
  job_id TEXT PRIMARY KEY,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL,
  payload_json JSONB NOT NULL,
  retry_count INT NOT NULL,
  max_retries INT NOT NULL,
  timeout_seconds INT NOT NULL,
  last_error TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  started_at TIMESTAMPTZ NULL,
  finished_at TIMESTAMPTZ NULL,
  next_run_at TIMESTAMPTZ NULL,
  dead_letter BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_state (
  workflow_id TEXT PRIMARY KEY,
  current_index INT NOT NULL,
  waiting_for TEXT NOT NULL,
  sleeping_until TIMESTAMPTZ NULL,
  completed BOOLEAN NOT NULL,
  compensated BOOLEAN NOT NULL,
  state_json JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS plugin_registry_pg (
  plugin_id TEXT PRIMARY KEY,
  version TEXT NOT NULL,
  api_version TEXT NOT NULL,
  mode TEXT NOT NULL,
  manifest_json JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS plugin_enabled_overlays_pg (
  plugin_id TEXT PRIMARY KEY,
  enabled BOOLEAN NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS plugin_configs_pg (
  plugin_id TEXT PRIMARY KEY,
  config_json JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS plugin_status_snapshots_pg (
  plugin_id TEXT PRIMARY KEY,
  last_dispatch_kind TEXT NOT NULL,
  last_dispatch_success BOOLEAN NOT NULL,
  last_dispatch_error TEXT NOT NULL,
  last_dispatch_at TIMESTAMPTZ NOT NULL,
  last_recovered_at TIMESTAMPTZ NULL,
  last_recovery_failure_count INT NOT NULL,
  current_failure_streak INT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS adapter_instances_pg (
  instance_id TEXT PRIMARY KEY,
  adapter TEXT NOT NULL,
  source TEXT NOT NULL,
  config_json JSONB NOT NULL,
  status TEXT NOT NULL,
  health TEXT NOT NULL,
  online BOOLEAN NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions_pg (
  session_id TEXT PRIMARY KEY,
  plugin_id TEXT NOT NULL,
  state_json JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS idempotency_keys_pg (
  idempotency_key TEXT PRIMARY KEY,
  event_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS operator_identities_pg (
  actor_id TEXT PRIMARY KEY,
  roles_json JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS rbac_snapshots_pg (
  snapshot_key TEXT PRIMARY KEY,
  console_read_permission TEXT NOT NULL,
  policies_json JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS replay_operation_records_pg (
  replay_id TEXT PRIMARY KEY,
  source_event_id TEXT NOT NULL,
  replay_event_id TEXT NOT NULL,
  status TEXT NOT NULL,
  reason TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS rollout_operation_records_pg (
  operation_id TEXT PRIMARY KEY,
  plugin_id TEXT NOT NULL,
  action TEXT NOT NULL,
  current_version TEXT NOT NULL,
  candidate_version TEXT NOT NULL,
  status TEXT NOT NULL,
  reason TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_log (
  actor TEXT NOT NULL,
  permission TEXT NOT NULL,
  action TEXT NOT NULL,
  target TEXT NOT NULL,
  allowed BOOLEAN NOT NULL,
  reason TEXT NULL,
  trace_id TEXT NOT NULL,
  event_id TEXT NOT NULL,
  plugin_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  correlation_id TEXT NOT NULL,
  error_category TEXT NOT NULL,
  error_code TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL
);
`

type PostgresStore struct {
	pool postgresPool
}

type rowScanner interface {
	Scan(dest ...any) error
}

type postgresRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

type postgresPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
	Close()
}

var _ EventJournalReader = (*PostgresStore)(nil)

func OpenPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	store := &PostgresStore{pool: pool}
	if err := store.Init(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

func (s *PostgresStore) Init(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, PostgresSchemaV0)
	if err != nil {
		return fmt.Errorf("init postgres schema: %w", err)
	}
	return nil
}

func WritePostgresMigration(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(PostgresSchemaV0), 0o644)
}

func (s *PostgresStore) SaveEvent(ctx context.Context, event eventmodel.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO event_log (event_id, trace_id, source, type, payload_json, created_at) VALUES ($1,$2,$3,$4,$5,$6)`, event.EventID, event.TraceID, event.Source, event.Type, payload, time.Now().UTC())
	return err
}

func (s *PostgresStore) RecordEvent(ctx context.Context, event eventmodel.Event) error {
	return s.SaveEvent(ctx, event)
}

func (s *PostgresStore) SaveJob(ctx context.Context, job Job) error {
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO job_state (job_id, job_type, status, payload_json, retry_count, max_retries, timeout_seconds, last_error, created_at, started_at, finished_at, next_run_at, dead_letter) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT (job_id) DO UPDATE SET status=excluded.status, payload_json=excluded.payload_json, retry_count=excluded.retry_count, max_retries=excluded.max_retries, timeout_seconds=excluded.timeout_seconds, last_error=excluded.last_error, started_at=excluded.started_at, finished_at=excluded.finished_at, next_run_at=excluded.next_run_at, dead_letter=excluded.dead_letter`, job.ID, job.Type, job.Status, payload, job.RetryCount, job.MaxRetries, int(job.Timeout.Seconds()), job.LastError, job.CreatedAt, job.StartedAt, job.FinishedAt, job.NextRunAt, job.DeadLetter)
	return err
}

func (s *PostgresStore) SaveWorkflow(ctx context.Context, workflow Workflow) error {
	payload, err := json.Marshal(workflow.State)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO workflow_state (workflow_id, current_index, waiting_for, sleeping_until, completed, compensated, state_json, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (workflow_id) DO UPDATE SET current_index=excluded.current_index, waiting_for=excluded.waiting_for, sleeping_until=excluded.sleeping_until, completed=excluded.completed, compensated=excluded.compensated, state_json=excluded.state_json, updated_at=excluded.updated_at`, workflow.ID, workflow.CurrentIndex, workflow.WaitingFor, workflow.SleepingUntil, workflow.Completed, workflow.Compensated, payload, time.Now().UTC())
	return err
}

func (s *PostgresStore) SavePluginManifest(ctx context.Context, manifest pluginsdk.PluginManifest) error {
	payload, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO plugin_registry_pg (plugin_id, version, api_version, mode, manifest_json, updated_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (plugin_id) DO UPDATE SET version=excluded.version, api_version=excluded.api_version, mode=excluded.mode, manifest_json=excluded.manifest_json, updated_at=excluded.updated_at`, manifest.ID, manifest.Version, manifest.APIVersion, manifest.Mode, payload, time.Now().UTC())
	return err
}

func (s *PostgresStore) SavePluginEnabledState(ctx context.Context, pluginID string, enabled bool) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return fmt.Errorf("save plugin enabled state: plugin id is required")
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO plugin_enabled_overlays_pg (plugin_id, enabled, updated_at)
VALUES ($1, $2, $3)
ON CONFLICT (plugin_id) DO UPDATE SET
  enabled=excluded.enabled,
  updated_at=excluded.updated_at
`, pluginID, enabled, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("upsert plugin enabled state: %w", err)
	}
	return nil
}

func (s *PostgresStore) LoadPluginEnabledState(ctx context.Context, pluginID string) (PluginEnabledState, error) {
	row := s.pool.QueryRow(ctx, `
SELECT plugin_id, enabled, updated_at
FROM plugin_enabled_overlays_pg
WHERE plugin_id = $1
`, strings.TrimSpace(pluginID))
	state, err := scanPostgresPluginEnabledState(row)
	if err != nil {
		if isPostgresNoRows(err) {
			return PluginEnabledState{}, sql.ErrNoRows
		}
		return PluginEnabledState{}, fmt.Errorf("load plugin enabled state: %w", err)
	}
	return state, nil
}

func (s *PostgresStore) ListPluginEnabledStates(ctx context.Context) ([]PluginEnabledState, error) {
	rows, err := s.pool.Query(ctx, `
SELECT plugin_id, enabled, updated_at
FROM plugin_enabled_overlays_pg
ORDER BY plugin_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list plugin enabled states: %w", err)
	}
	defer rows.Close()
	return scanPostgresPluginEnabledStates(rows)
}

func (s *PostgresStore) SavePluginConfig(ctx context.Context, pluginID string, rawConfig json.RawMessage) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return fmt.Errorf("save plugin config: plugin id is required")
	}
	if len(rawConfig) == 0 {
		return fmt.Errorf("save plugin config: raw config is required")
	}
	var normalized any
	if err := json.Unmarshal(rawConfig, &normalized); err != nil {
		return fmt.Errorf("save plugin config: unmarshal raw config: %w", err)
	}
	normalizedRaw, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("save plugin config: marshal raw config: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO plugin_configs_pg (plugin_id, config_json, updated_at)
VALUES ($1, $2, $3)
ON CONFLICT (plugin_id) DO UPDATE SET
  config_json=excluded.config_json,
  updated_at=excluded.updated_at
`, pluginID, normalizedRaw, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("upsert plugin config: %w", err)
	}
	return nil
}

func (s *PostgresStore) LoadPluginConfig(ctx context.Context, pluginID string) (PluginConfigState, error) {
	row := s.pool.QueryRow(ctx, `
SELECT plugin_id, config_json, updated_at
FROM plugin_configs_pg
WHERE plugin_id = $1
`, strings.TrimSpace(pluginID))
	state, err := scanPostgresPluginConfigState(row)
	if err != nil {
		if isPostgresNoRows(err) {
			return PluginConfigState{}, sql.ErrNoRows
		}
		return PluginConfigState{}, fmt.Errorf("load plugin config: %w", err)
	}
	return state, nil
}

func (s *PostgresStore) ListPluginConfigs(ctx context.Context) ([]PluginConfigState, error) {
	rows, err := s.pool.Query(ctx, `
SELECT plugin_id, config_json, updated_at
FROM plugin_configs_pg
ORDER BY plugin_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list plugin configs: %w", err)
	}
	defer rows.Close()
	return scanPostgresPluginConfigStates(rows)
}

func (s *PostgresStore) RecordDispatchResult(result DispatchResult) error {
	return s.SavePluginStatusSnapshot(context.Background(), result)
}

func (s *PostgresStore) SavePluginStatusSnapshot(ctx context.Context, result DispatchResult) error {
	if strings.TrimSpace(result.PluginID) == "" {
		return fmt.Errorf("save plugin status snapshot: plugin id is required")
	}
	if result.At.IsZero() {
		result.At = time.Now().UTC()
	}
	current, err := s.LoadPluginStatusSnapshot(ctx, result.PluginID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load plugin status snapshot: %w", err)
	}
	var previous *PluginStatusSnapshot
	if err == nil {
		previous = &current
	}
	snapshot := nextPluginStatusSnapshot(previous, result)
	_, err = s.pool.Exec(ctx, `
INSERT INTO plugin_status_snapshots_pg (
  plugin_id, last_dispatch_kind, last_dispatch_success, last_dispatch_error, last_dispatch_at,
  last_recovered_at, last_recovery_failure_count, current_failure_streak, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (plugin_id) DO UPDATE SET
  last_dispatch_kind=excluded.last_dispatch_kind,
  last_dispatch_success=excluded.last_dispatch_success,
  last_dispatch_error=excluded.last_dispatch_error,
  last_dispatch_at=excluded.last_dispatch_at,
  last_recovered_at=excluded.last_recovered_at,
  last_recovery_failure_count=excluded.last_recovery_failure_count,
  current_failure_streak=excluded.current_failure_streak,
  updated_at=excluded.updated_at
`, snapshot.PluginID, snapshot.LastDispatchKind, snapshot.LastDispatchSuccess, snapshot.LastDispatchError, snapshot.LastDispatchAt.UTC(), postgresNullableTimestamp(snapshot.LastRecoveredAt), snapshot.LastRecoveryFailureCount, snapshot.CurrentFailureStreak, snapshot.UpdatedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert plugin status snapshot: %w", err)
	}
	return nil
}

func (s *PostgresStore) LoadPluginStatusSnapshot(ctx context.Context, pluginID string) (PluginStatusSnapshot, error) {
	row := s.pool.QueryRow(ctx, `
SELECT plugin_id, last_dispatch_kind, last_dispatch_success, last_dispatch_error, last_dispatch_at,
       last_recovered_at, last_recovery_failure_count, current_failure_streak, updated_at
FROM plugin_status_snapshots_pg
WHERE plugin_id = $1
`, strings.TrimSpace(pluginID))
	snapshot, err := scanPostgresPluginStatusSnapshot(row)
	if err != nil {
		if isPostgresNoRows(err) {
			return PluginStatusSnapshot{}, sql.ErrNoRows
		}
		return PluginStatusSnapshot{}, fmt.Errorf("load plugin status snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *PostgresStore) ListPluginStatusSnapshots(ctx context.Context) ([]PluginStatusSnapshot, error) {
	rows, err := s.pool.Query(ctx, `
SELECT plugin_id, last_dispatch_kind, last_dispatch_success, last_dispatch_error, last_dispatch_at,
       last_recovered_at, last_recovery_failure_count, current_failure_streak, updated_at
FROM plugin_status_snapshots_pg
ORDER BY plugin_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list plugin status snapshots: %w", err)
	}
	defer rows.Close()
	return scanPostgresPluginStatusSnapshots(rows)
}

func (s *PostgresStore) SaveSession(ctx context.Context, session SessionState) error {
	payload, err := json.Marshal(session.State)
	if err != nil {
		return fmt.Errorf("marshal session state: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO sessions_pg (session_id, plugin_id, state_json, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (session_id) DO UPDATE SET
  plugin_id=excluded.plugin_id,
  state_json=excluded.state_json,
  updated_at=excluded.updated_at
`, session.SessionID, session.PluginID, payload, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	return nil
}

func (s *PostgresStore) SaveAdapterInstance(ctx context.Context, state AdapterInstanceState) error {
	state.InstanceID = strings.TrimSpace(state.InstanceID)
	if state.InstanceID == "" {
		return fmt.Errorf("save adapter instance: instance id is required")
	}
	state.Adapter = strings.TrimSpace(state.Adapter)
	if state.Adapter == "" {
		return fmt.Errorf("save adapter instance: adapter is required")
	}
	state.Source = strings.TrimSpace(state.Source)
	if state.Source == "" {
		return fmt.Errorf("save adapter instance: source is required")
	}
	if len(state.RawConfig) == 0 {
		state.RawConfig = json.RawMessage(`{}`)
	}
	var rawConfigValue any
	if err := json.Unmarshal(state.RawConfig, &rawConfigValue); err != nil {
		return fmt.Errorf("save adapter instance: unmarshal config: %w", err)
	}
	normalizedRawConfig, err := json.Marshal(rawConfigValue)
	if err != nil {
		return fmt.Errorf("save adapter instance: marshal config: %w", err)
	}
	state.Status = strings.TrimSpace(state.Status)
	if state.Status == "" {
		state.Status = "registered"
	}
	state.Health = strings.TrimSpace(state.Health)
	if state.Health == "" {
		state.Health = "unknown"
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO adapter_instances_pg (instance_id, adapter, source, config_json, status, health, online, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (instance_id) DO UPDATE SET
  adapter=excluded.adapter,
  source=excluded.source,
  config_json=excluded.config_json,
  status=excluded.status,
  health=excluded.health,
  online=excluded.online,
  updated_at=excluded.updated_at
`, state.InstanceID, state.Adapter, state.Source, normalizedRawConfig, state.Status, state.Health, state.Online, state.UpdatedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert adapter instance: %w", err)
	}
	return nil
}

func (s *PostgresStore) LoadAdapterInstance(ctx context.Context, instanceID string) (AdapterInstanceState, error) {
	row := s.pool.QueryRow(ctx, `
SELECT instance_id, adapter, source, config_json, status, health, online, updated_at
FROM adapter_instances_pg
WHERE instance_id = $1
`, strings.TrimSpace(instanceID))
	state, err := scanPostgresAdapterInstance(row)
	if err != nil {
		if isPostgresNoRows(err) {
			return AdapterInstanceState{}, sql.ErrNoRows
		}
		return AdapterInstanceState{}, fmt.Errorf("load adapter instance: %w", err)
	}
	return state, nil
}

func (s *PostgresStore) ListAdapterInstances(ctx context.Context) ([]AdapterInstanceState, error) {
	rows, err := s.pool.Query(ctx, `
SELECT instance_id, adapter, source, config_json, status, health, online, updated_at
FROM adapter_instances_pg
ORDER BY instance_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list adapter instances: %w", err)
	}
	defer rows.Close()
	return scanPostgresAdapterInstances(rows)
}

func (s *PostgresStore) SaveIdempotencyKey(ctx context.Context, key, eventID string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO idempotency_keys_pg (idempotency_key, event_id, created_at) VALUES ($1,$2,$3) ON CONFLICT (idempotency_key) DO NOTHING`, key, eventID, time.Now().UTC())
	return err
}

func (s *PostgresStore) HasIdempotencyKey(ctx context.Context, key string) (bool, error) {
	_, found, err := s.FindIdempotencyKey(ctx, key)
	return found, err
}

func (s *PostgresStore) SaveReplayOperationRecord(ctx context.Context, record ReplayOperationRecord) error {
	record.ReplayID = strings.TrimSpace(record.ReplayID)
	if record.ReplayID == "" {
		return fmt.Errorf("save replay operation record: replay id is required")
	}
	record.SourceEventID = strings.TrimSpace(record.SourceEventID)
	if record.SourceEventID == "" {
		return fmt.Errorf("save replay operation record: source event id is required")
	}
	record.ReplayEventID = strings.TrimSpace(record.ReplayEventID)
	if record.ReplayEventID == "" {
		return fmt.Errorf("save replay operation record: replay event id is required")
	}
	record.Status = strings.TrimSpace(record.Status)
	if record.Status == "" {
		return fmt.Errorf("save replay operation record: status is required")
	}
	record.Reason = strings.TrimSpace(record.Reason)
	occurredAt := record.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	updatedAt := record.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = occurredAt
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO replay_operation_records_pg (replay_id, source_event_id, replay_event_id, status, reason, occurred_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (replay_id) DO UPDATE SET
  source_event_id=excluded.source_event_id,
  replay_event_id=excluded.replay_event_id,
  status=excluded.status,
  reason=excluded.reason,
  occurred_at=excluded.occurred_at,
  updated_at=excluded.updated_at
`, record.ReplayID, record.SourceEventID, record.ReplayEventID, record.Status, record.Reason, occurredAt.UTC(), updatedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert replay operation record: %w", err)
	}
	return nil
}

func (s *PostgresStore) ListReplayOperationRecords(ctx context.Context) ([]ReplayOperationRecord, error) {
	rows, err := s.pool.Query(ctx, `
SELECT replay_id, source_event_id, replay_event_id, status, reason, occurred_at, updated_at
FROM replay_operation_records_pg
ORDER BY occurred_at DESC, replay_id DESC
`)
	if err != nil {
		return nil, fmt.Errorf("list replay operation records: %w", err)
	}
	defer rows.Close()
	return scanPostgresReplayOperationRecords(rows)
}

func (s *PostgresStore) SaveRolloutOperationRecord(ctx context.Context, record RolloutOperationRecord) error {
	record.OperationID = strings.TrimSpace(record.OperationID)
	if record.OperationID == "" {
		return fmt.Errorf("save rollout operation record: operation id is required")
	}
	record.PluginID = strings.TrimSpace(record.PluginID)
	if record.PluginID == "" {
		return fmt.Errorf("save rollout operation record: plugin id is required")
	}
	record.Action = strings.TrimSpace(record.Action)
	if record.Action == "" {
		return fmt.Errorf("save rollout operation record: action is required")
	}
	record.Status = strings.TrimSpace(record.Status)
	if record.Status == "" {
		return fmt.Errorf("save rollout operation record: status is required")
	}
	record.Reason = strings.TrimSpace(record.Reason)
	occurredAt := record.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	updatedAt := record.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = occurredAt
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO rollout_operation_records_pg (operation_id, plugin_id, action, current_version, candidate_version, status, reason, occurred_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (operation_id) DO UPDATE SET
  plugin_id=excluded.plugin_id,
  action=excluded.action,
  current_version=excluded.current_version,
  candidate_version=excluded.candidate_version,
  status=excluded.status,
  reason=excluded.reason,
  occurred_at=excluded.occurred_at,
  updated_at=excluded.updated_at
`, record.OperationID, record.PluginID, record.Action, record.CurrentVersion, record.CandidateVersion, record.Status, record.Reason, occurredAt.UTC(), updatedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert rollout operation record: %w", err)
	}
	return nil
}

func (s *PostgresStore) ListRolloutOperationRecords(ctx context.Context) ([]RolloutOperationRecord, error) {
	rows, err := s.pool.Query(ctx, `
SELECT operation_id, plugin_id, action, current_version, candidate_version, status, reason, occurred_at, updated_at
FROM rollout_operation_records_pg
ORDER BY occurred_at DESC, operation_id DESC
`)
	if err != nil {
		return nil, fmt.Errorf("list rollout operation records: %w", err)
	}
	defer rows.Close()
	return scanPostgresRolloutOperationRecords(rows)
}

func (s *PostgresStore) ReplaceCurrentRBACState(ctx context.Context, identities []OperatorIdentityState, snapshot RBACSnapshotState) error {
	if strings.TrimSpace(snapshot.SnapshotKey) == "" {
		snapshot.SnapshotKey = CurrentRBACSnapshotKey
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM operator_identities_pg`); err != nil {
		return fmt.Errorf("clear operator identities: %w", err)
	}
	for _, identity := range identities {
		if err := s.saveOperatorIdentity(ctx, identity); err != nil {
			return err
		}
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM rbac_snapshots_pg WHERE snapshot_key <> $1`, snapshot.SnapshotKey); err != nil {
		return fmt.Errorf("clear stale rbac snapshots: %w", err)
	}
	if err := s.saveRBACSnapshot(ctx, snapshot); err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) saveOperatorIdentity(ctx context.Context, state OperatorIdentityState) error {
	state.ActorID = strings.TrimSpace(state.ActorID)
	if state.ActorID == "" {
		return fmt.Errorf("save operator identity: actor id is required")
	}
	normalizedRoles := normalizeStringSlice(state.Roles)
	rolesJSON, err := json.Marshal(normalizedRoles)
	if err != nil {
		return fmt.Errorf("marshal operator identity roles: %w", err)
	}
	updatedAt := state.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO operator_identities_pg (actor_id, roles_json, updated_at)
VALUES ($1, $2, $3)
ON CONFLICT (actor_id) DO UPDATE SET
  roles_json=excluded.roles_json,
  updated_at=excluded.updated_at
`, state.ActorID, rolesJSON, updatedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert operator identity: %w", err)
	}
	return nil
}

func (s *PostgresStore) LoadOperatorIdentity(ctx context.Context, actorID string) (OperatorIdentityState, error) {
	row := s.pool.QueryRow(ctx, `
SELECT actor_id, roles_json, updated_at
FROM operator_identities_pg
WHERE actor_id = $1
`, strings.TrimSpace(actorID))
	state, err := scanPostgresOperatorIdentity(row)
	if err != nil {
		if isPostgresNoRows(err) {
			return OperatorIdentityState{}, sql.ErrNoRows
		}
		return OperatorIdentityState{}, fmt.Errorf("load operator identity: %w", err)
	}
	return state, nil
}

func (s *PostgresStore) ListOperatorIdentities(ctx context.Context) ([]OperatorIdentityState, error) {
	rows, err := s.pool.Query(ctx, `
SELECT actor_id, roles_json, updated_at
FROM operator_identities_pg
ORDER BY actor_id ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list operator identities: %w", err)
	}
	defer rows.Close()
	return scanPostgresOperatorIdentities(rows)
}

func (s *PostgresStore) saveRBACSnapshot(ctx context.Context, snapshot RBACSnapshotState) error {
	snapshot.SnapshotKey = strings.TrimSpace(snapshot.SnapshotKey)
	if snapshot.SnapshotKey == "" {
		snapshot.SnapshotKey = CurrentRBACSnapshotKey
	}
	policiesJSON, err := json.Marshal(cloneAuthorizationPolicies(snapshot.Policies))
	if err != nil {
		return fmt.Errorf("marshal rbac snapshot policies: %w", err)
	}
	updatedAt := snapshot.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO rbac_snapshots_pg (snapshot_key, console_read_permission, policies_json, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (snapshot_key) DO UPDATE SET
  console_read_permission=excluded.console_read_permission,
  policies_json=excluded.policies_json,
  updated_at=excluded.updated_at
`, snapshot.SnapshotKey, strings.TrimSpace(snapshot.ConsoleReadPermission), policiesJSON, updatedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert rbac snapshot: %w", err)
	}
	return nil
}

func (s *PostgresStore) LoadRBACSnapshot(ctx context.Context, snapshotKey string) (RBACSnapshotState, error) {
	row := s.pool.QueryRow(ctx, `
SELECT snapshot_key, console_read_permission, policies_json, updated_at
FROM rbac_snapshots_pg
WHERE snapshot_key = $1
`, strings.TrimSpace(snapshotKey))
	state, err := scanPostgresRBACSnapshot(row)
	if err != nil {
		if isPostgresNoRows(err) {
			return RBACSnapshotState{}, sql.ErrNoRows
		}
		return RBACSnapshotState{}, fmt.Errorf("load rbac snapshot: %w", err)
	}
	return state, nil
}

func (s *PostgresStore) SaveAudit(ctx context.Context, entry pluginsdk.AuditEntry) error {
	occurredAt, err := parseAuditOccurredAt(entry.OccurredAt)
	if err != nil {
		return fmt.Errorf("parse audit occurred_at: %w", err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_log (actor, permission, action, target, allowed, reason, trace_id, event_id, plugin_id, run_id, correlation_id, error_category, error_code, occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		entry.Actor, entry.Permission, entry.Action, entry.Target, entry.Allowed, auditEntryReason(entry), entry.TraceID, entry.EventID, entry.PluginID, entry.RunID, entry.CorrelationID, entry.ErrorCategory, entry.ErrorCode, occurredAt.UTC())
	return err
}

func (s *PostgresStore) RecordAudit(entry pluginsdk.AuditEntry) error {
	return s.SaveAudit(context.Background(), entry)
}

func (s *PostgresStore) ListAudits(ctx context.Context) ([]pluginsdk.AuditEntry, error) {
	rows, err := s.pool.Query(ctx, `
SELECT actor, permission, action, target, allowed, reason,
       trace_id, event_id, plugin_id, run_id, correlation_id,
       error_category, error_code, occurred_at
FROM audit_log
ORDER BY occurred_at ASC, actor ASC, action ASC, target ASC
`)
	if err != nil {
		return nil, fmt.Errorf("list audits: %w", err)
	}
	defer rows.Close()
	return scanPostgresAuditEntries(rows)
}

func (s *PostgresStore) AuditEntries() []pluginsdk.AuditEntry {
	entries, err := s.ListAudits(context.Background())
	if err != nil {
		return nil
	}
	return entries
}

func (s *PostgresStore) LoadEvent(ctx context.Context, eventID string) (eventmodel.Event, error) {
	row := s.pool.QueryRow(ctx, `SELECT payload_json FROM event_log WHERE event_id = $1`, eventID)
	return loadPostgresEvent(row)
}

func (s *PostgresStore) LoadPluginManifest(ctx context.Context, pluginID string) (pluginsdk.PluginManifest, error) {
	row := s.pool.QueryRow(ctx, `SELECT manifest_json FROM plugin_registry_pg WHERE plugin_id = $1`, pluginID)
	manifest, err := decodePostgresManifest(row)
	if err != nil {
		if isPostgresNoRows(err) {
			return pluginsdk.PluginManifest{}, sql.ErrNoRows
		}
		return pluginsdk.PluginManifest{}, err
	}
	return manifest, nil
}

func (s *PostgresStore) FindIdempotencyKey(ctx context.Context, key string) (string, bool, error) {
	row := s.pool.QueryRow(ctx, `SELECT event_id FROM idempotency_keys_pg WHERE idempotency_key = $1`, key)
	return decodePostgresIdempotencyLookup(row)
}

func (s *PostgresStore) Counts(ctx context.Context) (map[string]int, error) {
	tables := map[string]string{
		"event_journal":             `SELECT COUNT(*) FROM event_log`,
		"plugin_registry":           `SELECT COUNT(*) FROM plugin_registry_pg`,
		"plugin_enabled_overlays":   `SELECT COUNT(*) FROM plugin_enabled_overlays_pg`,
		"plugin_configs":            `SELECT COUNT(*) FROM plugin_configs_pg`,
		"plugin_status_snapshots":   `SELECT COUNT(*) FROM plugin_status_snapshots_pg`,
		"adapter_instances":         `SELECT COUNT(*) FROM adapter_instances_pg`,
		"sessions":                  `SELECT COUNT(*) FROM sessions_pg`,
		"idempotency_keys":          `SELECT COUNT(*) FROM idempotency_keys_pg`,
		"operator_identities":       `SELECT COUNT(*) FROM operator_identities_pg`,
		"rbac_snapshots":            `SELECT COUNT(*) FROM rbac_snapshots_pg`,
		"replay_operation_records":  `SELECT COUNT(*) FROM replay_operation_records_pg`,
		"rollout_operation_records": `SELECT COUNT(*) FROM rollout_operation_records_pg`,
		"audit_log":                 `SELECT COUNT(*) FROM audit_log`,
	}
	counts := make(map[string]int, len(tables))
	for name, query := range tables {
		var count int
		if err := s.pool.QueryRow(ctx, query).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", name, err)
		}
		counts[name] = count
	}
	return counts, nil
}

func decodePostgresIdempotencyLookup(row rowScanner) (string, bool, error) {
	var eventID string
	err := row.Scan(&eventID)
	if err != nil {
		if isPostgresNoRows(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return eventID, true, nil
}

func decodePostgresEvent(row rowScanner) (eventmodel.Event, error) {
	var payload []byte
	if err := row.Scan(&payload); err != nil {
		return eventmodel.Event{}, err
	}
	var event eventmodel.Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return eventmodel.Event{}, fmt.Errorf("decode postgres event payload: %w", err)
	}
	return event, nil
}

func decodePostgresManifest(row rowScanner) (pluginsdk.PluginManifest, error) {
	var payload []byte
	if err := row.Scan(&payload); err != nil {
		return pluginsdk.PluginManifest{}, err
	}
	var manifest pluginsdk.PluginManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return pluginsdk.PluginManifest{}, fmt.Errorf("decode postgres manifest payload: %w", err)
	}
	return manifest, nil
}

func loadPostgresEvent(row rowScanner) (eventmodel.Event, error) {
	event, err := decodePostgresEvent(row)
	if err != nil {
		return eventmodel.Event{}, fmt.Errorf("load event journal: %w", err)
	}
	return event, nil
}

func scanPostgresPluginEnabledState(row rowScanner) (PluginEnabledState, error) {
	var state PluginEnabledState
	var enabled bool
	if err := row.Scan(&state.PluginID, &enabled, &state.UpdatedAt); err != nil {
		return PluginEnabledState{}, err
	}
	state.Enabled = enabled
	state.UpdatedAt = state.UpdatedAt.UTC()
	return state, nil
}

func scanPostgresPluginEnabledStates(rows postgresRows) ([]PluginEnabledState, error) {
	states := make([]PluginEnabledState, 0)
	for rows.Next() {
		state, err := scanPostgresPluginEnabledState(rows)
		if err != nil {
			return nil, fmt.Errorf("scan plugin enabled state: %w", err)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin enabled states: %w", err)
	}
	return states, nil
}

func scanPostgresPluginConfigState(row rowScanner) (PluginConfigState, error) {
	var state PluginConfigState
	var rawConfig []byte
	if err := row.Scan(&state.PluginID, &rawConfig, &state.UpdatedAt); err != nil {
		return PluginConfigState{}, err
	}
	state.RawConfig = append(json.RawMessage(nil), rawConfig...)
	state.UpdatedAt = state.UpdatedAt.UTC()
	return state, nil
}

func scanPostgresPluginConfigStates(rows postgresRows) ([]PluginConfigState, error) {
	states := make([]PluginConfigState, 0)
	for rows.Next() {
		state, err := scanPostgresPluginConfigState(rows)
		if err != nil {
			return nil, fmt.Errorf("scan plugin config: %w", err)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin configs: %w", err)
	}
	return states, nil
}

func scanPostgresPluginStatusSnapshot(row rowScanner) (PluginStatusSnapshot, error) {
	var (
		snapshot      PluginStatusSnapshot
		lastRecovered sql.NullTime
	)
	if err := row.Scan(
		&snapshot.PluginID,
		&snapshot.LastDispatchKind,
		&snapshot.LastDispatchSuccess,
		&snapshot.LastDispatchError,
		&snapshot.LastDispatchAt,
		&lastRecovered,
		&snapshot.LastRecoveryFailureCount,
		&snapshot.CurrentFailureStreak,
		&snapshot.UpdatedAt,
	); err != nil {
		return PluginStatusSnapshot{}, err
	}
	snapshot.LastDispatchAt = snapshot.LastDispatchAt.UTC()
	snapshot.UpdatedAt = snapshot.UpdatedAt.UTC()
	if lastRecovered.Valid {
		recoveredAt := lastRecovered.Time.UTC()
		snapshot.LastRecoveredAt = &recoveredAt
	}
	return snapshot, nil
}

func scanPostgresPluginStatusSnapshots(rows postgresRows) ([]PluginStatusSnapshot, error) {
	snapshots := make([]PluginStatusSnapshot, 0)
	for rows.Next() {
		snapshot, err := scanPostgresPluginStatusSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("scan plugin status snapshot: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin status snapshots: %w", err)
	}
	return snapshots, nil
}

func scanPostgresAdapterInstance(row rowScanner) (AdapterInstanceState, error) {
	var state AdapterInstanceState
	var rawConfig []byte
	if err := row.Scan(&state.InstanceID, &state.Adapter, &state.Source, &rawConfig, &state.Status, &state.Health, &state.Online, &state.UpdatedAt); err != nil {
		return AdapterInstanceState{}, err
	}
	state.RawConfig = append(json.RawMessage(nil), rawConfig...)
	state.UpdatedAt = state.UpdatedAt.UTC()
	return state, nil
}

func scanPostgresAdapterInstances(rows postgresRows) ([]AdapterInstanceState, error) {
	states := make([]AdapterInstanceState, 0)
	for rows.Next() {
		state, err := scanPostgresAdapterInstance(rows)
		if err != nil {
			return nil, fmt.Errorf("scan adapter instance: %w", err)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate adapter instances: %w", err)
	}
	return states, nil
}

func scanPostgresOperatorIdentity(row rowScanner) (OperatorIdentityState, error) {
	var state OperatorIdentityState
	var rolesJSON []byte
	if err := row.Scan(&state.ActorID, &rolesJSON, &state.UpdatedAt); err != nil {
		return OperatorIdentityState{}, err
	}
	if len(rolesJSON) > 0 && string(rolesJSON) != "null" {
		if err := json.Unmarshal(rolesJSON, &state.Roles); err != nil {
			return OperatorIdentityState{}, fmt.Errorf("unmarshal operator identity roles: %w", err)
		}
	}
	state.Roles = normalizeStringSlice(state.Roles)
	state.UpdatedAt = state.UpdatedAt.UTC()
	return state, nil
}

func scanPostgresOperatorIdentities(rows postgresRows) ([]OperatorIdentityState, error) {
	states := make([]OperatorIdentityState, 0)
	for rows.Next() {
		state, err := scanPostgresOperatorIdentity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan operator identity: %w", err)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operator identities: %w", err)
	}
	return states, nil
}

func scanPostgresRBACSnapshot(row rowScanner) (RBACSnapshotState, error) {
	var state RBACSnapshotState
	var policiesJSON []byte
	if err := row.Scan(&state.SnapshotKey, &state.ConsoleReadPermission, &policiesJSON, &state.UpdatedAt); err != nil {
		return RBACSnapshotState{}, err
	}
	if len(policiesJSON) > 0 && string(policiesJSON) != "null" {
		if err := json.Unmarshal(policiesJSON, &state.Policies); err != nil {
			return RBACSnapshotState{}, fmt.Errorf("unmarshal rbac snapshot policies: %w", err)
		}
	}
	state.Policies = cloneAuthorizationPolicies(state.Policies)
	state.UpdatedAt = state.UpdatedAt.UTC()
	return state, nil
}

func scanPostgresReplayOperationRecord(row rowScanner) (ReplayOperationRecord, error) {
	var record ReplayOperationRecord
	if err := row.Scan(&record.ReplayID, &record.SourceEventID, &record.ReplayEventID, &record.Status, &record.Reason, &record.OccurredAt, &record.UpdatedAt); err != nil {
		return ReplayOperationRecord{}, err
	}
	record.OccurredAt = record.OccurredAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	return record, nil
}

func scanPostgresReplayOperationRecords(rows postgresRows) ([]ReplayOperationRecord, error) {
	records := make([]ReplayOperationRecord, 0)
	for rows.Next() {
		record, err := scanPostgresReplayOperationRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan replay operation record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate replay operation records: %w", err)
	}
	return records, nil
}

func scanPostgresRolloutOperationRecord(row rowScanner) (RolloutOperationRecord, error) {
	var record RolloutOperationRecord
	if err := row.Scan(&record.OperationID, &record.PluginID, &record.Action, &record.CurrentVersion, &record.CandidateVersion, &record.Status, &record.Reason, &record.OccurredAt, &record.UpdatedAt); err != nil {
		return RolloutOperationRecord{}, err
	}
	record.OccurredAt = record.OccurredAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	return record, nil
}

func scanPostgresRolloutOperationRecords(rows postgresRows) ([]RolloutOperationRecord, error) {
	records := make([]RolloutOperationRecord, 0)
	for rows.Next() {
		record, err := scanPostgresRolloutOperationRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan rollout operation record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rollout operation records: %w", err)
	}
	return records, nil
}

func scanPostgresAuditEntry(row rowScanner) (pluginsdk.AuditEntry, error) {
	var (
		entry      pluginsdk.AuditEntry
		allowed    bool
		reason     sql.NullString
		occurredAt time.Time
	)
	if err := row.Scan(
		&entry.Actor,
		&entry.Permission,
		&entry.Action,
		&entry.Target,
		&allowed,
		&reason,
		&entry.TraceID,
		&entry.EventID,
		&entry.PluginID,
		&entry.RunID,
		&entry.CorrelationID,
		&entry.ErrorCategory,
		&entry.ErrorCode,
		&occurredAt,
	); err != nil {
		return pluginsdk.AuditEntry{}, err
	}
	entry.Allowed = allowed
	entry.OccurredAt = occurredAt.UTC().Format(time.RFC3339Nano)
	if reason.Valid {
		setAuditEntryReason(&entry, strings.TrimSpace(reason.String))
	}
	return entry, nil
}

func scanPostgresAuditEntries(rows postgresRows) ([]pluginsdk.AuditEntry, error) {
	entries := make([]pluginsdk.AuditEntry, 0)
	for rows.Next() {
		entry, err := scanPostgresAuditEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audits: %w", err)
	}
	return entries, nil
}

func postgresNullableTimestamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func isPostgresNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
