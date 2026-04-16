package runtimecore

import (
	"strings"
	"testing"
	"time"
)

func TestTraceRecorderTracksCoreSpanChain(t *testing.T) {
	t.Parallel()

	recorder := NewTraceRecorder()
	timestamps := []time.Time{
		time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 1, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 2, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 3, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 4, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 5, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 6, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 7, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 8, 0, time.UTC),
		time.Date(2026, 4, 3, 10, 0, 9, 0, time.UTC),
	}
	index := 0
	recorder.now = func() time.Time {
		value := timestamps[index]
		index++
		return value
	}

	for _, spanName := range []string{"adapter.ingress", "runtime.dispatch", "plugin.invoke", "job.lifecycle", "reply.send"} {
		finish := recorder.StartSpan("trace-1", spanName, "evt-1", "plugin-echo", "run-1", "corr-1", nil)
		finish()
	}

	rendered := recorder.RenderTrace("trace-1")
	for _, expected := range []string{"adapter.ingress", "runtime.dispatch", "plugin.invoke", "job.lifecycle", "reply.send"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered trace to contain %q, got %s", expected, rendered)
		}
	}
}
