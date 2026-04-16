package plugintemplatesmoke

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

func TestTemplateManifestConstantsStayInSync(t *testing.T) {
	t.Parallel()

	plugin := New(&recordingReplyService{}, Config{})
	manifest := plugin.Definition().Manifest
	staticManifest := readStaticManifest(t)

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
	if manifest.Mode != "subprocess" {
		t.Fatalf("manifest mode = %q, want %q", manifest.Mode, "subprocess")
	}
	if len(manifest.Permissions) != 1 || manifest.Permissions[0] != "reply:send" {
		t.Fatalf("unexpected manifest permissions %+v", manifest.Permissions)
	}
	if manifest.ConfigSchema["type"] != "object" {
		t.Fatalf("unexpected config schema %+v", manifest.ConfigSchema)
	}

	manifestPayload := readManifestPayload(t, plugin.Definition().Manifest)
	staticManifestPayload := readManifestPayload(t, staticManifest)
	expectedPublish := map[string]any{
		"sourceType":          TemplatePluginPublishSourceType,
		"sourceUri":           TemplatePluginPublishSourceURI,
		"runtimeVersionRange": TemplatePluginRuntimeVersionRange,
	}
	if !reflect.DeepEqual(manifestPayload["publish"], expectedPublish) {
		t.Fatalf("manifest publish = %+v, want %+v", manifestPayload["publish"], expectedPublish)
	}

	if staticManifest.ID != manifest.ID {
		t.Fatalf("manifest.json id = %q, want %q", staticManifest.ID, manifest.ID)
	}
	if staticManifest.Name != manifest.Name {
		t.Fatalf("manifest.json name = %q, want %q", staticManifest.Name, manifest.Name)
	}
	if staticManifest.Version != manifest.Version {
		t.Fatalf("manifest.json version = %q, want %q", staticManifest.Version, manifest.Version)
	}
	if staticManifest.APIVersion != manifest.APIVersion {
		t.Fatalf("manifest.json apiVersion = %q, want %q", staticManifest.APIVersion, manifest.APIVersion)
	}
	if staticManifest.Mode != manifest.Mode {
		t.Fatalf("manifest.json mode = %q, want %q", staticManifest.Mode, manifest.Mode)
	}
	if !reflect.DeepEqual(staticManifest.Permissions, manifest.Permissions) {
		t.Fatalf("manifest.json permissions = %+v, want %+v", staticManifest.Permissions, manifest.Permissions)
	}
	if !reflect.DeepEqual(staticManifest.ConfigSchema, manifest.ConfigSchema) {
		t.Fatalf("manifest.json config schema = %+v, want %+v", staticManifest.ConfigSchema, manifest.ConfigSchema)
	}
	if !reflect.DeepEqual(staticManifestPayload["publish"], manifestPayload["publish"]) {
		t.Fatalf("manifest.json publish = %+v, want %+v", staticManifestPayload["publish"], manifestPayload["publish"])
	}
	if staticManifest.Entry.Module != manifest.Entry.Module {
		t.Fatalf("manifest.json entry module = %q, want %q", staticManifest.Entry.Module, manifest.Entry.Module)
	}
	if staticManifest.Entry.Symbol != manifest.Entry.Symbol {
		t.Fatalf("manifest.json entry symbol = %q, want %q", staticManifest.Entry.Symbol, manifest.Entry.Symbol)
	}
}

func readStaticManifest(t *testing.T) pluginsdk.PluginManifest {
	t.Helper()

	rawManifest, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}

	var manifest pluginsdk.PluginManifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatalf("unmarshal manifest.json: %v", err)
	}

	return manifest
}

func readManifestPayload(t *testing.T, manifest any) map[string]any {
	t.Helper()

	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest payload: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(rawManifest, &payload); err != nil {
		t.Fatalf("unmarshal manifest payload: %v", err)
	}

	return payload
}
