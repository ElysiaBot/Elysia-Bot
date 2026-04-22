package runtimecore

import (
	"context"
	stdsql "database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

const postgresTestDSNEnv = "BOT_PLATFORM_POSTGRES_TEST_DSN"

type fakeRow struct {
	values []any
	err    error
}

type fakeRows struct {
	rows  [][]any
	index int
	err   error
}

type fakePostgresPool struct {
	execSQL      []string
	execArgs     [][]any
	execErr      error
	querySQL     []string
	queryArgs    [][]any
	queryRows    pgx.Rows
	queryRow     pgx.Row
	closed       bool
	commandTag   pgconn.CommandTag
	queryFunc    func(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	queryRowFunc func(ctx context.Context, sql string, arguments ...any) pgx.Row
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(value))
	}
	return nil
}

func (r *fakeRows) Next() bool {
	if r == nil {
		return false
	}
	return r.index < len(r.rows)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r == nil {
		return errors.New("fake rows is nil")
	}
	if r.err != nil {
		return r.err
	}
	if r.index >= len(r.rows) {
		return errors.New("scan past rows")
	}
	for i, value := range r.rows[r.index] {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(value))
	}
	r.index++
	return nil
}

func (r *fakeRows) Err() error {
	if r == nil {
		return nil
	}
	return r.err
}

func (r *fakeRows) Close() {}

func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, errors.New("not implemented") }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

func (p *fakePostgresPool) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	p.execSQL = append(p.execSQL, sql)
	p.execArgs = append(p.execArgs, append([]any(nil), arguments...))
	return p.commandTag, p.execErr
}

func (p *fakePostgresPool) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	p.querySQL = append(p.querySQL, sql)
	p.queryArgs = append(p.queryArgs, append([]any(nil), arguments...))
	if p.queryRowFunc != nil {
		return p.queryRowFunc(ctx, sql, arguments...)
	}
	return p.queryRow
}

func (p *fakePostgresPool) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	p.querySQL = append(p.querySQL, sql)
	p.queryArgs = append(p.queryArgs, append([]any(nil), arguments...))
	if p.queryFunc != nil {
		return p.queryFunc(ctx, sql, arguments...)
	}
	if p.queryRows != nil {
		return p.queryRows, nil
	}
	return nil, errors.New("unexpected query")
}

func (p *fakePostgresPool) Close() {
	p.closed = true
}

func TestWritePostgresMigrationCreatesSchemaFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "postgres", "001_init.sql")
	if err := WritePostgresMigration(path); err != nil {
		t.Fatalf("write postgres migration: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read postgres migration: %v", err)
	}
	content := string(raw)
	for _, expected := range []string{"event_log", "job_state", "workflow_state", "plugin_registry_pg", "idempotency_keys_pg", "audit_log"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected migration to contain %q, got %s", expected, content)
		}
	}
}

