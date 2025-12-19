package client

import (
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
	return errors.Is(err, ErrTimeout)
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

// A default checker that returns true always, since it's any error is an error
func defaultErrorChecker(result any, err error) bool {
	return true
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
	options := newRetryOptions(retryOptions...)

	var timeout <-chan time.Time
	if options.Timeout > 0 {
		timeout = time.After(options.Timeout)
	}

	tryCounter := 0
	for {
		// Check if we reached the timeout
		select {
		case <-timeout:
			return zeroVal[T](), ErrTimeout
			// return errgo.Mask(TimeoutError, errgo.Any)
		default:
		}

		// Execute the op
		tryCounter++
		result, lastError := op()

		// check if bad result
		isBadResult := options.ResultChecker(result)
		if isBadResult || lastError != nil {
			// Is this an error worthy of being rerun
			if isBadResult || (options.Checker != nil && options.Checker(result, lastError)) {
				// Check max retries
				if tryCounter >= options.RetryLimit {
					options.AfterRetryLimit(lastError)
					// return zeroVal[T](), fmt.Errorf("%w. last error: %v", ErrMaxRetriesReached, lastError)
					return zeroVal[T](), fmt.Errorf("%w, (%d/%d). last error: %v", ErrMaxRetriesReached, tryCounter, options.RetryLimit, lastError)
				}

				if options.Sleep > 0 {
					time.Sleep(options.Sleep)
				}
				continue
			}
			return zeroVal[T](), lastError
			// return zeroVal[T](), errgo.Mask(lastError, errgo.Any)
		}
		return result, nil
	}
}
