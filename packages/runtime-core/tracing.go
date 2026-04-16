package runtimecore

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Span struct {
	TraceID       string
	SpanName      string
	StartedAt     time.Time
	FinishedAt    time.Time
	EventID       string
	PluginID      string
	RunID         string
	CorrelationID string
	Metadata      map[string]any
}

type TraceRecorder struct {
	mu    sync.RWMutex
	spans []Span
	now   func() time.Time
}

func NewTraceRecorder() *TraceRecorder {
	return &TraceRecorder{now: time.Now().UTC}
}

func (r *TraceRecorder) StartSpan(traceID, spanName, eventID, pluginID, runID, correlationID string, metadata map[string]any) func() {
	startedAt := r.now()
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.spans = append(r.spans, Span{
			TraceID:       traceID,
			SpanName:      spanName,
			StartedAt:     startedAt,
			FinishedAt:    r.now(),
			EventID:       eventID,
			PluginID:      pluginID,
			RunID:         runID,
			CorrelationID: correlationID,
			Metadata:      metadata,
		})
	}
}

func (r *TraceRecorder) SpansByTrace(traceID string) []Span {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Span, 0)
	for _, span := range r.spans {
		if span.TraceID == traceID {
			result = append(result, span)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.Before(result[j].StartedAt)
	})
	return result
}

func (r *TraceRecorder) RenderTrace(traceID string) string {
	spans := r.SpansByTrace(traceID)
	lines := make([]string, 0, len(spans))
	for _, span := range spans {
		lines = append(lines, fmt.Sprintf("%s event_id=%s plugin_id=%s", span.SpanName, span.EventID, span.PluginID))
	}
	return strings.Join(lines, "\n")
}
