package event

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"

	"github.com/SpherePrime/CLI/vendordeps/posthog/posthog-go"
)

// unreachableBatches is how many consecutive batches may fail to reach the
// ingestion endpoint before telemetry gives up for the rest of the session. The
// client already retries a batch several times before reporting it as dropped,
// so this counts confirmation rounds rather than HTTP attempts.
const unreachableBatches = 2

var _ posthog.Callback = (*deliveryBreaker)(nil)

// deliveryBreaker watches delivery results and turns telemetry off when the
// endpoint cannot be contacted at all: a name that does not resolve, no route,
// or a refused connection. Those do not fix themselves mid-session, and
// without this the client keeps dialing the same dead host on every flush.
type deliveryBreaker struct {
	// closeFn shuts the telemetry client down. It is called from its own
	// goroutine because the delivery callback runs on the goroutine Close waits
	// for.
	closeFn func() error

	mu      sync.Mutex
	last    error
	batches int

	unreachable atomic.Bool
	tripOnce    sync.Once
}

func newDeliveryBreaker(closeFn func() error) *deliveryBreaker {
	return &deliveryBreaker{closeFn: closeFn}
}

// enabled reports whether messages may still be handed to the client.
func (b *deliveryBreaker) enabled() bool { return !b.unreachable.Load() }

// Success resets the streak, so an endpoint that comes back mid-session keeps
// working.
func (b *deliveryBreaker) Success(posthog.APIMessage) {
	b.mu.Lock()
	b.last = nil
	b.batches = 0
	b.mu.Unlock()
}

// Failure counts one dropped message. The client reports every message of a
// batch with the same error value, so the batch is counted once by tracking
// that value.
func (b *deliveryBreaker) Failure(_ posthog.APIMessage, err error) {
	if !isUnreachable(err) {
		return
	}
	b.mu.Lock()
	if err != b.last {
		b.last = err
		b.batches++
	}
	reached := b.batches >= unreachableBatches
	b.mu.Unlock()

	if !reached {
		return
	}
	b.trip(err)
}

// trip stops telemetry once for the session and lets the client go.
func (b *deliveryBreaker) trip(err error) {
	b.tripOnce.Do(func() {
		b.unreachable.Store(true)
		slog.Info("Analytics endpoint unreachable, disabling telemetry for this session", "error", err)
		go func() {
			if b.closeFn == nil {
				return
			}
			if closeErr := b.closeFn(); closeErr != nil && !errors.Is(closeErr, posthog.ErrClosed) {
				slog.Debug("Failed to shut down the analytics client", "error", closeErr)
			}
		}()
	})
}

// isUnreachable reports whether err means the host could not be contacted at
// all. A server that answers badly is a different problem: it may recover, and
// the client's own retries handle it.
func isUnreachable(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return !isTimeout(opErr)
	}
	return false
}

func isTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, os.ErrDeadlineExceeded)
}