func TestPostgresStoreSaveMethodsIssueExpectedWrites(t *testing.T) {
	t.Parallel()

	pool := &fakePostgresPool{queryRow: fakeRow{err: pgx.ErrNoRows}}
	store := &PostgresStore{pool: pool}
	ctx := context.Background()
	event := eventmodel.Event{EventID: "evt-pg", TraceID: "trace-pg", Source: "webhook", Type: "webhook.received", Timestamp: time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC), IdempotencyKey: "webhook:pg:1"}
	job := Job{ID: "job-pg", Type: "ai.chat", Status: JobStatusPending, Payload: map[string]any{"prompt": "hi"}, MaxRetries: 3, Timeout: 30 * time.Second, CreatedAt: time.Date(2026, 4, 6, 10, 1, 0, 0, time.UTC)}
	workflow := Workflow{ID: "wf-pg", CurrentIndex: 1, WaitingFor: "message.received", Completed: false, Compensated: false, State: map[string]any{"step": "wait"}}
	manifest := pluginsdk.PluginManifest{ID: "plugin-echo", Name: "Echo Plugin", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess}
	audit := pluginsdk.AuditEntry{
		Actor:         "admin-user",
		Permission:    "plugin:enable",
		Action:        "enable",
		Target:        "plugin-echo",
		Allowed:       true,
		TraceID:       "trace-pg-audit",
		EventID:       "evt-pg-audit",
		PluginID:      "plugin-echo",
		RunID:         "run-pg-audit",
		CorrelationID: "corr-pg-audit",
		ErrorCategory: "operator",
		ErrorCode:     "rollout_prepared",
		OccurredAt:    "2026-04-06T10:02:00Z",
	}
	setAuditEntryReason(&audit, "rollout_prepared")
	adapterConfig := json.RawMessage(`{"mode":"demo-ingress"}`)

	if err := store.SaveEvent(ctx, event); err != nil {
		t.Fatalf("save event: %v", err)
	}
	if err := store.SaveJob(ctx, job); err != nil {
		t.Fatalf("save job: %v", err)
	}
	if err := store.SaveWorkflow(ctx, workflow); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	if err := store.SavePluginManifest(ctx, manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	if err := store.SaveIdempotencyKey(ctx, "idem-1", event.EventID); err != nil {
		t.Fatalf("save idempotency key: %v", err)
	}
	if err := store.SaveAudit(ctx, audit); err != nil {
		t.Fatalf("save audit: %v", err)
	}
	if err := store.SavePluginEnabledState(ctx, "plugin-echo", false); err != nil {
		t.Fatalf("save plugin enabled state: %v", err)
	}
	if err := store.SavePluginConfig(ctx, "plugin-echo", json.RawMessage(`{"prefix":"persisted: "}`)); err != nil {
		t.Fatalf("save plugin config: %v", err)
	}
	if err := store.SavePluginStatusSnapshot(ctx, DispatchResult{PluginID: "plugin-echo", Kind: "event", Success: false, Error: "timeout", At: time.Date(2026, 4, 6, 10, 3, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("save plugin status snapshot: %v", err)
	}
	if err := store.SaveSession(ctx, SessionState{SessionID: "session-1", PluginID: "plugin-ai-chat", State: map[string]any{"topic": "hello"}}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := store.SaveAdapterInstance(ctx, AdapterInstanceState{InstanceID: "adapter-onebot-demo", Adapter: "onebot", Source: "onebot", RawConfig: adapterConfig, Status: "registered", Health: "ready", Online: true}); err != nil {
		t.Fatalf("save adapter instance: %v", err)
	}
	if err := store.SaveReplayOperationRecord(ctx, ReplayOperationRecord{ReplayID: "replay-op-1", SourceEventID: event.EventID, ReplayEventID: "replay-evt-pg", Status: "succeeded", Reason: "replay_dispatched"}); err != nil {
		t.Fatalf("save replay operation record: %v", err)
	}
	if err := store.SaveRolloutOperationRecord(ctx, RolloutOperationRecord{OperationID: "rollout-op-1", PluginID: "plugin-echo", Action: "prepare", CurrentVersion: "0.1.0", CandidateVersion: "0.2.0-candidate", Status: "prepared"}); err != nil {
		t.Fatalf("save rollout operation record: %v", err)
	}
	if err := store.ReplaceCurrentRBACState(ctx, []OperatorIdentityState{{ActorID: "viewer-user", Roles: []string{"viewer"}}}, RBACSnapshotState{SnapshotKey: CurrentRBACSnapshotKey, ConsoleReadPermission: "console:read", Policies: map[string]pluginsdk.AuthorizationPolicy{"viewer": {Permissions: []string{"console:read"}, PluginScope: []string{"console"}}}}); err != nil {
		t.Fatalf("replace current rbac state: %v", err)
	}

	if len(pool.execSQL) != 17 {
		t.Fatalf("expected 17 exec calls, got %d", len(pool.execSQL))
	}
	for _, expected := range []string{"INSERT INTO event_log", "INSERT INTO job_state", "INSERT INTO workflow_state", "INSERT INTO plugin_registry_pg", "INSERT INTO idempotency_keys_pg", "INSERT INTO audit_log", "INSERT INTO plugin_enabled_overlays_pg", "INSERT INTO plugin_configs_pg", "INSERT INTO plugin_status_snapshots_pg", "INSERT INTO sessions_pg", "INSERT INTO adapter_instances_pg", "INSERT INTO replay_operation_records_pg", "INSERT INTO rollout_operation_records_pg", "INSERT INTO operator_identities_pg", "INSERT INTO rbac_snapshots_pg"} {
		matched := false
		for _, sql := range pool.execSQL {
			if strings.Contains(sql, expected) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("expected exec statements to include %q, got %+v", expected, pool.execSQL)
		}
	}
	if len(pool.execArgs[0]) != 6 || pool.execArgs[0][0] != event.EventID {
		t.Fatalf("unexpected event exec args %+v", pool.execArgs[0])
	}
	if len(pool.execArgs[1]) != 13 || pool.execArgs[1][0] != job.ID || pool.execArgs[1][1] != job.Type {
		t.Fatalf("unexpected job exec args %+v", pool.execArgs[1])
	}
	if len(pool.execArgs[4]) != 3 || pool.execArgs[4][0] != "idem-1" || pool.execArgs[4][1] != event.EventID {
		t.Fatalf("unexpected idempotency exec args %+v", pool.execArgs[4])
	}
	if len(pool.execArgs[5]) != 14 || pool.execArgs[5][1] != "plugin:enable" || pool.execArgs[5][5] != "rollout_prepared" || pool.execArgs[5][6] != "trace-pg-audit" || pool.execArgs[5][7] != "evt-pg-audit" || pool.execArgs[5][8] != "plugin-echo" || pool.execArgs[5][9] != "run-pg-audit" || pool.execArgs[5][10] != "corr-pg-audit" || pool.execArgs[5][11] != "operator" || pool.execArgs[5][12] != "rollout_prepared" {
		t.Fatalf("unexpected audit exec args %+v", pool.execArgs[5])
	}
}

func TestPostgresStoreLoadMethodsIssueExpectedQueries(t *testing.T) {
	t.Parallel()

	eventPayload, _ := json.Marshal(eventmodel.Event{EventID: "evt-load", TraceID: "trace-load", Source: "webhook", Type: "webhook.received", Timestamp: time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC), IdempotencyKey: "webhook:load:1"})
	manifestPayload, _ := json.Marshal(pluginsdk.PluginManifest{ID: "plugin-echo", Name: "Echo Plugin", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess})
	pool := &fakePostgresPool{queryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(sql, "FROM event_log"):
			return fakeRow{values: []any{eventPayload}}
		case strings.Contains(sql, "FROM plugin_registry_pg"):
			return fakeRow{values: []any{manifestPayload}}
		case strings.Contains(sql, "FROM idempotency_keys_pg"):
			return fakeRow{values: []any{"evt-load"}}
		default:
			return fakeRow{err: errors.New("unexpected query")}
		}
	}}
	store := &PostgresStore{pool: pool}
	ctx := context.Background()

	event, err := store.LoadEvent(ctx, "evt-load")
	if err != nil || event.EventID != "evt-load" {
		t.Fatalf("load event: event=%+v err=%v", event, err)
	}
	manifest, err := store.LoadPluginManifest(ctx, "plugin-echo")
	if err != nil || manifest.ID != "plugin-echo" {
		t.Fatalf("load manifest: manifest=%+v err=%v", manifest, err)
	}
	eventID, found, err := store.FindIdempotencyKey(ctx, "idem-load")
	if err != nil || !found || eventID != "evt-load" {
		t.Fatalf("find idempotency key: eventID=%q found=%v err=%v", eventID, found, err)
	}

	if len(pool.querySQL) != 3 {
		t.Fatalf("expected 3 query calls, got %d", len(pool.querySQL))
	}
	if !strings.Contains(pool.querySQL[0], "FROM event_log") || !strings.Contains(pool.querySQL[1], "FROM plugin_registry_pg") || !strings.Contains(pool.querySQL[2], "FROM idempotency_keys_pg") {
		t.Fatalf("unexpected query SQLs %+v", pool.querySQL)
	}
}

