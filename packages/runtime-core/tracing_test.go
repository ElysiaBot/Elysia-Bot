package runtimecore

import (
	"strings"
	"testing"
	"time"
)

func TestTraceRecorderLocksActive2CanonicalSpanNames(t *testing.T) {
	t.Parallel()

	recorder := NewTraceRecorder()
	base := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	index := 0
	recorder.now = func() time.Time {
		value := base.Add(time.Duration(index) * time.Second)
		index++
		return value
	}

	canonicalSpanNames := []string{
		"adapter.ingress",
		"runtime.event.dispatch",
		"job.enqueue",
		"job.dispatch",
		"workflow.start_or_resume",
		"plugin.dispatch",
		"reply.dispatch",
	}

	for _, spanName := range canonicalSpanNames {
		finish := recorder.StartSpan("trace-active2", spanName, "evt-active2", "plugin-echo", "run-active2", "corr-active2", nil)
		finish()
	}

	spans := recorder.SpansByTrace("trace-active2")
	if len(spans) != len(canonicalSpanNames) {
		t.Fatalf("expected %d canonical spans, got %d", len(canonicalSpanNames), len(spans))
	}
	for i, expected := range canonicalSpanNames {
		if spans[i].SpanName != expected {
			t.Fatalf("expected span %d to be %q, got %q", i, expected, spans[i].SpanName)
		}
	}

	rendered := recorder.RenderTrace("trace-active2")
	for _, expected := range canonicalSpanNames {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered trace to contain %q, got %s", expected, rendered)
		}
	}
}

func TestTraceRecorderSpanCarriesFiveIDContext(t *testing.T) {
	t.Parallel()

	recorder := NewTraceRecorder()
	finish := recorder.StartSpan("trace-ctx-1", "plugin.dispatch", "evt-ctx-1", "plugin-echo", "run-ctx-1", "corr-ctx-1", map[string]any{"dispatch_kind": "event"})
	finish()

	spans := recorder.SpansByTrace("trace-ctx-1")
	if len(spans) != 1 {
		t.Fatalf("expected one span, got %d", len(spans))
	}

	span := spans[0]
	if span.TraceID != "trace-ctx-1" || span.EventID != "evt-ctx-1" || span.PluginID != "plugin-echo" || span.RunID != "run-ctx-1" || span.CorrelationID != "corr-ctx-1" {
		t.Fatalf("expected five-ID span context to round-trip, got %+v", span)
	}
}

func TestTraceRecorderExporterDisabledPreservesLocalRecorderBehavior(t *testing.T) {
	t.Parallel()

	recorder := NewTraceRecorder()
	finish := recorder.StartSpan("trace-local-only", "runtime.dispatch", "evt-local-only", "", "run-local-only", "corr-local-only", map[string]any{"dispatch_kind": "event"})
	finish()

	if recorder.ExporterEnabled() {
		t.Fatal("expected exporter to be disabled by default")
	}
	spans := recorder.SpansByTrace("trace-local-only")
	if len(spans) != 1 {
		t.Fatalf("expected one local span, got %d", len(spans))
	}
	if spans[0].SpanName != "runtime.dispatch" {
		t.Fatalf("expected local span name to remain unchanged, got %q", spans[0].SpanName)
	}
	if rendered := recorder.RenderTrace("trace-local-only"); !strings.Contains(rendered, "runtime.dispatch") {
		t.Fatalf("expected local rendered trace to keep existing span name, got %s", rendered)
	}
}

func TestTraceRecorderExporterReceivesCanonicalSpanNamesAndIDs(t *testing.T) {
	t.Parallel()

	recorder := NewTraceRecorder()
	exporter := NewInMemoryTraceExporter()
	recorder.SetExporter(exporter)

	finishDispatch := recorder.StartSpan("trace-export", "runtime.dispatch", "evt-export", "", "run-export", "corr-export", map[string]any{"dispatch_kind": "event"})
	finishDispatch()
	finishPlugin := recorder.StartSpan("trace-export", "plugin.invoke", "evt-export", "plugin-echo", "run-export", "corr-export", map[string]any{"dispatch_kind": "event", "parent_span_name": "runtime.dispatch"})
	finishPlugin()

	spans := exporter.SpansByTrace("trace-export")
	if len(spans) != 2 {
		t.Fatalf("expected two exported spans, got %d", len(spans))
	}
	if spans[0].SpanName != "runtime.event.dispatch" {
		t.Fatalf("expected canonical runtime span name, got %+v", spans[0])
	}
	if spans[1].SpanName != "plugin.dispatch" {
		t.Fatalf("expected canonical plugin span name, got %+v", spans[1])
	}
	for _, span := range spans {
		if span.TraceID != "trace-export" || span.EventID != "evt-export" || span.RunID != "run-export" || span.CorrelationID != "corr-export" {
			t.Fatalf("expected exported span to retain canonical IDs, got %+v", span)
		}
		if span.SpanID == "" {
			t.Fatalf("expected exported span id, got %+v", span)
		}
	}
	if spans[1].ParentSpanID == "" {
		t.Fatalf("expected plugin span to carry parent span id, got %+v", spans[1])
	}
	if spans[1].PluginID != "plugin-echo" {
		t.Fatalf("expected exported plugin id to round-trip, got %+v", spans[1])
	}
}
