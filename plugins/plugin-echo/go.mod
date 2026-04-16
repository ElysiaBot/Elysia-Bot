module github.com/ohmyopencode/bot-platform/plugins/plugin-echo

go 1.24.0

require (
	github.com/ohmyopencode/bot-platform/packages/event-model v0.0.0
	github.com/ohmyopencode/bot-platform/packages/plugin-sdk v0.0.0
)

replace github.com/ohmyopencode/bot-platform/packages/event-model => ../../packages/event-model

replace github.com/ohmyopencode/bot-platform/packages/plugin-sdk => ../../packages/plugin-sdk
