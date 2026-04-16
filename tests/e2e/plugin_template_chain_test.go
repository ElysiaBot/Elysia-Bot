package e2e

import (
	"context"
	"testing"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
	runtimecore "github.com/ohmyopencode/bot-platform/packages/runtime-core"
	plugintemplatesmoke "github.com/ohmyopencode/bot-platform/plugins/plugin-template-smoke"
)

type templateReplyRecorder struct {
	handle eventmodel.ReplyHandle
	text   string
}

func (r *templateReplyRecorder) ReplyText(handle eventmodel.ReplyHandle, text string) error {
	r.handle = handle
	r.text = text
	return nil
}

func (r *templateReplyRecorder) ReplyImage(handle eventmodel.ReplyHandle, imageURL string) error {
	return nil
}

func (r *templateReplyRecorder) ReplyFile(handle eventmodel.ReplyHandle, fileURL string) error {
	return nil
}

func TestPluginTemplateSmokeCreateRegisterRunFlow(t *testing.T) {
	t.Parallel()

	replies := &templateReplyRecorder{}
	runtime := runtimecore.NewInMemoryRuntime(runtimecore.NoopSupervisor{}, runtimecore.DirectPluginHost{})
	plugin := plugintemplatesmoke.New(replies, plugintemplatesmoke.Config{Prefix: "template: "})
	definition := plugin.Definition()

	if err := runtime.RegisterPlugin(definition); err != nil {
		t.Fatalf("register plugin template smoke: %v", err)
	}

	err := runtime.DispatchEvent(context.Background(), eventmodel.Event{
		EventID:        "evt-template-smoke",
		TraceID:        "trace-template-smoke",
		Source:         "onebot",
		Type:           "message.received",
		Timestamp:      time.Date(2026, 4, 10, 11, 0, 0, 0, time.UTC),
		IdempotencyKey: "onebot:msg:template-smoke",
		Reply:          &eventmodel.ReplyHandle{Capability: "onebot.reply", TargetID: "group-42"},
		Message:        &eventmodel.Message{Text: "hello runtime"},
	})
	if err != nil {
		t.Fatalf("dispatch event: %v", err)
	}
	if replies.text != "template: hello runtime" {
		t.Fatalf("unexpected reply text %q", replies.text)
	}
	if replies.handle.TargetID != "group-42" {
		t.Fatalf("unexpected reply handle %+v", replies.handle)
	}
	if definition.Manifest.ID != "plugin-template-smoke" || definition.Manifest.Mode != pluginsdk.ModeSubprocess {
		t.Fatalf("unexpected template manifest %+v", definition.Manifest)
	}
}
