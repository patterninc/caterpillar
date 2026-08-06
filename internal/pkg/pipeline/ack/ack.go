// Package ack tracks completion of a single source record as it flows through
// a pipeline, so the source task can defer acknowledging it until every
// downstream branch produced from it has finished processing.
package ack

import (
	"context"
	"sync/atomic"
)

type catterpillarAckKey string

const CATERPILLAR_ACK catterpillarAckKey = "CATERPILLAR_ACK"

// Ack is created once per source record with exactly one pending branch: the
// record itself. It rides downstream on the record's context.Context, so tasks
// that only transform a record need no explicit wiring.
//
// Tasks that change a record's branch count must say so before sending: Fanout
// for one-to-many, Joined for many-to-one, Drop or Reject for a record they
// don't forward. A task with a nil output channel settles every record it
// consumes.
type Ack struct {
	remaining atomic.Int32
	failed    atomic.Bool
	done      chan struct{}
	children  []*Ack // settled with this Ack's own outcome once it completes; see Joined
}

// New returns an Ack with a single pending branch.
func New() *Ack {
	a := &Ack{done: make(chan struct{})}
	a.remaining.Store(1)
	return a
}

// AddBranch registers cnt additional branches that must call Done or Fail
// before Wait's channel closes.
func (a *Ack) AddBranch(cnt int32) {
	a.remaining.Add(cnt)
}

// Done marks one branch as complete.
func (a *Ack) Done() {
	a.complete(false)
}

// Fail marks one branch as complete but unsuccessful, so Failed reports true
// once every branch has finished. Use it when a branch didn't make it to where
// it needed to go, so the source knows not to acknowledge the record.
func (a *Ack) Fail() {
	a.complete(true)
}

func (a *Ack) complete(failed bool) {

	if failed {
		a.failed.Store(true)
	}

	if a.remaining.Add(-1) != 0 {
		return
	}

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

// Fanout adjusts the Ack embedded in ctx, if any, so it represents n branches
// derived from the single branch ctx currently carries. Call it once, before
// sending any of the n outputs, so no downstream Done/Fail can race ahead of
// the adjustment. n counts sends rather than distinct records; n == 0
// completes the Ack immediately.
func Fanout(ctx context.Context, n int) {

	a, ok := FromContext(ctx)
	if !ok {
		return
	}

	switch {
	case n == 0:
		a.Done()
	case n > 1:
		a.AddBranch(int32(n - 1))
	}

}

// Drop completes the Ack embedded in ctx, if any, for a record a task decided
// not to forward downstream (e.g. it was filtered out). It is the
// single-record equivalent of Fanout(ctx, 0).
func Drop(ctx context.Context) {
	if a, ok := FromContext(ctx); ok {
		a.Done()
	}
}

// Reject completes the Ack embedded in ctx, if any, as failed, for a record a
// task could not process. Where Drop means the record is legitimately finished
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

// Joined returns a new Ack with a single pending branch representing one output
// record combined from several inputs. Attach it to that output via
// WithContext: completing it transitively completes every Ack found among
// ctxs, so the combined record's outcome is attributed back to each record
// that went into it. ctxs with no Ack attached are ignored.
func Joined(ctxs ...context.Context) *Ack {

	a := New()

	for _, c := range ctxs {
		if child, ok := FromContext(c); ok {
			a.children = append(a.children, child)
		}
	}

	return a

}
