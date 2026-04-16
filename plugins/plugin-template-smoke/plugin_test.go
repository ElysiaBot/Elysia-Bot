package plugintemplatesmoke

import (
	"errors"
	"testing"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
)

type recordingReplyService struct {
	handle eventmodel.ReplyHandle
	text   string
	err    error
}

func (r *recordingReplyService) ReplyText(handle eventmodel.ReplyHandle, text string) error {
	r.handle = handle
	r.text = text
	return r.err
}

func (r *recordingReplyService) ReplyImage(handle eventmodel.ReplyHandle, imageURL string) error {
	return nil
}

func (r *recordingReplyService) ReplyFile(handle eventmodel.ReplyHandle, fileURL string) error {
	return nil
}

func TestPluginTemplateSmokeRepliesToMessageEvent(t *testing.T) {
	t.Parallel()

	replies := &recordingReplyService{}
	plugin := New(replies, Config{Prefix: "template: "})

	err := plugin.OnEvent(eventmodel.Event{
		EventID:        "evt-template",
		TraceID:        "trace-template",
		Source:         "onebot",
		Type:           "message.received",
		Timestamp:      time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
		IdempotencyKey: "onebot:msg:template",
		Message:        &eventmodel.Message{Text: "hello"},
	}, eventmodel.ExecutionContext{
		TraceID: "trace-template",
		EventID: "evt-template",
		Reply:   &eventmodel.ReplyHandle{Capability: "onebot.reply", TargetID: "group-42"},
	})
	if err != nil {
		t.Fatalf("on event: %v", err)
	}
	if replies.text != "template: hello" {
		t.Fatalf("unexpected reply text %q", replies.text)
	}
	if replies.handle.TargetID != "group-42" {
		t.Fatalf("unexpected reply handle %+v", replies.handle)
	}
}

func TestPluginTemplateSmokeRequiresReplyHandle(t *testing.T) {
	t.Parallel()

	plugin := New(&recordingReplyService{}, Config{})
	err := plugin.OnEvent(eventmodel.Event{
		EventID:        "evt-template",
		TraceID:        "trace-template",
		Source:         "onebot",
		Type:           "message.received",
		Timestamp:      time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
		IdempotencyKey: "onebot:msg:template",
		Message:        &eventmodel.Message{Text: "hello"},
	}, eventmodel.ExecutionContext{TraceID: "trace-template", EventID: "evt-template"})
	if err == nil {
		t.Fatal("expected missing reply handle to fail")
	}
}

func TestPluginTemplateSmokeReturnsReplyError(t *testing.T) {
	t.Parallel()

	plugin := New(&recordingReplyService{err: errors.New("send failed")}, Config{})
	err := plugin.OnEvent(eventmodel.Event{
		EventID:        "evt-template",
		TraceID:        "trace-template",
		Source:         "onebot",
		Type:           "message.received",
		Timestamp:      time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
		IdempotencyKey: "onebot:msg:template",
		Message:        &eventmodel.Message{Text: "hello"},
	}, eventmodel.ExecutionContext{
		TraceID: "trace-template",
		EventID: "evt-template",
		Reply:   &eventmodel.ReplyHandle{Capability: "onebot.reply", TargetID: "group-42"},
	})
	if err == nil {
		t.Fatal("expected reply service error to bubble up")
	}
}
