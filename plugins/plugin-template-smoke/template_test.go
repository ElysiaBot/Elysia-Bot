package plugintemplatesmoke

import (
	"testing"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

func TestTemplateManifestConstantsStayInSync(t *testing.T) {
	t.Parallel()

	plugin := New(&recordingReplyService{}, Config{})
	manifest := plugin.Definition().Manifest

	if manifest.ID != TemplatePluginID {
		t.Fatalf("manifest id = %q, want %q", manifest.ID, TemplatePluginID)
	}
	if manifest.Name != TemplatePluginName {
		t.Fatalf("manifest name = %q, want %q", manifest.Name, TemplatePluginName)
	}
	if manifest.Entry.Module != TemplatePluginModule {
		t.Fatalf("manifest entry module = %q, want %q", manifest.Entry.Module, TemplatePluginModule)
	}
	if manifest.Entry.Symbol != TemplatePluginSymbol {
		t.Fatalf("manifest entry symbol = %q, want %q", manifest.Entry.Symbol, TemplatePluginSymbol)
	}
	if manifest.Mode != pluginsdk.ModeSubprocess {
		t.Fatalf("manifest mode = %q, want %q", manifest.Mode, pluginsdk.ModeSubprocess)
	}
	if len(manifest.Permissions) != 1 || manifest.Permissions[0] != "reply:send" {
		t.Fatalf("unexpected manifest permissions %+v", manifest.Permissions)
	}
	if manifest.ConfigSchema["type"] != "object" {
		t.Fatalf("unexpected config schema %+v", manifest.ConfigSchema)
	}
}
