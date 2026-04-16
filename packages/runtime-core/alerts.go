package runtimecore

import (
	"errors"
	"strings"
	"time"
)

const (
	alertObjectTypeJob            = "job"
	alertFailureTypeJobDeadLetter = "job.dead_letter"
)

type AlertRecord struct {
	ID               string    `json:"id"`
	ObjectType       string    `json:"objectType"`
	ObjectID         string    `json:"objectId"`
	FailureType      string    `json:"failureType"`
	FirstOccurredAt  time.Time `json:"firstOccurredAt"`
	LatestOccurredAt time.Time `json:"latestOccurredAt"`
	LatestReason     string    `json:"latestReason"`
	TraceID          string    `json:"traceId,omitempty"`
	EventID          string    `json:"eventId,omitempty"`
	RunID            string    `json:"runId,omitempty"`
	Correlation      string    `json:"correlation,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

func normalizeAlertRecord(alert AlertRecord) (AlertRecord, error) {
	alert.ID = strings.TrimSpace(alert.ID)
	alert.ObjectType = strings.TrimSpace(alert.ObjectType)
	alert.ObjectID = strings.TrimSpace(alert.ObjectID)
	alert.FailureType = strings.TrimSpace(alert.FailureType)
	if alert.ID == "" {
		return AlertRecord{}, errors.New("alert id is required")
	}
	if alert.ObjectType == "" {
		return AlertRecord{}, errors.New("alert object type is required")
	}
	if alert.ObjectID == "" {
		return AlertRecord{}, errors.New("alert object id is required")
	}
	if alert.FailureType == "" {
		return AlertRecord{}, errors.New("alert failure type is required")
	}
	if alert.FirstOccurredAt.IsZero() {
		return AlertRecord{}, errors.New("alert first occurred time is required")
	}
	if alert.LatestOccurredAt.IsZero() {
		alert.LatestOccurredAt = alert.FirstOccurredAt
	}
	alert.FirstOccurredAt = alert.FirstOccurredAt.UTC()
	alert.LatestOccurredAt = alert.LatestOccurredAt.UTC()
	if alert.LatestOccurredAt.Before(alert.FirstOccurredAt) {
		return AlertRecord{}, errors.New("alert latest occurred time cannot be before first occurred time")
	}
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = alert.LatestOccurredAt
	}
	alert.CreatedAt = alert.CreatedAt.UTC()
	return alert, nil
}

func jobDeadLetterAlert(job Job) AlertRecord {
	occurredAt := job.CreatedAt.UTC()
	if job.FinishedAt != nil && !job.FinishedAt.IsZero() {
		occurredAt = job.FinishedAt.UTC()
	}
	return AlertRecord{
		ID:               deadLetterAlertID(job.ID),
		ObjectType:       alertObjectTypeJob,
		ObjectID:         job.ID,
		FailureType:      alertFailureTypeJobDeadLetter,
		FirstOccurredAt:  occurredAt,
		LatestOccurredAt: occurredAt,
		LatestReason:     job.LastError,
		TraceID:          job.TraceID,
		EventID:          job.EventID,
		RunID:            job.RunID,
		Correlation:      job.Correlation,
		CreatedAt:        occurredAt,
	}
}

func deadLetterAlertID(jobID string) string {
	return alertFailureTypeJobDeadLetter + ":" + strings.TrimSpace(jobID)
}
