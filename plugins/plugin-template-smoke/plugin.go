package plugintemplatesmoke

import (
	"encoding/json"
	"errors"
	"fmt"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

const (
	TemplatePluginID                  = "plugin-template-smoke"
	TemplatePluginName                = "Plugin Template Smoke"
	TemplatePluginModule              = "plugins/plugin-template-smoke"
	TemplatePluginSymbol              = "Plugin"
	TemplatePluginPublishSourceType   = "git"
	TemplatePluginPublishSourceURI    = "https://github.com/ohmyopencode/bot-platform/tree/main/plugins/plugin-template-smoke"
	TemplatePluginRuntimeVersionRange = ">=0.1.0 <1.0.0"
)

type manifestPublishMetadata struct {
	SourceType          string `json:"sourceType"`
	SourceURI           string `json:"sourceUri"`
	RuntimeVersionRange string `json:"runtimeVersionRange"`
}

type Config struct {
	Prefix string `json:"prefix"`
}

type Plugin struct {
	Manifest     pluginsdk.PluginManifest
	Config       Config
	ReplyService pluginsdk.ReplyService
}

func New(replyService pluginsdk.ReplyService, config Config) Plugin {
	manifest := pluginsdk.PluginManifest{
		ID:         TemplatePluginID,
		Name:       TemplatePluginName,
		Version:    "0.1.0",
		APIVersion: "v0",
		Mode:       pluginsdk.ModeSubprocess,
		Permissions: []string{
			"reply:send",
		},
		ConfigSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prefix": map[string]any{"type": "string"},
			},
		},
		Entry: pluginsdk.PluginEntry{Module: TemplatePluginModule, Symbol: TemplatePluginSymbol},
	}

	return Plugin{
		Manifest:     withPublishMetadata(manifest),
		Config:       config,
		ReplyService: replyService,
	}
}

func withPublishMetadata(manifest pluginsdk.PluginManifest) pluginsdk.PluginManifest {
	rawManifest, err := json.Marshal(struct {
		pluginsdk.PluginManifest
		Publish manifestPublishMetadata `json:"publish"`
	}{
		PluginManifest: manifest,
		Publish: manifestPublishMetadata{
			SourceType:          TemplatePluginPublishSourceType,
			SourceURI:           TemplatePluginPublishSourceURI,
			RuntimeVersionRange: TemplatePluginRuntimeVersionRange,
		},
	})
	if err != nil {
		panic(fmt.Sprintf("marshal template publish metadata: %v", err))
	}
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		panic(fmt.Sprintf("unmarshal template publish metadata: %v", err))
	}
	return manifest
}

func (p Plugin) Definition() pluginsdk.Plugin {
	return pluginsdk.Plugin{
		Manifest: p.Manifest,
		Handlers: pluginsdk.Handlers{Event: p},
	}
}

func (p Plugin) OnEvent(event eventmodel.Event, ctx eventmodel.ExecutionContext) error {
	if event.Type != "message.received" || event.Message == nil {
		return nil
	}
	if ctx.Reply == nil {
		return errors.New("reply handle is required")
	}
	if p.ReplyService == nil {
		return errors.New("reply service is required")
	}

	message := event.Message.Text
	if p.Config.Prefix != "" {
		message = fmt.Sprintf("%s%s", p.Config.Prefix, message)
	}

	return p.ReplyService.ReplyText(*ctx.Reply, message)
}
