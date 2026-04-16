package runtimecore

import (
	"encoding/json"
	"io"
	"time"
)

type LogContext struct {
	TraceID       string `json:"trace_id,omitempty"`
	EventID       string `json:"event_id,omitempty"`
	PluginID      string `json:"plugin_id,omitempty"`
	RunID         string `json:"run_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

type LogEntry struct {
	Timestamp     string         `json:"timestamp"`
	Level         string         `json:"level"`
	Message       string         `json:"message"`
	TraceID       string         `json:"trace_id,omitempty"`
	EventID       string         `json:"event_id,omitempty"`
	PluginID      string         `json:"plugin_id,omitempty"`
	RunID         string         `json:"run_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Fields        map[string]any `json:"fields,omitempty"`
}

type Logger struct {
	writer io.Writer
	now    func() time.Time
}

func NewLogger(writer io.Writer) *Logger {
	return &Logger{writer: writer, now: time.Now().UTC}
}

func (l *Logger) Log(level, message string, ctx LogContext, fields map[string]any) error {
	entry := LogEntry{
		Timestamp:     l.now().Format(time.RFC3339),
		Level:         level,
		Message:       message,
		TraceID:       ctx.TraceID,
		EventID:       ctx.EventID,
		PluginID:      ctx.PluginID,
		RunID:         ctx.RunID,
		CorrelationID: ctx.CorrelationID,
		Fields:        fields,
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = l.writer.Write(append(encoded, '\n'))
	return err
}
