// Package ack tracks completion of a single source record as it flows through
// a pipeline, so the source task can defer acknowledging it until every
// downstream branch produced from it has finished processing.
package ack

import (
	"context"
	"sync"
	"sync/atomic"
)

type catterpillarAckKey string

const CATERPILLAR_ACK catterpillarAckKey = "CATERPILLAR_ACK"

// Ack is created once per source record and rides downstream on its context. The
// counter is the live branches descending from that record, and each send registers
// its own — so a task declares no fan-out count, it settles what it consumes.
type Ack struct {
	mu        sync.Mutex
	settled   bool
	remaining atomic.Int32
	failed    atomic.Bool
	done      chan struct{}
	children  []*Ack // settled with this Ack's own outcome once it completes; see Joined
}

// New returns an Ack with no branches yet. The send that puts its record on an
// output channel registers the first one, which is why a counter that starts at
// one would never reach zero.
func New() *Ack {
	return &Ack{done: make(chan struct{})}
}

// AddBranch registers cnt additional branches that must call Done or Fail
// before Wait's channel closes. Sending a record does this for the caller.
func (a *Ack) AddBranch(cnt int32) {

	a.mu.Lock()
	defer a.mu.Unlock()

	// a settled record has already been reported to the source; re-opening its
	// counter would let it settle a second time and close done twice.
	if a.settled {
		return
	}

	a.remaining.Add(cnt)

}

// Done marks one branch as complete, settling the record if it was the last.
func (a *Ack) Done() {

	a.mu.Lock()

	if a.settled || a.remaining.Add(-1) != 0 {
		a.mu.Unlock()
		return
	}

	a.settled = true
	a.mu.Unlock()

	a.finish()

}

// Fail abandons the record so the broker redelivers it. It settles immediately
// rather than decrementing like Done, because a sibling branch that never
// completes would otherwise suppress the failure and hang the source.
func (a *Ack) Fail() {

	a.mu.Lock()

	// after settling, the outcome is already reported: a late Fail must not flip a
	// record the source has been told to acknowledge into one it redelivers.
	if a.settled {
		a.mu.Unlock()
		return
	}

	a.settled = true
	a.failed.Store(true)
	a.mu.Unlock()

	a.finish()

}

func (a *Ack) finish() {

	// a joined record's single downstream outcome applies equally to every
	// record that went into it, so children get this Ack's overall result
	// rather than the outcome of this particular call.
	anyFailed := a.failed.Load()
	for _, c := range a.children {
		if anyFailed {
			c.Fail()
		} else {
			c.Done()
		}
	}

	close(a.done)

}

// Failed reports whether any branch called Fail instead of Done. Only
// meaningful after Wait's channel has closed.
func (a *Ack) Failed() bool {
	return a.failed.Load()
}

// Wait returns a channel that closes once every branch has called Done or
// Fail.
func (a *Ack) Wait() <-chan struct{} {
	return a.done
}

// WithContext returns a copy of ctx carrying a, recoverable later via
// FromContext. A nil ctx is treated as context.Background(), since an
// aggregating task may never have received a record to inherit one from.
func WithContext(ctx context.Context, a *Ack) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, CATERPILLAR_ACK, a)
}

// FromContext recovers the Ack embedded in ctx, if any.
func FromContext(ctx context.Context) (*Ack, bool) {
	a, ok := ctx.Value(CATERPILLAR_ACK).(*Ack)
	return a, ok
}

// Release completes one branch: this task is finished with the record, whether it
// forwarded, fanned out, or filtered it. Exactly once per record consumed —
// twice acknowledges early, never leaves it for redelivery.
func Release(ctx context.Context) {
	if a, ok := FromContext(ctx); ok {
		a.Done()
	}
}

// Reject completes the Ack embedded in ctx, if any, as failed, for a record a
// task could not process. Where Release means the record is legitimately finished
// with, Reject means it never made it: the source leaves it unacknowledged and
// the broker redelivers it, rather than the pipeline waiting for a completion
// that can't come.
func Reject(ctx context.Context) {
	if a, ok := FromContext(ctx); ok {
		a.Fail()
	}
}

// Rejected is Reject followed by err, for a task bailing out on the record it
// is holding. Keeping the settle and the return on one line stops the two
// drifting apart: a bare return strands the record, and the symptom is the
// whole pipeline hanging at shutdown rather than anything pointing here.
func Rejected(ctx context.Context, err error) error {
	Reject(ctx)
	return err
}

// Joined returns an Ack for one record combined from several inputs; completing it
// transitively completes every Ack among ctxs. An aggregator therefore does not
// Release its inputs — this settles them, per contribution rather than per Ack.
func Joined(ctxs ...context.Context) *Ack {

	a := New()

	for _, c := range ctxs {
		if child, ok := FromContext(c); ok {
			a.children = append(a.children, child)
		}
	}

	return a

}
