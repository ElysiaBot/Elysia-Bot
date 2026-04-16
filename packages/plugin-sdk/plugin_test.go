package pluginsdk

import (
	"testing"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
)

type stubEventHandler struct{}

func (stubEventHandler) OnEvent(event eventmodel.Event, ctx eventmodel.ExecutionContext) error {
	return nil
}

func TestPluginManifestValidateAcceptsMinimalContract(t *testing.T) {
	t.Parallel()

	manifest := PluginManifest{
		ID:         "plugin-echo",
		Name:       "Echo Plugin",
		Version:    "0.1.0",
		APIVersion: "v0",
		Mode:       ModeSubprocess,
		Entry: PluginEntry{
			Module: "plugins/echo",
			Symbol: "Plugin",
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected manifest to validate, got %v", err)
	}
}

func TestPluginManifestValidateRejectsInvalidPermissionFormat(t *testing.T) {
	t.Parallel()

	manifest := PluginManifest{
		ID:          "plugin-bad-perm",
		Name:        "Bad Permission Plugin",
		Version:     "0.1.0",
		APIVersion:  "v0",
		Mode:        ModeSubprocess,
		Permissions: []string{"reply-send"},
		Entry:       PluginEntry{Module: "plugins/bad", Symbol: "Plugin"},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected invalid permission format to fail validation")
	}
}

func TestPluginManifestValidateRejectsInvalidManifest(t *testing.T) {
	t.Parallel()

	manifest := PluginManifest{}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected invalid manifest to fail validation")
	}

	manifest = PluginManifest{
		ID:         "plugin-bad",
		Name:       "Bad Plugin",
		Version:    "0.1.0",
		APIVersion: "v0",
		Mode:       "embedded",
		Entry:      PluginEntry{Module: "plugins/bad"},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected unsupported mode to fail validation")
	}
}

func TestRegistryRegistersAndListsPlugin(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	plugin := Plugin{
		Manifest: PluginManifest{
			ID:         "plugin-echo",
			Name:       "Echo Plugin",
			Version:    "0.1.0",
			APIVersion: "v0",
			Mode:       ModeSubprocess,
			Entry: PluginEntry{
				Module: "plugins/echo",
				Symbol: "Plugin",
			},
		},
		Handlers: Handlers{Event: stubEventHandler{}},
	}

	if err := registry.Register(plugin); err != nil {
		t.Fatalf("register plugin: %v", err)
	}

	registered, err := registry.Get("plugin-echo")
	if err != nil {
		t.Fatalf("get plugin: %v", err)
	}
	if registered.Manifest.ID != "plugin-echo" {
		t.Fatalf("unexpected plugin id %q", registered.Manifest.ID)
	}

	manifests := registry.List()
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}

	if err := registry.Register(plugin); err == nil {
		t.Fatal("expected duplicate registration to fail")
	}
}
