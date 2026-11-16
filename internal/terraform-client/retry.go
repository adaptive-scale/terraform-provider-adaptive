package client

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	DefaultFunctionMaxRetry   = 10
	DefaultFunctionMaxTimeout = time.Duration(time.Minute * 5)
)

var (
	// ErrTimeout           = errgo.New("Operation aborted. Timeout occured")
	// ErrMaxRetriesReached = errgo.New("Operation aborted. Too many errors.")
	ErrTimeout           = errors.New("timeout occured")
	ErrMaxRetriesReached = errors.New("too many errors")
)

// IsTimeout returns true if the cause of the given error is a TimeoutError.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded)
	// return errgo.Cause(err) == ErrTimeout
}

// IsMaxRetriesReached returns true if the cause of the given error is a MaxRetriesReachedError.
func IsMaxRetriesReached(err error) bool {
	return errors.Is(err, ErrMaxRetriesReached)
	// return errgo.Cause(err) == ErrMaxRetriesReached
}

// Option to dictate behaviour of retry validator
type RetryOption func(options *retryOptions)

// Timeout specifies the maximum time that should be used before aborting the retry loop.
// Note that this does not abort the operation in progress.
func Timeout(d time.Duration) RetryOption {
	return func(options *retryOptions) {
		options.Timeout = d
	}
}

// RetryLimit specifies the maximum number of times op will be called by Do().
func RetryLimit(tries int) RetryOption {
	return func(options *retryOptions) {
		options.RetryLimit = tries
	}
}

// RetryChecker defines whether the given error is an error that can be retried.
func RetryChecker(checker func(result any, err error) bool) RetryOption {
	return func(options *retryOptions) {
		options.Checker = checker
	}
}

func Sleep(d time.Duration) RetryOption {
	return func(options *retryOptions) {
		options.Sleep = d
	}
}

// AfterRetryLimit is called after a retry limit is reached and can be used
// e.g. to emit events.
func AfterRetryLimit(afterRetryLimit func(err error)) RetryOption {
	return func(options *retryOptions) {
		options.AfterRetryLimit = afterRetryLimit
	}
}

func RetryResultChecker(resultChecker func(result any) bool) RetryOption {
	return func(options *retryOptions) {
		options.ResultChecker = resultChecker
	}
}

// Polling fails on errors unless retry is explicitly authorized.
func defaultErrorChecker(result any, err error) bool {
	return false
}

func defaultResultChecker(result any) bool {
	return result != nil
}

// TODO: Use generics
type retryOptions struct {
	Timeout         time.Duration
	RetryLimit      int
	Checker         func(result any, err error) bool
	ResultChecker   func(result any) bool
	Sleep           time.Duration
	AfterRetryLimit func(err error)
}

func newRetryOptions(options ...RetryOption) retryOptions {
	state := retryOptions{
		Timeout:         DefaultFunctionMaxTimeout,
		RetryLimit:      DefaultFunctionMaxRetry,
		Checker:         defaultErrorChecker,
		ResultChecker:   defaultResultChecker,
		AfterRetryLimit: func(err error) {},
	}

	for _, option := range options {
		option(&state)
	}

	return state
}

func zeroVal[T any]() T {
	return *new(T)
}

func Do[T any](op func() (T, error), retryOptions ...RetryOption) (T, error) {
	return DoContext(context.Background(), op, retryOptions...)
}

// DoContext is the cancellation-aware polling variant. Operations must use
// ctx themselves for in-flight cancellation. Errors stop polling by default;
// callers can explicitly opt into retries with RetryChecker.
func DoContext[T any](ctx context.Context, op func() (T, error), retryOptions ...RetryOption) (T, error) {
	options := newRetryOptions(retryOptions...)
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return zeroVal[T](), err
		}
		result, err := op()
		if err != nil {
			if options.Checker == nil || !options.Checker(result, err) {
				return zeroVal[T](), err
			}
		} else if options.ResultChecker == nil || !options.ResultChecker(result) {
			return result, nil
		}
		if attempt >= options.RetryLimit {
			options.AfterRetryLimit(err)
			return zeroVal[T](), fmt.Errorf("%w (%d attempts)", ErrMaxRetriesReached, attempt)
		}
		timer := time.NewTimer(options.Sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zeroVal[T](), ctx.Err()
		case <-timer.C:
		}
	}
}
