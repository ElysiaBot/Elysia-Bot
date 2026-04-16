package runtimecore

import (
	"time"

	pluginsdk "github.com/ohmyopencode/bot-platform/packages/plugin-sdk"
)

type JobStatus = pluginsdk.JobStatus

const (
	JobStatusPending   = pluginsdk.JobStatusPending
	JobStatusRunning   = pluginsdk.JobStatusRunning
	JobStatusRetrying  = pluginsdk.JobStatusRetrying
	JobStatusCancelled = pluginsdk.JobStatusCancelled
	JobStatusFailed    = pluginsdk.JobStatusFailed
	JobStatusDead      = pluginsdk.JobStatusDead
	JobStatusDone      = pluginsdk.JobStatusDone
)

type Job = pluginsdk.Job

func NewJob(id, jobType string, maxRetries int, timeout time.Duration) Job {
	return pluginsdk.NewJob(id, jobType, maxRetries, timeout)
}
