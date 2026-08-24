package ack

import "sync"

// Acknowledger is the broker-specific half of deferred acknowledgement: how a
// source settles a single message once the pipeline is done with it. Tracker
// owns the bookkeeping common to every broker, an Acknowledger owns what isn't.
type Acknowledger interface {
	// Ack settles one message. failed reports whether any downstream branch
	// signalled Fail, in which case implementations should normally leave the
	// message unacknowledged so the broker redelivers it. Called at most once
	// per message, from its own goroutine, and at most the Tracker's
	// concurrency at a time.
	Ack(failed bool)
}

// Tracker lets a source task defer acknowledging a message until every
// downstream task has finished with the record produced from it, without each
// source having to re-implement the bookkeeping.
//
// Nothing here gates the source's receive loop: a cap on unacknowledged
// messages would deadlock against any fan-in task that must accumulate records
// before it can emit, since freeing a slot depends on the very completion the
// fan-in is waiting to produce. Pipeline occupancy is already bounded by
// channel capacity; acks that have settled but not yet been deleted wait only
// on the concurrency slots around Ack, so a slow broker can accumulate them.
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

		// take the slot after the wait, not before: the record is already
		// through the pipeline, so freeing it depends only on Ack returning
		// and never on the pipeline making further progress.
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
