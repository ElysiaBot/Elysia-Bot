package adapteronebot

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	runtimecore "github.com/ohmyopencode/bot-platform/packages/runtime-core"
)

func TestConvertMessageEventMapsToStandardEvent(t *testing.T) {
	t.Parallel()

	logs := &bytes.Buffer{}
	converter := NewIngressConverter(logs)
	converter.now = func() time.Time {
		return time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC)
	}

	event, err := converter.ConvertMessageEvent(MessageEventPayload{
		PostType:    "message",
		MessageType: "group",
		Time:        1712034000,
		UserID:      10001,
		GroupID:     42,
		MessageID:   9001,
		RawMessage:  "hello platform",
		Sender:      SenderPayload{Nickname: "alice"},
	})
	if err != nil {
		t.Fatalf("convert message event: %v", err)
	}

	if event.Source != "onebot" || event.Type != "message.received" {
		t.Fatalf("unexpected mapped event: %+v", event)
	}
	if event.Actor == nil || event.Actor.ID != "user-10001" {
		t.Fatalf("unexpected actor mapping: %+v", event.Actor)
	}
	if event.Channel == nil || event.Channel.ID != "group-42" {
		t.Fatalf("unexpected channel mapping: %+v", event.Channel)
	}
	if event.Message == nil || event.Message.Text != "hello platform" {
		t.Fatalf("unexpected message mapping: %+v", event.Message)
	}
	if event.Reply == nil || event.Reply.TargetID != "group-42" {
		t.Fatalf("expected reply handle for mapped event, got %+v", event.Reply)
	}
	if _, exists := event.Metadata["raw_payload"]; exists {
		t.Fatal("raw payload must not leak into standard event metadata")
	}
	if !strings.Contains(logs.String(), "onebot ingress mapped to standard event") {
		t.Fatalf("expected ingress log, got %s", logs.String())
	}
}

func TestMarshalEventJSONPrintsStandardEvent(t *testing.T) {
	t.Parallel()

	converter := NewIngressConverter(&bytes.Buffer{})
	converter.now = func() time.Time {
		return time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC)
	}

	event, err := converter.ConvertMessageEvent(MessageEventPayload{
		PostType:    "message",
		MessageType: "private",
		UserID:      10002,
		MessageID:   9002,
		RawMessage:  "ping",
		Sender:      SenderPayload{Nickname: "bob"},
	})
	if err != nil {
		t.Fatalf("convert message event: %v", err)
	}

	encoded, err := MarshalEventJSON(event)
	if err != nil {
		t.Fatalf("marshal event json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal encoded event: %v", err)
	}
	if decoded["source"] != "onebot" || decoded["type"] != "message.received" {
		t.Fatalf("unexpected printed event json: %s", string(encoded))
	}
}

func TestConvertMessageEventRejectsUnsupportedPayload(t *testing.T) {
	t.Parallel()

	converter := NewIngressConverter(&bytes.Buffer{})
	_, err := converter.ConvertMessageEvent(MessageEventPayload{PostType: "notice"})
	if err == nil {
		t.Fatal("expected unsupported payload to fail")
	}
}

func TestConvertMessageEventRecordsIngressObservability(t *testing.T) {
	t.Parallel()

	logs := &bytes.Buffer{}
	tracer := runtimecore.NewTraceRecorder()
	converter := NewIngressConverter(logs)
	converter.SetObservability(tracer)

	event, err := converter.ConvertMessageEvent(MessageEventPayload{PostType: "message", MessageType: "group", UserID: 10001, GroupID: 42, MessageID: 9001, RawMessage: "hello", Sender: SenderPayload{Nickname: "alice"}})
	if err != nil {
		t.Fatalf("convert message event: %v", err)
	}
	if !strings.Contains(logs.String(), event.IdempotencyKey) {
		t.Fatalf("expected correlation id in ingress log, got %s", logs.String())
	}
	if rendered := tracer.RenderTrace(event.TraceID); !strings.Contains(rendered, "adapter.ingress") {
		t.Fatalf("expected ingress span, got %s", rendered)
	}
}
