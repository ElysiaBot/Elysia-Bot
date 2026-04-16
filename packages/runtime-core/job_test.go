package runtimecore

import (
	"testing"
	"time"
)

func TestJobValidateAndExplainStatus(t *testing.T) {
	t.Parallel()

	job := NewJob("job-1", "ai.call", 3, 30*time.Second)
	if err := job.Validate(); err != nil {
		t.Fatalf("validate job: %v", err)
	}
	if job.ExplainStatus() != "job is waiting to be picked up" {
		t.Fatalf("unexpected status explanation %q", job.ExplainStatus())
	}
}

func TestJobTransitionHappyPath(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 4, 2, 19, 0, 0, 0, time.UTC)
	job := NewJob("job-2", "webhook.retry", 2, time.Minute)

	running, err := job.Transition(JobStatusRunning, at, "")
	if err != nil {
		t.Fatalf("transition to running: %v", err)
	}
	if running.StartedAt == nil || !running.StartedAt.Equal(at) {
		t.Fatalf("expected started_at to be set, got %+v", running.StartedAt)
	}

	done, err := running.Transition(JobStatusDone, at.Add(time.Second), "")
	if err != nil {
		t.Fatalf("transition to done: %v", err)
	}
	if done.FinishedAt == nil || done.Status != JobStatusDone {
		t.Fatalf("expected finished job, got %+v", done)
	}
}

func TestJobTransitionRetryAndDeadLetter(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 4, 2, 19, 5, 0, 0, time.UTC)
	job := NewJob("job-3", "file.process", 1, time.Minute)

	running, err := job.Transition(JobStatusRunning, at, "")
	if err != nil {
		t.Fatalf("transition to running: %v", err)
	}

	retrying, err := running.Transition(JobStatusRetrying, at.Add(time.Second), "timeout")
	if err != nil {
		t.Fatalf("transition to retrying: %v", err)
	}
	if retrying.RetryCount != 1 || retrying.NextRunAt == nil {
		t.Fatalf("expected retry metadata, got %+v", retrying)
	}

	dead, err := retrying.Transition(JobStatusDead, at.Add(2*time.Second), "max retries exceeded")
	if err != nil {
		t.Fatalf("transition to dead: %v", err)
	}
	if !dead.DeadLetter || dead.FinishedAt == nil {
		t.Fatalf("expected dead-letter job, got %+v", dead)
	}
}

func TestJobRejectsInvalidTransition(t *testing.T) {
	t.Parallel()

	job := NewJob("job-4", "ai.call", 1, time.Minute)
	if _, err := job.Transition(JobStatusDone, time.Now().UTC(), ""); err == nil {
		t.Fatal("expected invalid transition from pending to done to fail")
	}
}

func TestJobCanCancelFromPendingAndRetrying(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 4, 2, 19, 10, 0, 0, time.UTC)
	job := NewJob("job-5", "ai.call", 1, time.Minute)

	cancelled, err := job.Transition(JobStatusCancelled, at, "")
	if err != nil {
		t.Fatalf("cancel pending job: %v", err)
	}
	if cancelled.Status != JobStatusCancelled || cancelled.FinishedAt == nil || cancelled.ExplainStatus() != "job was cancelled before execution completed" {
		t.Fatalf("expected cancelled pending job, got %+v", cancelled)
	}

	running, err := NewJob("job-6", "ai.call", 1, time.Minute).Transition(JobStatusRunning, at, "")
	if err != nil {
		t.Fatalf("start running job: %v", err)
	}
	retrying, err := running.Transition(JobStatusRetrying, at.Add(time.Second), "boom")
	if err != nil {
		t.Fatalf("transition to retrying: %v", err)
	}
	cancelledRetry, err := retrying.Transition(JobStatusCancelled, at.Add(2*time.Second), "")
	if err != nil {
		t.Fatalf("cancel retrying job: %v", err)
	}
	if cancelledRetry.Status != JobStatusCancelled || cancelledRetry.NextRunAt != nil {
		t.Fatalf("expected cancelled retrying job, got %+v", cancelledRetry)
	}
}
