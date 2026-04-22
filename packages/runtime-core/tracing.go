package runtimecore

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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

type ExportedSpan struct {
	TraceID       string
	SpanID        string
	ParentSpanID  string
	SpanName      string
	StartedAt     time.Time
	FinishedAt    time.Time
	EventID       string
	PluginID      string
	RunID         string
	CorrelationID string
	Metadata      map[string]any
}

type TraceExporter interface {
	ExportSpan(ExportedSpan) error
}

type InMemoryTraceExporter struct {
	mu    sync.RWMutex
	spans []ExportedSpan
}

func NewInMemoryTraceExporter() *InMemoryTraceExporter {
	return &InMemoryTraceExporter{}
}

func (e *InMemoryTraceExporter) ExportSpan(span ExportedSpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, ExportedSpan{
		TraceID:       span.TraceID,
		SpanID:        span.SpanID,
		ParentSpanID:  span.ParentSpanID,
		SpanName:      span.SpanName,
		StartedAt:     span.StartedAt,
		FinishedAt:    span.FinishedAt,
		EventID:       span.EventID,
		PluginID:      span.PluginID,
		RunID:         span.RunID,
		CorrelationID: span.CorrelationID,
		Metadata:      cloneAnyMap(span.Metadata),
	})
	return nil
}

func (e *InMemoryTraceExporter) SpansByTrace(traceID string) []ExportedSpan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]ExportedSpan, 0)
	for _, span := range e.spans {
		if span.TraceID == traceID {
			result = append(result, span)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.Before(result[j].StartedAt)
	})
	return result
}

type TraceRecorder struct {
	mu       sync.RWMutex
	spans    []Span
	now      func() time.Time
	exporter TraceExporter
	spanSeq  uint64
}

func NewTraceRecorder() *TraceRecorder {
	return &TraceRecorder{now: time.Now().UTC}
}

func (r *TraceRecorder) SetExporter(exporter TraceExporter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exporter = exporter
}

func (r *TraceRecorder) ExporterEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.exporter != nil
}

func (r *TraceRecorder) StartSpan(traceID, spanName, eventID, pluginID, runID, correlationID string, metadata map[string]any) func() {
	startedAt := r.now()
	metadataCopy := cloneAnyMap(metadata)
	return func() {
		finishedAt := r.now()
		span := Span{
			TraceID:       traceID,
			SpanName:      spanName,
			StartedAt:     startedAt,
			FinishedAt:    finishedAt,
			EventID:       eventID,
			PluginID:      pluginID,
			RunID:         runID,
			CorrelationID: correlationID,
			Metadata:      cloneAnyMap(metadataCopy),
		}

		var exporter TraceExporter
		r.mu.Lock()
		r.spans = append(r.spans, span)
		exporter = r.exporter
		r.mu.Unlock()

		if exporter != nil {
			_ = exporter.ExportSpan(r.exportedSpan(span))
		}
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

func (r *TraceRecorder) exportedSpan(span Span) ExportedSpan {
	parentSpanID := ""
	if parentSpanName, _ := span.Metadata["parent_span_name"].(string); strings.TrimSpace(parentSpanName) != "" {
		parentSpanID = exportSpanIDFor(traceParentSeed(span, parentSpanName))
	}
	return ExportedSpan{
		TraceID:       span.TraceID,
		SpanID:        r.nextExportSpanID(span),
		ParentSpanID:  parentSpanID,
		SpanName:      canonicalExportSpanName(span.SpanName, span.Metadata),
		StartedAt:     span.StartedAt,
		FinishedAt:    span.FinishedAt,
		EventID:       span.EventID,
		PluginID:      span.PluginID,
		RunID:         span.RunID,
		CorrelationID: span.CorrelationID,
		Metadata:      cloneAnyMap(span.Metadata),
	}
}

func (r *TraceRecorder) nextExportSpanID(span Span) string {
	sequence := atomic.AddUint64(&r.spanSeq, 1)
	return exportSpanIDFor(fmt.Sprintf("%s|%s|%s|%d|%d", span.TraceID, span.SpanName, span.EventID, span.StartedAt.UnixNano(), sequence))
}

func exportSpanIDFor(seed string) string {
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:8])
}

func traceParentSeed(span Span, parentSpanName string) string {
	return fmt.Sprintf("%s|%s|%s|%d|parent", span.TraceID, parentSpanName, span.EventID, span.StartedAt.UnixNano())
}

func canonicalExportSpanName(spanName string, metadata map[string]any) string {
	switch strings.TrimSpace(spanName) {
	case "runtime.dispatch":
		dispatchKind, _ := metadata["dispatch_kind"].(string)
		dispatchKind = strings.TrimSpace(dispatchKind)
		if dispatchKind == "" {
			return spanName
		}
		return "runtime." + dispatchKind + ".dispatch"
	case "plugin.invoke":
		return "plugin.dispatch"
	case "reply.send":
		return "reply.dispatch"
	case "job.lifecycle":
		jobStatus, _ := metadata["job_status"].(string)
		switch strings.TrimSpace(jobStatus) {
		case string(JobStatusPending):
			return "job.enqueue"
		case string(JobStatusRunning):
			return "job.dispatch"
		}
	}
	return spanName
}
