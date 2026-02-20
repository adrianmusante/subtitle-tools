package translate

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

// DefaultRetryMaxAttempts is the default number of attempts used when no retry policy
// is explicitly set on the client.
const DefaultRetryMaxAttempts = 5

type RetryOptions struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      float64 // 0.0-1.0
}

func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts: DefaultRetryMaxAttempts,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		Jitter:      0.2,
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isRetryableHTTPStatus(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 500 && code <= 599)
}

func isRejectedHTTPStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusUnauthorized, http.StatusForbidden:
		return true
	default:
		return false
	}
}

func isRetryableNetErr(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && (ne.Timeout() || ne.Temporary())
}

var jitterMu sync.Mutex
var jitterRng = rand.New(rand.NewSource(time.Now().UnixNano()))

func jitterFloat64() float64 {
	jitterMu.Lock()
	defer jitterMu.Unlock()
	return jitterRng.Float64()
}

func computeBackoff(attempt int, o RetryOptions) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if o.BaseDelay <= 0 {
		o.BaseDelay = 500 * time.Millisecond
	}
	if o.MaxDelay <= 0 {
		o.MaxDelay = 10 * time.Second
	}
	if o.Jitter < 0 {
		o.Jitter = 0
	}
	if o.Jitter > 1 {
		o.Jitter = 1
	}

	// exponential: base * 2^(attempt-1)
	d := time.Duration(float64(o.BaseDelay) * math.Pow(2, float64(attempt-1)))
	if d > o.MaxDelay {
		d = o.MaxDelay
	}
	if d < 0 {
		d = 0
	}

	if o.Jitter > 0 {
		// +/- jitter. Thread-safe RNG without reseeding per attempt.
		j := (jitterFloat64()*2 - 1) * o.Jitter
		d = time.Duration(float64(d) * (1 + j))
		if d < 0 {
			d = 0
		}
		if d > o.MaxDelay {
			d = o.MaxDelay
		}
	}
	return d
}

type retryDecision struct {
	err   error
	delay time.Duration
	retry bool
}

func requestWithRetry[T any](
	ctx context.Context,
	o RetryOptions,
	do func(attempt int) (T, retryDecision),
) (T, error) {
	var zero T
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= o.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}

		v, d := do(attempt)
		if d.err == nil {
			return v, nil
		}
		lastErr = d.err

		if d.retry && attempt < o.MaxAttempts {
			delay := d.delay
			if delay <= 0 {
				delay = computeBackoff(attempt, o)
			}
			slog.Warn("Sleeping before retrying request", "attempt", attempt, "delay", delay, "error", lastErr)
			if err := sleepWithContext(ctx, delay); err != nil {
				return zero, err
			}
			continue
		}
		return zero, d.err
	}

	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return zero, lastErr
}
