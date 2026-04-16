package e2e

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	adapteronebot "github.com/ohmyopencode/bot-platform/adapters/adapter-onebot"
	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
	runtimecore "github.com/ohmyopencode/bot-platform/packages/runtime-core"
	pluginecho "github.com/ohmyopencode/bot-platform/plugins/plugin-echo"
)

type recordingReplyService struct {
	target string
	text   string
}

func (r *recordingReplyService) ReplyText(handle eventmodel.ReplyHandle, text string) error {
	r.target = handle.TargetID
	r.text = text
	return nil
}

func (r *recordingReplyService) ReplyImage(handle eventmodel.ReplyHandle, imageURL string) error {
	return nil
}

func (r *recordingReplyService) ReplyFile(handle eventmodel.ReplyHandle, fileURL string) error {
	return nil
}

type pluginBridgeHost struct{}

func (pluginBridgeHost) DispatchEvent(ctx context.Context, plugin pluginsdk.Plugin, event eventmodel.Event, executionContext eventmodel.ExecutionContext) error {
	if plugin.Handlers.Event == nil {
		return nil
	}
	return plugin.Handlers.Event.OnEvent(event, executionContext)
}

func (pluginBridgeHost) DispatchCommand(ctx context.Context, plugin pluginsdk.Plugin, command eventmodel.CommandInvocation, executionContext eventmodel.ExecutionContext) error {
	if plugin.Handlers.Command == nil {
		return nil
	}
	return plugin.Handlers.Command.OnCommand(command, executionContext)
}

func (pluginBridgeHost) DispatchJob(ctx context.Context, plugin pluginsdk.Plugin, job pluginsdk.JobInvocation, executionContext eventmodel.ExecutionContext) error {
	if plugin.Handlers.Job == nil {
		return nil
	}
	return plugin.Handlers.Job.OnJob(job, executionContext)
}

func (pluginBridgeHost) DispatchSchedule(ctx context.Context, plugin pluginsdk.Plugin, trigger pluginsdk.ScheduleTrigger, executionContext eventmodel.ExecutionContext) error {
	if plugin.Handlers.Schedule == nil {
		return nil
	}
	return plugin.Handlers.Schedule.OnSchedule(trigger, executionContext)
}

func TestFirstChainOneBotToEchoReplyAndLogs(t *testing.T) {
	t.Parallel()

	ingressLogs := &bytes.Buffer{}
	converter := adapteronebot.NewIngressConverter(ingressLogs)
	converter.NowForTest(func() time.Time {
		return time.Date(2026, 4, 2, 18, 0, 0, 0, time.UTC)
	})

	replies := &recordingReplyService{}
	echoPlugin := pluginecho.New(replies, pluginecho.Config{Prefix: "echo: "})

	runtime := runtimecore.NewInMemoryRuntime(runtimecore.NoopSupervisor{}, pluginBridgeHost{})
	if err := runtime.RegisterPlugin(echoPlugin.Definition()); err != nil {
		t.Fatalf("register echo plugin: %v", err)
	}

	event, err := converter.ConvertMessageEvent(adapteronebot.MessageEventPayload{
		PostType:    "message",
		MessageType: "group",
		Time:        1712034000,
		UserID:      10001,
		GroupID:     42,
		MessageID:   9001,
		RawMessage:  "hello platform",
		Sender:      adapteronebot.SenderPayload{Nickname: "alice"},
	})
	if err != nil {
		t.Fatalf("convert ingress event: %v", err)
	}

	if err := runtime.DispatchEvent(context.Background(), event); err != nil {
		t.Fatalf("dispatch runtime event: %v", err)
	}

	if replies.target != "group-42" {
		t.Fatalf("unexpected reply target %q", replies.target)
	}
	if replies.text != "echo: hello platform" {
		t.Fatalf("unexpected reply text %q", replies.text)
	}

	results := runtime.DispatchResults()
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("unexpected dispatch results %+v", results)
	}

	logOutput := ingressLogs.String()
	for _, expected := range []string{"trace_id", "event_id", "onebot ingress mapped to standard event"} {
		if !strings.Contains(logOutput, expected) {
			t.Fatalf("expected ingress log to contain %q, got %s", expected, logOutput)
		}
	}
}
