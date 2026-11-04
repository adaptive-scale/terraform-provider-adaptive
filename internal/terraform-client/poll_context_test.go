package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPollCancellationInterruptsSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := time.Now()
	_, err := DoContext(ctx, func() (int, error) { cancel(); return 0, nil }, Sleep(time.Hour), RetryResultChecker(func(any) bool { return true }))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled: %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancellation did not interrupt sleep")
	}
}
func TestPollTimeoutClassification(t *testing.T) {
	_, err := DoContext(context.Background(), func() (int, error) { return 0, nil }, Timeout(time.Millisecond), Sleep(time.Hour), RetryResultChecker(func(any) bool { return true }))
	if !errors.Is(err, context.DeadlineExceeded) || !IsTimeout(err) {
		t.Fatalf("timeout classification lost: %v", err)
	}
}

func TestPollDoesNotRetryPermanentError(t *testing.T) {
	calls := 0
	_, err := DoContext(context.Background(), func() (int, error) { calls++; return 0, &HTTPError{StatusCode: 403} }, RetryResultChecker(func(any) bool { return true }))
	if err == nil || calls != 1 {
		t.Fatalf("permanent error retried: %d %v", calls, err)
	}
}