func TestPostgresStoreControlStateReadbacks(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 4, 6, 11, 30, 0, 0, time.UTC)
	recoveredAt := updatedAt.Add(2 * time.Minute)
	pool := &fakePostgresPool{queryRowFunc: func(_ context.Context, query string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(query, "FROM plugin_enabled_overlays_pg"):
			return fakeRow{values: []any{"plugin-echo", false, updatedAt}}
		case strings.Contains(query, "FROM plugin_configs_pg"):
			return fakeRow{values: []any{"plugin-echo", []byte(`{"prefix":"persisted: "}`), updatedAt}}
		case strings.Contains(query, "FROM plugin_status_snapshots_pg"):
			return fakeRow{values: []any{"plugin-echo", "event", true, "", updatedAt, stdsql.NullTime{Time: recoveredAt, Valid: true}, 1, 0, updatedAt}}
		case strings.Contains(query, "FROM adapter_instances_pg"):
			return fakeRow{values: []any{"adapter-onebot-demo", "onebot", "onebot", []byte(`{"mode":"demo-ingress"}`), "registered", "ready", true, updatedAt}}
		case strings.Contains(query, "FROM operator_identities_pg"):
			return fakeRow{values: []any{"viewer-user", []byte(`[
"viewer"]`), updatedAt}}
		case strings.Contains(query, "FROM rbac_snapshots_pg"):
			return fakeRow{values: []any{CurrentRBACSnapshotKey, "console:read", []byte(`{"viewer":{"permissions":["console:read"],"plugin_scope":["console"]}}`), updatedAt}}
		default:
			return fakeRow{err: errors.New("unexpected query")}
		}
	}}
	store := &PostgresStore{pool: pool}
	ctx := context.Background()

	enabled, err := store.LoadPluginEnabledState(ctx, "plugin-echo")
	if err != nil || enabled.PluginID != "plugin-echo" || enabled.Enabled {
		t.Fatalf("load plugin enabled state: state=%+v err=%v", enabled, err)
	}
	config, err := store.LoadPluginConfig(ctx, "plugin-echo")
	if err != nil || config.PluginID != "plugin-echo" || string(config.RawConfig) != `{"prefix":"persisted: "}` {
		t.Fatalf("load plugin config: state=%+v err=%v", config, err)
	}
	snapshot, err := store.LoadPluginStatusSnapshot(ctx, "plugin-echo")
	if err != nil || snapshot.PluginID != "plugin-echo" || !snapshot.LastDispatchSuccess || snapshot.LastRecoveredAt == nil || !snapshot.LastRecoveredAt.Equal(recoveredAt) {
		t.Fatalf("load plugin status snapshot: snapshot=%+v err=%v", snapshot, err)
	}
	adapter, err := store.LoadAdapterInstance(ctx, "adapter-onebot-demo")
	if err != nil || adapter.InstanceID != "adapter-onebot-demo" || adapter.Adapter != "onebot" || !adapter.Online {
		t.Fatalf("load adapter instance: state=%+v err=%v", adapter, err)
	}
	identity, err := store.LoadOperatorIdentity(ctx, "viewer-user")
	if err != nil || identity.ActorID != "viewer-user" || len(identity.Roles) != 1 || identity.Roles[0] != "viewer" {
		t.Fatalf("load operator identity: state=%+v err=%v", identity, err)
	}
	rbac, err := store.LoadRBACSnapshot(ctx, CurrentRBACSnapshotKey)
	if err != nil || rbac.ConsoleReadPermission != "console:read" || rbac.Policies["viewer"].Permissions[0] != "console:read" {
		t.Fatalf("load rbac snapshot: state=%+v err=%v", rbac, err)
	}
}

