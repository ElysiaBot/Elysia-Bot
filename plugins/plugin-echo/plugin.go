package pluginecho

import (
	"errors"
	"fmt"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

type Config struct {
	Prefix string `json:"prefix"`
}

type Plugin struct {
	Manifest     pluginsdk.PluginManifest
	Config       Config
	ReplyService pluginsdk.ReplyService
}

func New(replyService pluginsdk.ReplyService, config Config) Plugin {
	return Plugin{
		Manifest: pluginsdk.PluginManifest{
			ID:         "plugin-echo",
			Name:       "Echo Plugin",
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
			Entry: pluginsdk.PluginEntry{Module: "plugins/plugin-echo", Symbol: "Plugin"},
		},
		Config:       config,
		ReplyService: replyService,
	}
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
