package pluginworkflowdemo

import (
	"context"
	"testing"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

type replyRecorder struct {
	texts []string
}

func (r *replyRecorder) ReplyText(handle eventmodel.ReplyHandle, text string) error {
	r.texts = append(r.texts, text)
	return nil
}

func (r *replyRecorder) ReplyImage(handle eventmodel.ReplyHandle, imageURL string) error { return nil }
func (r *replyRecorder) ReplyFile(handle eventmodel.ReplyHandle, fileURL string) error   { return nil }

type sessionRecorder struct {
	sessions []pluginsdk.SessionState
}

func (s *sessionRecorder) SaveSession(ctx context.Context, session pluginsdk.SessionState) error {
	s.sessions = append(s.sessions, session)
	return nil
}

func TestWorkflowDemoStartsAndResumesWorkflow(t *testing.T) {
	t.Parallel()

	replies := &replyRecorder{}
	sessions := &sessionRecorder{}
	plugin := New(replies, sessions)
	plugin.Now = func() time.Time { return time.Date(2026, 4, 3, 16, 0, 0, 0, time.UTC) }

	first := eventmodel.Event{
		EventID:        "evt-1",
		TraceID:        "trace-1",
		Source:         "onebot",
		Type:           "message.received",
		Timestamp:      plugin.Now(),
		IdempotencyKey: "msg:1",
		Actor:          &eventmodel.Actor{ID: "user-1", Type: "user"},
		Message:        &eventmodel.Message{Text: "start workflow"},
	}
	ctx := eventmodel.ExecutionContext{TraceID: "trace-1", EventID: "evt-1", Reply: &eventmodel.ReplyHandle{Capability: "onebot.reply", TargetID: "group-42"}}

	if err := plugin.OnEvent(first, ctx); err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	if replies.texts[0] != "workflow started, please send another message to continue" {
		t.Fatalf("unexpected first reply %+v", replies.texts)
	}

	second := eventmodel.Event{
		EventID:        "evt-2",
		TraceID:        "trace-2",
		Source:         "onebot",
		Type:           "message.received",
		Timestamp:      plugin.Now(),
		IdempotencyKey: "msg:2",
		Actor:          &eventmodel.Actor{ID: "user-1", Type: "user"},
		Message:        &eventmodel.Message{Text: "continue"},
	}
	ctx2 := eventmodel.ExecutionContext{TraceID: "trace-2", EventID: "evt-2", Reply: &eventmodel.ReplyHandle{Capability: "onebot.reply", TargetID: "group-42"}}

	if err := plugin.OnEvent(second, ctx2); err != nil {
		t.Fatalf("resume workflow: %v", err)
	}
	if replies.texts[1] != "workflow resumed and completed" {
		t.Fatalf("unexpected second reply %+v", replies.texts)
	}
	if len(sessions.sessions) != 2 {
		t.Fatalf("expected two persisted session snapshots, got %+v", sessions.sessions)
	}
	if completed, _ := sessions.sessions[1].State["completed"].(bool); !completed {
		t.Fatalf("expected completed workflow state, got %+v", sessions.sessions[1])
	}
}