func TestPostgresStoreControlStateListReadbacks(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	occurredAt := updatedAt.Add(-5 * time.Minute)
	pool := &fakePostgresPool{queryFunc: func(_ context.Context, query string, _ ...any) (pgx.Rows, error) {
		switch {
		case strings.Contains(query, "FROM plugin_enabled_overlays_pg"):
			return &fakeRows{rows: [][]any{{"plugin-echo", false, updatedAt}}}, nil
		case strings.Contains(query, "FROM plugin_configs_pg"):
			return &fakeRows{rows: [][]any{{"plugin-echo", []byte(`{"prefix":"persisted: "}`), updatedAt}}}, nil
		case strings.Contains(query, "FROM plugin_status_snapshots_pg"):
			return &fakeRows{rows: [][]any{{"plugin-echo", "event", false, "timeout", occurredAt, stdsql.NullTime{}, 0, 1, updatedAt}}}, nil
		case strings.Contains(query, "FROM adapter_instances_pg"):
			return &fakeRows{rows: [][]any{{"adapter-onebot-demo", "onebot", "onebot", []byte(`{"mode":"demo-ingress"}`), "registered", "ready", true, updatedAt}}}, nil
		case strings.Contains(query, "FROM operator_identities_pg"):
			return &fakeRows{rows: [][]any{{"viewer-user", []byte(`[
"viewer"]`), updatedAt}}}, nil
		case strings.Contains(query, "FROM replay_operation_records_pg"):
			return &fakeRows{rows: [][]any{{"replay-op-1", "evt-source-1", "replay-evt-1", "succeeded", "replay_dispatched", occurredAt, updatedAt}}}, nil
		case strings.Contains(query, "FROM rollout_operation_records_pg"):
			return &fakeRows{rows: [][]any{{"rollout-op-1", "plugin-echo", "prepare", "0.1.0", "0.2.0-candidate", "prepared", "", occurredAt, updatedAt}}}, nil
		case strings.Contains(query, "FROM audit_log"):
			return &fakeRows{rows: [][]any{{"admin-user", "plugin:enable", "enable", "plugin-echo", true, stdsql.NullString{String: "rollout_prepared", Valid: true}, "trace-1", "evt-1", "plugin-echo", "run-1", "corr-1", "operator", "rollout_prepared", updatedAt}}}, nil
		default:
			return nil, errors.New("unexpected query")
		}
	}}
	store := &PostgresStore{pool: pool}
	ctx := context.Background()

	enabledStates, err := store.ListPluginEnabledStates(ctx)
	if err != nil || len(enabledStates) != 1 || enabledStates[0].PluginID != "plugin-echo" {
		t.Fatalf("list plugin enabled states: states=%+v err=%v", enabledStates, err)
	}
	configs, err := store.ListPluginConfigs(ctx)
	if err != nil || len(configs) != 1 || string(configs[0].RawConfig) != `{"prefix":"persisted: "}` {
		t.Fatalf("list plugin configs: states=%+v err=%v", configs, err)
	}
	snapshots, err := store.ListPluginStatusSnapshots(ctx)
	if err != nil || len(snapshots) != 1 || snapshots[0].CurrentFailureStreak != 1 {
		t.Fatalf("list plugin status snapshots: states=%+v err=%v", snapshots, err)
	}
	adapters, err := store.ListAdapterInstances(ctx)
	if err != nil || len(adapters) != 1 || adapters[0].InstanceID != "adapter-onebot-demo" {
		t.Fatalf("list adapter instances: states=%+v err=%v", adapters, err)
	}
	identities, err := store.ListOperatorIdentities(ctx)
	if err != nil || len(identities) != 1 || identities[0].ActorID != "viewer-user" {
		t.Fatalf("list operator identities: states=%+v err=%v", identities, err)
	}
	replayRecords, err := store.ListReplayOperationRecords(ctx)
	if err != nil || len(replayRecords) != 1 || replayRecords[0].ReplayID != "replay-op-1" {
		t.Fatalf("list replay operation records: states=%+v err=%v", replayRecords, err)
	}
	rolloutRecords, err := store.ListRolloutOperationRecords(ctx)
	if err != nil || len(rolloutRecords) != 1 || rolloutRecords[0].OperationID != "rollout-op-1" {
		t.Fatalf("list rollout operation records: states=%+v err=%v", rolloutRecords, err)
	}
	audits, err := store.ListAudits(ctx)
	if err != nil || len(audits) != 1 || audits[0].Reason != "rollout_prepared" {
		t.Fatalf("list audits: states=%+v err=%v", audits, err)
	}
	if len(store.AuditEntries()) != 1 || store.AuditEntries()[0].Action != "enable" {
		t.Fatalf("audit entries helper: %+v", store.AuditEntries())
	}
}

