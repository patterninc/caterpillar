package ack

import "sync"

// Acknowledger is the broker-specific half of deferred acknowledgement: how
// one particular source settles a single message once the pipeline is done
// with it. SQS deletes the message's receipt, Kafka stores its offset,
// another broker does something else again - Tracker owns the bookkeeping
// that's common to all of them, an Acknowledger owns what isn't.
//
// Implementations are typically a small per-message struct holding whatever
// handle the client needs (a receipt, a partition and offset, ...).
type Acknowledger interface {
	// Ack settles one message. failed reports whether any downstream branch
	// signalled Fail rather than Done, in which case the message wasn't
	// fully processed: implementations should normally leave it
	// unacknowledged so the broker redelivers it instead of losing it.
	//
	// Ack is called at most once per message, from its own goroutine, and no
	// more than the Tracker's concurrency at a time. It should log rather
	// than panic on client errors, since by the time it runs the record is
	// already through the pipeline and the worst case is a redelivery.
	Ack(failed bool)
}

// Tracker lets a source task defer acknowledging a message (deleting an SQS
// receipt, committing a Kafka offset) until every downstream task has
// finished with the record produced from it, without each source having to
// re-implement the bookkeeping.
//
// Nothing here gates the source's receive loop. A limit on unacknowledged
// messages sounds prudent, but releasing it depends on downstream completion,
// which deadlocks against any fan-in task that must accumulate records before
// it can emit anything: the source stops receiving at the limit, the fan-in
// never reaches its flush threshold, so nothing ever completes and no slot
// ever frees. The number of messages in flight is instead bounded by the
// pipeline's channel capacity, which is what applies backpressure already.
//
// A Tracker must be created with NewTracker; the zero value is not usable.
type Tracker struct {
	slots chan struct{} // bounds concurrent Ack calls; acquired only after settling
	wg    sync.WaitGroup
}

// NewTracker returns a Tracker that runs at most concurrency Ack calls at a
// time. A value below 1 is treated as 1.
func NewTracker(concurrency int) *Tracker {

	if concurrency < 1 {
		concurrency = 1
	}

	return &Tracker{slots: make(chan struct{}, concurrency)}

}

// Track watches a in the background and calls target.Ack once every
// downstream task has signalled Done or Fail for it, passing on whether any
// of them failed. It does not block.
//
// Every Track for a given Tracker must happen before its Wait.
func (t *Tracker) Track(a *Ack, target Acknowledger) {

	t.wg.Add(1)

	go func() {

		defer t.wg.Done()

		<-a.Wait()

		// bound how many broker calls run at once. Taking the slot here
		// rather than before the wait is what keeps this safe: by now the
		// record is through the pipeline, so releasing the slot depends only
		// on Ack returning, never on the pipeline making further progress.
		t.slots <- struct{}{}
		defer func() { <-t.slots }()

		target.Ack(a.Failed())

	}()

}

// Wait blocks until every tracked Ack has settled and its acknowledgement
// has been carried out. It is idempotent.
func (t *Tracker) Wait() {
	t.wg.Wait()
}
