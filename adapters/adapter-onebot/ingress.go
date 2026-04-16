package adapteronebot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	runtimecore "github.com/ohmyopencode/bot-platform/packages/runtime-core"
)

type MessageEventPayload struct {
	PostType    string        `json:"post_type"`
	MessageType string        `json:"message_type"`
	Time        int64         `json:"time"`
	SelfID      int64         `json:"self_id"`
	UserID      int64         `json:"user_id"`
	GroupID     int64         `json:"group_id,omitempty"`
	MessageID   int64         `json:"message_id"`
	RawMessage  string        `json:"raw_message"`
	Sender      SenderPayload `json:"sender"`
}

type SenderPayload struct {
	Nickname string `json:"nickname"`
}

type IngressConverter struct {
	logger *runtimecore.Logger
	tracer *runtimecore.TraceRecorder
	now    func() time.Time
}

func NewIngressConverter(logWriter io.Writer) *IngressConverter {
	return &IngressConverter{
		logger: runtimecore.NewLogger(logWriter),
		tracer: runtimecore.NewTraceRecorder(),
		now:    time.Now().UTC,
	}
}

func (c *IngressConverter) SetObservability(tracer *runtimecore.TraceRecorder) {
	if tracer != nil {
		c.tracer = tracer
	}
}

func (c *IngressConverter) NowForTest(now func() time.Time) {
	c.now = now
}

func (c *IngressConverter) ConvertMessageEvent(payload MessageEventPayload) (eventmodel.Event, error) {
	if payload.PostType != "message" {
		return eventmodel.Event{}, fmt.Errorf("unsupported post_type %q", payload.PostType)
	}

	timestamp := time.Unix(payload.Time, 0).UTC()
	if payload.Time == 0 {
		timestamp = c.now()
	}

	event := eventmodel.Event{
		EventID:   newID("evt"),
		TraceID:   newID("trace"),
		Source:    "onebot",
		Type:      "message.received",
		Timestamp: timestamp,
		Actor: &eventmodel.Actor{
			ID:          fmt.Sprintf("user-%d", payload.UserID),
			Type:        "user",
			DisplayName: payload.Sender.Nickname,
		},
		Channel: &eventmodel.Channel{
			ID:    channelID(payload),
			Type:  payload.MessageType,
			Title: channelTitle(payload),
		},
		Message: &eventmodel.Message{
			ID:   fmt.Sprintf("msg-%d", payload.MessageID),
			Text: payload.RawMessage,
		},
		Reply: ReplyHandleFromMessageEvent(payload),
		Metadata: map[string]any{
			"adapter": "onebot/v11",
		},
		IdempotencyKey: fmt.Sprintf("onebot:%s:%d", payload.MessageType, payload.MessageID),
	}

	if err := event.Validate(); err != nil {
		return eventmodel.Event{}, err
	}
	finishSpan := c.tracer.StartSpan(event.TraceID, "adapter.ingress", event.EventID, "", "", event.IdempotencyKey, map[string]any{
		"source": event.Source,
		"type":   event.Type,
	})
	defer finishSpan()

	if err := c.logger.Log("info", "onebot ingress mapped to standard event", runtimecore.LogContext{
		TraceID:       event.TraceID,
		EventID:       event.EventID,
		CorrelationID: event.IdempotencyKey,
	}, map[string]any{
		"source":       event.Source,
		"type":         event.Type,
		"message_type": payload.MessageType,
		"message_id":   payload.MessageID,
		"user_id":      payload.UserID,
	}); err != nil {
		return eventmodel.Event{}, err
	}
	return event, nil
}

func MarshalEventJSON(event eventmodel.Event) ([]byte, error) {
	return json.MarshalIndent(event, "", "  ")
}

func channelID(payload MessageEventPayload) string {
	if payload.MessageType == "group" && payload.GroupID != 0 {
		return fmt.Sprintf("group-%d", payload.GroupID)
	}
	return fmt.Sprintf("private-%d", payload.UserID)
}

func channelTitle(payload MessageEventPayload) string {
	if payload.MessageType == "group" && payload.GroupID != 0 {
		return fmt.Sprintf("group-%d", payload.GroupID)
	}
	return payload.Sender.Nickname
}

func newID(prefix string) string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s-fallback", prefix)
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buffer))
}