func TestPostgresStoreCountsReadsSmokeTables(t *testing.T) {
	t.Parallel()

	pool := &fakePostgresPool{queryRowFunc: func(_ context.Context, query string, _ ...any) pgx.Row {
		switch {
		case strings.Contains(query, "FROM event_log"):
			return fakeRow{values: []any{3}}
		case strings.Contains(query, "FROM plugin_registry_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM plugin_enabled_overlays_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM plugin_configs_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM plugin_status_snapshots_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM adapter_instances_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM sessions_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM idempotency_keys_pg"):
			return fakeRow{values: []any{5}}
		case strings.Contains(query, "FROM operator_identities_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM rbac_snapshots_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM replay_operation_records_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM rollout_operation_records_pg"):
			return fakeRow{values: []any{1}}
		case strings.Contains(query, "FROM audit_log"):
			return fakeRow{values: []any{1}}
		default:
			return fakeRow{err: errors.New("unexpected query")}
		}
	}}
	store := &PostgresStore{pool: pool}
	counts, err := store.Counts(context.Background())
	if err != nil {
		t.Fatalf("count postgres smoke tables: %v", err)
	}
	if counts["event_journal"] != 3 || counts["idempotency_keys"] != 5 || counts["plugin_configs"] != 1 || counts["audit_log"] != 1 {
		t.Fatalf("unexpected counts %+v", counts)
	}
	if len(pool.querySQL) != 13 {
		t.Fatalf("expected 2 count queries, got %+v", pool.querySQL)
	}
}

