package pluginworkflowdemo

import (
	"context"
	"errors"
	"time"

	eventmodel "github.com/ohmyopencode/bot-platform/packages/event-model"
	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

type SessionStore interface {
	SaveSession(ctx context.Context, session pluginsdk.SessionState) error
}

type Plugin struct {
	Manifest     pluginsdk.PluginManifest
	ReplyService pluginsdk.ReplyService
	Sessions     SessionStore
	Workflows    map[string]pluginsdk.Workflow
	Now          func() time.Time
}

func New(replyService pluginsdk.ReplyService, sessions SessionStore) *Plugin {
	return &Plugin{
		Manifest: pluginsdk.PluginManifest{
			ID:         "plugin-workflow-demo",
			Name:       "Workflow Demo Plugin",
			Version:    "0.1.0",
			APIVersion: "v0",
			Mode:       pluginsdk.ModeSubprocess,
			Permissions: []string{
				"reply:send",
				"session:write",
			},
			Entry: pluginsdk.PluginEntry{Module: "plugins/plugin-workflow-demo", Symbol: "Plugin"},
		},
		ReplyService: replyService,
		Sessions:     sessions,
		Workflows:    map[string]pluginsdk.Workflow{},
		Now:          time.Now().UTC,
	}
}

func (p *Plugin) Definition() pluginsdk.Plugin {
	return pluginsdk.Plugin{Manifest: p.Manifest, Handlers: pluginsdk.Handlers{Event: p}}
}

func (p *Plugin) OnEvent(event eventmodel.Event, ctx eventmodel.ExecutionContext) error {
	if event.Type != "message.received" || event.Message == nil || ctx.Reply == nil {
		return nil
	}
	if p.ReplyService == nil || p.Sessions == nil {
		return errors.New("reply service and session store are required")
	}

	sessionID := workflowSessionID(event)
	workflow, exists := p.Workflows[sessionID]
	if !exists {
		workflow = pluginsdk.NewWorkflow(
			sessionID,
			pluginsdk.WorkflowStep{Kind: pluginsdk.WorkflowStepKindPersist, Name: "greeting", Value: event.Message.Text},
			pluginsdk.WorkflowStep{Kind: pluginsdk.WorkflowStepKindWaitEvent, Name: "wait-confirm", Value: "message.received"},
			pluginsdk.WorkflowStep{Kind: pluginsdk.WorkflowStepKindCompensate, Name: "complete"},
		)
		var err error
		workflow, err = workflow.Advance(p.Now())
		if err != nil {
			return err
		}
		workflow, err = workflow.Advance(p.Now())
		if err != nil {
			return err
		}
		p.Workflows[sessionID] = workflow
		if err := p.persistWorkflow(sessionID, workflow); err != nil {
			return err
		}
		return p.ReplyService.ReplyText(*ctx.Reply, "workflow started, please send another message to continue")
	}

	var err error
	workflow, err = workflow.ResumeWithEvent(event.Type)
	if err != nil {
		return err
	}
	workflow, err = workflow.Advance(p.Now())
	if err != nil {
		return err
	}
	p.Workflows[sessionID] = workflow
	if err := p.persistWorkflow(sessionID, workflow); err != nil {
		return err
	}
	return p.ReplyService.ReplyText(*ctx.Reply, "workflow resumed and completed")
}

func (p *Plugin) persistWorkflow(sessionID string, workflow pluginsdk.Workflow) error {
	return p.Sessions.SaveSession(context.Background(), pluginsdk.SessionState{
		SessionID: sessionID,
		PluginID:  p.Manifest.ID,
		State: map[string]any{
			"current_index": workflow.CurrentIndex,
			"waiting_for":   workflow.WaitingFor,
			"completed":     workflow.Completed,
			"compensated":   workflow.Compensated,
		},
	})
}

func workflowSessionID(event eventmodel.Event) string {
	if event.Actor != nil && event.Actor.ID != "" {
		return "workflow-" + event.Actor.ID
	}
	return "workflow-anon"
}
