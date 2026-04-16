package runtimecore

import pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"

type WorkflowStepKind = pluginsdk.WorkflowStepKind

const (
	WorkflowStepKindStep       = pluginsdk.WorkflowStepKindStep
	WorkflowStepKindWaitEvent  = pluginsdk.WorkflowStepKindWaitEvent
	WorkflowStepKindSleep      = pluginsdk.WorkflowStepKindSleep
	WorkflowStepKindCallJob    = pluginsdk.WorkflowStepKindCallJob
	WorkflowStepKindPersist    = pluginsdk.WorkflowStepKindPersist
	WorkflowStepKindCompensate = pluginsdk.WorkflowStepKindCompensate
)

type WorkflowStep = pluginsdk.WorkflowStep
type Workflow = pluginsdk.Workflow

func NewWorkflow(id string, steps ...WorkflowStep) Workflow {
	return pluginsdk.NewWorkflow(id, steps...)
}