func TestPostgresStoreInitAndCloseDelegateToPool(t *testing.T) {
	t.Parallel()

	pool := &fakePostgresPool{}
	store := &PostgresStore{pool: pool}
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if len(pool.execSQL) != 1 || !strings.Contains(pool.execSQL[0], "CREATE TABLE IF NOT EXISTS event_log") {
		t.Fatalf("expected init to execute schema, got %+v", pool.execSQL)
	}
	store.Close()
	if !pool.closed {
		t.Fatal("expected close to delegate to pool")
	}
}

func TestPostgresStoreLiveRoundTrip(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(postgresTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run live Postgres smoke test", postgresTestDSNEnv)
	}

	ctx := context.Background()
	store, err := OpenPostgresStore(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres store: %v", err)
	}
	defer store.Close()

	eventID := fmt.Sprintf("evt-live-%d", time.Now().UnixNano())
	event := eventmodel.Event{
		EventID:        eventID,
		TraceID:        fmt.Sprintf("trace-live-%d", time.Now().UnixNano()),
		Source:         "webhook",
		Type:           "webhook.received",
		Timestamp:      time.Now().UTC(),
		IdempotencyKey: fmt.Sprintf("idem-live-%s", eventID),
		Metadata:       map[string]any{"smoke": true},
	}

	if err := store.SaveEvent(ctx, event); err != nil {
		t.Fatalf("save live event: %v", err)
	}
	loaded, err := store.LoadEvent(ctx, eventID)
	if err != nil {
		t.Fatalf("load live event: %v", err)
	}
	if loaded.EventID != event.EventID || loaded.TraceID != event.TraceID || loaded.Type != event.Type {
		t.Fatalf("unexpected loaded event %+v", loaded)
	}

	if err := store.SaveIdempotencyKey(ctx, event.IdempotencyKey, event.EventID); err != nil {
		t.Fatalf("save live idempotency key: %v", err)
	}
	lookupEventID, found, err := store.FindIdempotencyKey(ctx, event.IdempotencyKey)
	if err != nil {
		t.Fatalf("find live idempotency key: %v", err)
	}
	if !found || lookupEventID != event.EventID {
		t.Fatalf("unexpected live idempotency lookup eventID=%q found=%v", lookupEventID, found)
	}

	manifest := pluginsdk.PluginManifest{
		ID:         fmt.Sprintf("plugin-live-%d", time.Now().UnixNano()),
		Name:       "Live Smoke Plugin",
		Version:    "0.1.0",
		APIVersion: "v0",
		Mode:       pluginsdk.ModeSubprocess,
		Permissions: []string{
			"message:read",
		},
	}
	if err := store.SavePluginManifest(ctx, manifest); err != nil {
		t.Fatalf("save live manifest: %v", err)
	}
	loadedManifest, err := store.LoadPluginManifest(ctx, manifest.ID)
	if err != nil {
		t.Fatalf("load live manifest: %v", err)
	}
	if loadedManifest.ID != manifest.ID || loadedManifest.Version != manifest.Version || loadedManifest.Mode != manifest.Mode {
		t.Fatalf("unexpected live manifest %+v", loadedManifest)
	}
}

