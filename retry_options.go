package restate

import (
	"time"

	"github.com/restatedev/sdk-go/internal/options"
)

// Retry options for [Run] (and [RunAsync] / [RunVoid]) and for the invocation retry
// policy ([WithInvocationRetryPolicy]).

type withMaxRetryAttempts struct{ maxAttempts uint }

var (
	_ options.RunOption                   = withMaxRetryAttempts{}
	_ options.InvocationRetryPolicyOption = withMaxRetryAttempts{}
)

func (w withMaxRetryAttempts) BeforeRun(o *options.RunOptions) {
	n := w.maxAttempts
	o.MaxRetryAttempts = &n
}

func (w withMaxRetryAttempts) BeforeRetryPolicy(p *options.InvocationRetryPolicy) {
	n := int(w.maxAttempts)
	p.MaxAttempts = &n
}

// WithMaxRetryAttempts sets the maximum number of attempts before giving up retrying.
// The initial call counts as the first attempt.
func WithMaxRetryAttempts(maxAttempts uint) withMaxRetryAttempts {
	return withMaxRetryAttempts{maxAttempts}
}

type withInitialRetryInterval struct{ d time.Duration }

var (
	_ options.RunOption                   = withInitialRetryInterval{}
	_ options.InvocationRetryPolicyOption = withInitialRetryInterval{}
)

func (w withInitialRetryInterval) BeforeRun(o *options.RunOptions) {
	d := w.d
	o.InitialRetryInterval = &d
}

func (w withInitialRetryInterval) BeforeRetryPolicy(p *options.InvocationRetryPolicy) {
	d := w.d
	p.InitialInterval = &d
}

// WithInitialRetryInterval sets the delay before the first retry attempt. The interval
// then grows by the factor set with [WithRetryIntervalFactor], capped by
// [WithMaxRetryInterval].
func WithInitialRetryInterval(d time.Duration) withInitialRetryInterval {
	return withInitialRetryInterval{d}
}

type withMaxRetryInterval struct{ d time.Duration }

var (
	_ options.RunOption                   = withMaxRetryInterval{}
	_ options.InvocationRetryPolicyOption = withMaxRetryInterval{}
)

func (w withMaxRetryInterval) BeforeRun(o *options.RunOptions) {
	d := w.d
	o.MaxRetryInterval = &d
}

func (w withMaxRetryInterval) BeforeRetryPolicy(p *options.InvocationRetryPolicy) {
	d := w.d
	p.MaxInterval = &d
}

// WithMaxRetryInterval caps the delay between retry attempts.
func WithMaxRetryInterval(d time.Duration) withMaxRetryInterval {
	return withMaxRetryInterval{d}
}

type withRetryIntervalFactor struct{ f float32 }

var (
	_ options.RunOption                   = withRetryIntervalFactor{}
	_ options.InvocationRetryPolicyOption = withRetryIntervalFactor{}
)

func (w withRetryIntervalFactor) BeforeRun(o *options.RunOptions) {
	f := w.f
	o.RetryIntervalFactor = &f
}

func (w withRetryIntervalFactor) BeforeRetryPolicy(p *options.InvocationRetryPolicy) {
	f := float64(w.f)
	p.ExponentiationFactor = &f
}

// WithRetryIntervalFactor sets the multiplier applied to the retry interval after each
// attempt (e.g. 2 doubles the interval each time).
func WithRetryIntervalFactor(f float32) withRetryIntervalFactor {
	return withRetryIntervalFactor{f}
}

type withMaxRetryDuration struct{ d time.Duration }

var _ options.RunOption = withMaxRetryDuration{}

func (w withMaxRetryDuration) BeforeRun(o *options.RunOptions) {
	d := w.d
	o.MaxRetryDuration = &d
}

// WithMaxRetryDuration sets the maximum total time spent retrying before giving up.
func WithMaxRetryDuration(d time.Duration) withMaxRetryDuration {
	return withMaxRetryDuration{d}
}

type withOnMaxAttempts struct{ v options.OnMaxAttempts }

var _ options.InvocationRetryPolicyOption = withOnMaxAttempts{}

func (w withOnMaxAttempts) BeforeRetryPolicy(p *options.InvocationRetryPolicy) {
	v := w.v
	p.OnMaxAttempts = &v
}

// PauseOnMaxAttempts pauses the invocation when the maximum number of attempts is reached.
func PauseOnMaxAttempts() withOnMaxAttempts { return withOnMaxAttempts{options.OnMaxAttemptsPause} }

// KillOnMaxAttempts kills the invocation when the maximum number of attempts is reached.
func KillOnMaxAttempts() withOnMaxAttempts { return withOnMaxAttempts{options.OnMaxAttemptsKill} }
