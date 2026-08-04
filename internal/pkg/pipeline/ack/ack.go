// Package ack tracks completion of a single source record (e.g. an SQS
// message) as it flows through a pipeline, so the source task can defer
// acknowledging it (e.g. deleting the SQS receipt) until every downstream
// branch produced from it has finished processing.
package ack

import (
	"context"
	"sync/atomic"
)

type catterpillarAckKey string

const CATERPILLAR_ACK catterpillarAckKey = "CATERPILLAR_ACK"

// Ack is created once per source record with exactly one pending branch:
// the record itself. It rides downstream attached to the record's
// context.Context (see WithContext/FromContext), so tasks that just
// transform a record in place need no explicit wiring at all.
//
// Tasks that fan a single input record out into multiple output records
// (e.g. split, or jq with explode) must call AddBranch(n-1) before sending
// the n outputs, so Wait's channel only closes once all n have completed
// (see the Fanout helper below). Tasks that decide not to forward a record
// at all (a filter, an empty query result) must call Done or Fail exactly
// once for it (see Drop). Tasks that fan multiple input records IN into a
// single output record (e.g. join, or archiving many records into one
// file) must attach the Ack returned by Joined to that output record
// instead of any one input's Ack, so completing the joined output
// transitively completes every record that went into it (see Joined).
// Terminal tasks (those with a nil output channel) must call Done, or
// Fail, once they finish processing each record they consume.
type Ack struct {
	remaining atomic.Int32
	failed    atomic.Bool
	done      chan struct{}
	children  []*Ack // completed (Done or Fail, per failed) once this Ack itself completes; see Joined
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

// Fail marks one branch as complete but unsuccessful, so Failed reports
// true once every branch has finished. Use this instead of Done when a
// branch didn't actually make it to where it needed to go (e.g. a Kafka
// delivery failure), so the source knows not to acknowledge the record.
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

	// this Ack itself is now fully complete: propagate that to whatever
	// Acks it was joined from, based on whether ANY of its own branches
	// failed (not just this particular call), since a single downstream
	// success/failure on the joined record applies to all of them equally.
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
// FromContext as the record moves downstream (record.Record.Context is
// forwarded by tasks even when they construct a new *record.Record). A nil
// ctx (e.g. an aggregating task, like archive pack, that never actually
// received a record to inherit a context from) is treated as
// context.Background() instead of panicking.
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

// Fanout adjusts the Ack embedded in ctx, if any, so it represents n
// branches derived from the single incoming branch ctx currently carries.
// Call it once, before sending any of the n outputs: this guarantees the
// adjustment is visible before any of those outputs can reach a downstream
// Done/Fail call, which would otherwise be able to race ahead of it. n may
// be 0 (the incoming record produces no output and is immediately
// completed) or repeat a record multiple times (n counts sends, not
// distinct records).
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

// Drop completes the Ack embedded in ctx, if any, for a record a task
// decided not to forward downstream (e.g. it was filtered out). It is the
// single-record equivalent of Fanout(ctx, 0).
func Drop(ctx context.Context) {
	if a, ok := FromContext(ctx); ok {
		a.Done()
	}
}

// Reject completes the Ack embedded in ctx, if any, as FAILED, for a record a
// task could not process. It is the counterpart of Drop: Drop means "this
// record is legitimately finished with", Reject means "this record never made
// it", so the source leaves it unacknowledged and the broker redelivers it
// instead of the pipeline waiting forever for a completion that can't come.
//
// A task bailing out mid-stream should Reject both the record it failed on and
// every record still queued behind it, since it won't be processing those
// either.
func Reject(ctx context.Context) {
	if a, ok := FromContext(ctx); ok {
		a.Fail()
	}
}

// Rejected is Reject followed by err, for the common case of a task bailing
// out on the record it is holding:
//
//	if err != nil {
//		return ack.Rejected(r.Context, err)
//	}
//
// Keeping the settle and the return on one line stops the two drifting apart -
// a bare `return err` here strands the record, and the symptom is the whole
// pipeline hanging at shutdown rather than anything that points back to this
// line.
func Rejected(ctx context.Context, err error) error {
	Reject(ctx)
	return err
}

// Joined returns a new Ack with a single pending branch representing one
// output record produced by combining n inputs (a "fan-in"), such as join
// or archiving several records into one file. Attach it to that output
// record via WithContext before sending it. Completing the returned Ack
// (Done or Fail, however the output record's own journey downstream ends)
// transitively completes every Ack found among ctxs, so a downstream
// success or failure on the combined record is correctly attributed back
// to each record that went into it. ctxs with no Ack attached are ignored.
func Joined(ctxs ...context.Context) *Ack {

	a := New()

	for _, c := range ctxs {
		if child, ok := FromContext(c); ok {
			a.children = append(a.children, child)
		}
	}

	return a

}
