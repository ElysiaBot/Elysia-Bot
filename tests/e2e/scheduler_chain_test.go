package e2e

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
	runtimecore "github.com/ohmyopencode/bot-platform/packages/runtime-core"
	pluginscheduler "github.com/ohmyopencode/bot-platform/plugins/plugin-scheduler"
)

type e2eSchedulerAdapter struct{ inner *runtimecore.Scheduler }

func (s e2eSchedulerAdapter) Register(plan pluginsdk.SchedulePlan) error {
	return s.inner.Register(runtimecore.SchedulePlan{
		ID:        plan.ID,
		Kind:      runtimecore.ScheduleKind(plan.Kind),
		CronExpr:  plan.CronExpr,
		Delay:     plan.Delay,
		ExecuteAt: plan.ExecuteAt,
		Source:    plan.Source,
		EventType: plan.EventType,
		Metadata:  plan.Metadata,
	})
}

func (s e2eSchedulerAdapter) Plan(id string) (pluginsdk.SchedulePlan, error) {
	plan, err := s.inner.Plan(id)
	if err != nil {
		return pluginsdk.SchedulePlan{}, err
	}
	return pluginsdk.SchedulePlan{ID: plan.ID, Kind: pluginsdk.ScheduleKind(plan.Kind), CronExpr: plan.CronExpr, Delay: plan.Delay, ExecuteAt: plan.ExecuteAt, Source: plan.Source, EventType: plan.EventType, Metadata: plan.Metadata}, nil
}

func (s e2eSchedulerAdapter) Trigger(id string) (eventmodel.Event, error) { return s.inner.Trigger(id) }
func (s e2eSchedulerAdapter) Cancel(id string) error                      { return s.inner.Cancel(id) }
func (s e2eSchedulerAdapter) Plans() []pluginsdk.SchedulePlan {
	plans := s.inner.Plans()
	items := make([]pluginsdk.SchedulePlan, 0, len(plans))
	for _, plan := range plans {
		items = append(items, pluginsdk.SchedulePlan{ID: plan.ID, Kind: pluginsdk.ScheduleKind(plan.Kind), CronExpr: plan.CronExpr, Delay: plan.Delay, ExecuteAt: plan.ExecuteAt, Source: plan.Source, EventType: plan.EventType, Metadata: plan.Metadata})
	}
	return items
}

func TestSchedulerRunnerDispatchesThroughRuntimeToPluginSchedulerReply(t *testing.T) {
	t.Parallel()

	scheduler := runtimecore.NewScheduler()

	replies := &recordingReplyService{}
	plugin := pluginscheduler.New(e2eSchedulerAdapter{inner: scheduler}, replies)
	runtime := runtimecore.NewInMemoryRuntime(runtimecore.NoopSupervisor{}, pluginBridgeHost{})
	if err := runtime.RegisterPlugin(plugin.Definition()); err != nil {
		t.Fatalf("register plugin-scheduler: %v", err)
	}

	if err := plugin.CreateDelayPlan("delay-e2e", 1, "scheduled from runtime chain"); err != nil {
		t.Fatalf("create delay plan: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatchErrs := make(chan error, 4)
	fired := make(chan struct{}, 1)
	if err := scheduler.Start(ctx, 5*time.Millisecond, func(event eventmodel.Event) error {
		event.Reply = &eventmodel.ReplyHandle{Capability: "onebot.reply", TargetID: "group-42"}
		select {
		case fired <- struct{}{}:
		default:
		}
		err := runtime.DispatchEvent(context.Background(), event)
		if err != nil {
			dispatchErrs <- err
		}
		return err
	}); err != nil {
		t.Fatalf("start scheduler: %v", err)
	}

	deadline := time.Now().Add(1500 * time.Millisecond)
	for replies.text == "" && time.Now().Before(deadline) {
		select {
		case err := <-dispatchErrs:
			t.Fatalf("scheduler dispatch callback returned error: %v; dispatches=%+v", err, runtime.DispatchResults())
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case <-fired:
	default:
		t.Fatal("expected scheduler runner to fire at least once")
	}

	if replies.text != "scheduled from runtime chain" {
		t.Fatalf("unexpected reply text %q with dispatches %+v", replies.text, runtime.DispatchResults())
	}
	if replies.target != "group-42" {
		t.Fatalf("unexpected reply target %q", replies.target)
	}
	results := runtime.DispatchResults()
	if len(results) != 1 || !results[0].Success || results[0].PluginID != "plugin-scheduler" {
		t.Fatalf("unexpected dispatch results %+v", results)
	}
	if _, err := scheduler.Plan("delay-e2e"); err == nil {
		t.Fatal("expected fired delay plan to be removed after runtime dispatch")
	}
}

func TestSchedulerRunnerRestoresPersistedDelayPlanAndDispatchesThroughPluginScheduler(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "state.db")
	store, err := runtimecore.OpenSQLiteStateStore(storePath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer func() { _ = store.Close() }()

	seedScheduler := runtimecore.NewScheduler()
	seedScheduler.SetStore(store)
	replies := &recordingReplyService{}
	plugin := pluginscheduler.New(e2eSchedulerAdapter{inner: seedScheduler}, replies)
	if err := plugin.CreateDelayPlan("delay-restart-e2e", 1, "restarted runtime chain"); err != nil {
		t.Fatalf("create persisted delay plan: %v", err)
	}

	restartedScheduler := runtimecore.NewScheduler()
	restartedScheduler.SetStore(store)
	runtime := runtimecore.NewInMemoryRuntime(runtimecore.NoopSupervisor{}, pluginBridgeHost{})
	if err := runtime.RegisterPlugin(plugin.Definition()); err != nil {
		t.Fatalf("register plugin-scheduler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatchErrs := make(chan error, 4)
	fired := make(chan struct{}, 1)
	if err := restartedScheduler.Start(ctx, 5*time.Millisecond, func(event eventmodel.Event) error {
		event.Reply = &eventmodel.ReplyHandle{Capability: "onebot.reply", TargetID: "group-42"}
		select {
		case fired <- struct{}{}:
		default:
		}
		err := runtime.DispatchEvent(context.Background(), event)
		if err != nil {
			dispatchErrs <- err
		}
		return err
	}); err != nil {
		t.Fatalf("start restarted scheduler: %v", err)
	}

	deadline := time.Now().Add(1500 * time.Millisecond)
	for replies.text == "" && time.Now().Before(deadline) {
		select {
		case err := <-dispatchErrs:
			t.Fatalf("scheduler dispatch callback returned error: %v; dispatches=%+v", err, runtime.DispatchResults())
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case <-fired:
	default:
		t.Fatal("expected restarted scheduler runner to fire at least once")
	}

	if replies.text != "restarted runtime chain" {
		t.Fatalf("unexpected reply text %q with dispatches %+v", replies.text, runtime.DispatchResults())
	}
	results := runtime.DispatchResults()
	if len(results) != 1 || !results[0].Success || results[0].PluginID != "plugin-scheduler" {
		t.Fatalf("unexpected dispatch results %+v", results)
	}
	if _, err := restartedScheduler.Plan("delay-restart-e2e"); err == nil {
		t.Fatal("expected restored delay plan to be removed after runtime dispatch")
	}
}
