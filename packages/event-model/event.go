package eventmodel

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Event defines the platform-agnostic contract every adapter must produce.
type Event struct {
	EventID        string             `json:"event_id"`
	TraceID        string             `json:"trace_id"`
	Source         string             `json:"source"`
	Type           string             `json:"type"`
	Timestamp      time.Time          `json:"timestamp"`
	Actor          *Actor             `json:"actor,omitempty"`
	Channel        *Channel           `json:"channel,omitempty"`
	Message        *Message           `json:"message,omitempty"`
	Reply          *ReplyHandle       `json:"reply,omitempty"`
	Command        *CommandInvocation `json:"command,omitempty"`
	System         *SystemEvent       `json:"system,omitempty"`
	Metadata       map[string]any     `json:"metadata,omitempty"`
	IdempotencyKey string             `json:"idempotency_key"`
}

type Actor struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	DisplayName string         `json:"display_name,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Channel struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Title    string         `json:"title,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Message struct {
	ID          string         `json:"id,omitempty"`
	Text        string         `json:"text,omitempty"`
	Attachments []Attachment   `json:"attachments,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Attachment struct {
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type"`
	URL      string         `json:"url,omitempty"`
	Name     string         `json:"name,omitempty"`
	MimeType string         `json:"mime_type,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CommandInvocation struct {
	Name      string         `json:"name"`
	Arguments []string       `json:"arguments,omitempty"`
	Raw       string         `json:"raw,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type SystemEvent struct {
	Name     string         `json:"name"`
	Status   string         `json:"status,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ReplyHandle struct {
	Capability string         `json:"capability"`
	TargetID   string         `json:"target_id"`
	MessageID  string         `json:"message_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ExecutionContext struct {
	TraceID       string         `json:"trace_id"`
	EventID       string         `json:"event_id"`
	PluginID      string         `json:"plugin_id,omitempty"`
	RunID         string         `json:"run_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Reply         *ReplyHandle   `json:"reply,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.EventID) == "" {
		return errors.New("event_id is required")
	}
	if strings.TrimSpace(e.TraceID) == "" {
		return errors.New("trace_id is required")
	}
	if strings.TrimSpace(e.Source) == "" {
		return errors.New("source is required")
	}
	if strings.TrimSpace(e.Type) == "" {
		return errors.New("type is required")
	}
	if e.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if strings.TrimSpace(e.IdempotencyKey) == "" {
		return errors.New("idempotency_key is required")
	}
	for index, attachment := range e.Message.AttachmentsOrNil() {
		if err := attachment.Validate(); err != nil {
			return fmt.Errorf("message.attachments[%d]: %w", index, err)
		}
	}
	if e.Command != nil && strings.TrimSpace(e.Command.Name) == "" {
		return errors.New("command.name is required when command is present")
	}
	if e.System != nil && strings.TrimSpace(e.System.Name) == "" {
		return errors.New("system.name is required when system is present")
	}
	return nil
}

func (c ExecutionContext) Validate() error {
	if strings.TrimSpace(c.TraceID) == "" {
		return errors.New("trace_id is required")
	}
	if strings.TrimSpace(c.EventID) == "" {
		return errors.New("event_id is required")
	}
	if c.Reply != nil {
		if err := c.Reply.Validate(); err != nil {
			return fmt.Errorf("reply: %w", err)
		}
	}
	return nil
}

func (r ReplyHandle) Validate() error {
	if strings.TrimSpace(r.Capability) == "" {
		return errors.New("capability is required")
	}
	if strings.TrimSpace(r.TargetID) == "" {
		return errors.New("target_id is required")
	}
	return nil
}

func (a Attachment) Validate() error {
	if strings.TrimSpace(a.Type) == "" {
		return errors.New("type is required")
	}
	return nil
}

func (m *Message) AttachmentsOrNil() []Attachment {
	if m == nil {
		return nil
	}
	return m.Attachments
}
