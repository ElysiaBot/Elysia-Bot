module github.com/ohmyopencode/bot-platform/plugins/plugin-workflow-demo

go 1.25.0

require (
	github.com/ohmyopencode/bot-platform/packages/event-model v0.0.0
	github.com/ohmyopencode/bot-platform/packages/plugin-sdk v0.0.0
	github.com/ohmyopencode/bot-platform/packages/runtime-core v0.0.0
)

replace github.com/ohmyopencode/bot-platform/packages/event-model => ../../packages/event-model

replace github.com/ohmyopencode/bot-platform/packages/plugin-sdk => ../../packages/plugin-sdk

replace github.com/ohmyopencode/bot-platform/packages/runtime-core => ../../packages/runtime-core
