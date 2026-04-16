package eventmodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEventExamplesValidate(t *testing.T) {
	t.Parallel()

	raw := readRepoFile(t, filepath.Join("..", "..", "schemas", "event.examples.json"))

	var examples []struct {
		Event   Event            `json:"event"`
		Context ExecutionContext `json:"context"`
	}

	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatalf("unmarshal event examples: %v", err)
	}

	if len(examples) != 3 {
		t.Fatalf("expected 3 examples, got %d", len(examples))
	}

	for index, example := range examples {
		if err := example.Event.Validate(); err != nil {
			t.Fatalf("example %d event invalid: %v", index, err)
		}
		if err := example.Context.Validate(); err != nil {
			t.Fatalf("example %d context invalid: %v", index, err)
		}
	}
}

func TestEventValidateRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	event := Event{}
	if err := event.Validate(); err == nil || !strings.Contains(err.Error(), "event_id") {
		t.Fatalf("expected missing event_id error, got %v", err)
	}

	context := ExecutionContext{}
	if err := context.Validate(); err == nil || !strings.Contains(err.Error(), "trace_id") {
		t.Fatalf("expected missing trace_id error, got %v", err)
	}

	reply := ReplyHandle{}
	if err := reply.Validate(); err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("expected missing capability error, got %v", err)
	}
}

func TestSchemaContainsRequiredContractFields(t *testing.T) {
	t.Parallel()

	raw := string(readRepoFile(t, filepath.Join("..", "..", "schemas", "event.schema.json")))

	for _, required := range []string{"event_id", "trace_id", "source", "type", "timestamp", "idempotency_key"} {
		if !strings.Contains(raw, `"`+required+`"`) {
			t.Fatalf("schema missing required field %q", required)
		}
	}
}

func readRepoFile(t *testing.T, relativePath string) []byte {
	t.Helper()

	raw, err := os.ReadFile(relativePath)
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}

	return raw
}