func TestDecodePostgresEventAndManifestPayloads(t *testing.T) {
	t.Parallel()

	eventPayload, _ := json.Marshal(eventmodel.Event{EventID: "evt-pg", TraceID: "trace-pg", Source: "webhook", Type: "webhook.received", Timestamp: time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC), IdempotencyKey: "webhook:pg:1"})
	event, err := decodePostgresEvent(fakeRow{values: []any{eventPayload}})
	if err != nil || event.EventID != "evt-pg" || event.TraceID != "trace-pg" {
		t.Fatalf("expected postgres event payload decode to succeed, got event=%+v err=%v", event, err)
	}

	manifestPayload, _ := json.Marshal(pluginsdk.PluginManifest{ID: "plugin-echo", Version: "0.1.0", APIVersion: "v0", Mode: pluginsdk.ModeSubprocess})
	manifest, err := decodePostgresManifest(fakeRow{values: []any{manifestPayload}})
	if err != nil || manifest.ID != "plugin-echo" || manifest.APIVersion != "v0" {
		t.Fatalf("expected postgres manifest payload decode to succeed, got manifest=%+v err=%v", manifest, err)
	}
}

func TestDecodePostgresIdempotencyLookupHandlesHitAndMiss(t *testing.T) {
	t.Parallel()

	eventID, found, err := decodePostgresIdempotencyLookup(fakeRow{values: []any{"evt-pg-hit"}})
	if err != nil || !found || eventID != "evt-pg-hit" {
		t.Fatalf("expected idempotency hit, got eventID=%q found=%v err=%v", eventID, found, err)
	}

	eventID, found, err = decodePostgresIdempotencyLookup(fakeRow{err: pgx.ErrNoRows})
	if err != nil || found || eventID != "" {
		t.Fatalf("expected idempotency miss, got eventID=%q found=%v err=%v", eventID, found, err)
	}
}

func TestDecodePostgresIdempotencyLookupDoesNotTreatArbitraryErrorAsMiss(t *testing.T) {
	t.Parallel()

	rowErr := errors.New("transport failed: no rows from upstream")
	_, found, err := decodePostgresIdempotencyLookup(fakeRow{err: rowErr})
	if err == nil || found || !errors.Is(err, rowErr) {
		t.Fatalf("expected arbitrary error to propagate, got found=%v err=%v", found, err)
	}
}

func TestDecodePostgresEventAndManifestPropagateNoRows(t *testing.T) {
	t.Parallel()

	if _, err := decodePostgresEvent(fakeRow{err: pgx.ErrNoRows}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected event no-row error to propagate, got %v", err)
	}
	if _, err := decodePostgresManifest(fakeRow{err: pgx.ErrNoRows}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected manifest no-row error to propagate, got %v", err)
	}
}

func TestPostgresLoadEventWrapsReplayJournalReadErrors(t *testing.T) {
	t.Parallel()

	_, err := loadPostgresEvent(fakeRow{err: pgx.ErrNoRows})
	if err == nil || !strings.Contains(err.Error(), "load event journal") || !strings.Contains(err.Error(), pgx.ErrNoRows.Error()) {
		t.Fatalf("expected replay-facing load error wrapping, got %v", err)
	}
}

func TestDecodePostgresHelpersPropagateUnexpectedErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	if _, err := decodePostgresEvent(fakeRow{err: boom}); !errors.Is(err, boom) {
		t.Fatalf("expected event decode to propagate error, got %v", err)
	}
	if _, err := decodePostgresManifest(fakeRow{err: boom}); !errors.Is(err, boom) {
		t.Fatalf("expected manifest decode to propagate error, got %v", err)
	}
	if _, _, err := decodePostgresIdempotencyLookup(fakeRow{err: boom}); !errors.Is(err, boom) {
		t.Fatalf("expected idempotency decode to propagate error, got %v", err)
	}
}
