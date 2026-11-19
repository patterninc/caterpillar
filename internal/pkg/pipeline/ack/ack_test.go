package ack

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// settled reports whether a has finished, without blocking.
func settled(a *Ack) bool {
	select {
	case <-a.Wait():
		return true
	default:
		return false
	}
}

func mustSettle(t *testing.T, a *Ack, wantFailed bool, format string, args ...any) {

	t.Helper()

	label := fmt.Sprintf(format, args...)

	if !settled(a) {
		t.Fatalf("%s: ack never settled", label)
	}
	if got := a.Failed(); got != wantFailed {
		t.Fatalf("%s: Failed() = %v, want %v", label, got, wantFailed)
	}

}

// send models what Base.SendRecord does on every emit: register one branch.
func send(ctx context.Context) {
	if a, ok := FromContext(ctx); ok {
		a.AddBranch(1)
	}
}

// TestShapes walks the accounting rule — every send registers a branch, every task
// releases each record it consumes exactly once — over the pipeline shapes that
// exist, asserting each source record settles exactly once.
func TestShapes(t *testing.T) {

	tests := []struct {
		name       string
		run        func(ctx context.Context)
		wantFailed bool
	}{
		{
			name: "source emit then terminal sink releases",
			run: func(ctx context.Context) {
				send(ctx)    // source emits
				Release(ctx) // sink is done with it
			},
		},
		{
			name: "one-to-one transform",
			run: func(ctx context.Context) {
				send(ctx)    // source emits
				send(ctx)    // transform emits its derived record
				Release(ctx) // transform releases its input
				Release(ctx) // sink releases
			},
		},
		{
			name: "fan-out to three, then all three sink",
			run: func(ctx context.Context) {
				send(ctx)
				for range 3 {
					send(ctx)
				}
				Release(ctx) // the fanning task releases its one input
				for range 3 {
					Release(ctx) // each branch reaches a sink
				}
			},
		},
		{
			name: "fan-out to zero is a drop",
			run: func(ctx context.Context) {
				send(ctx)
				Release(ctx) // filtered away without emitting
			},
		},
		{
			name: "chained fan-out: 2 then each to 2",
			run: func(ctx context.Context) {
				send(ctx)
				send(ctx)
				send(ctx)
				Release(ctx) // first task
				for range 2 {
					send(ctx)
					send(ctx)
					Release(ctx) // second task, once per input
				}
				for range 4 {
					Release(ctx)
				}
			},
		},
		{
			name: "reject anywhere fails the record",
			run: func(ctx context.Context) {
				send(ctx)
				send(ctx)
				Release(ctx)
				Reject(ctx) // the derived record never made it
			},
			wantFailed: true,
		},
		{
			name: "one failed branch of three fails the record",
			run: func(ctx context.Context) {
				send(ctx)
				for range 3 {
					send(ctx)
				}
				Release(ctx)
				Release(ctx)
				Reject(ctx)
				Release(ctx)
			},
			wantFailed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			a := New()
			ctx := WithContext(context.Background(), a)

			if settled(a) {
				t.Fatal("a fresh ack must not be settled")
			}

			tc.run(ctx)

			mustSettle(t, a, tc.wantFailed, "%s", tc.name)

		})
	}

}

// TestPrematureSettle is the property the whole design turns on: while any
// descendant is still outstanding, the source record must not settle.
func TestPrematureSettle(t *testing.T) {

	a := New()
	ctx := WithContext(context.Background(), a)

	send(ctx) // source emits

	// a task fans out to three and releases its input. Three branches outstanding.
	send(ctx)
	send(ctx)
	send(ctx)
	Release(ctx)

	for i := range 3 {
		if settled(a) {
			t.Fatalf("settled with %d branch(es) still outstanding", 3-i)
		}
		Release(ctx)
	}

	mustSettle(t, a, false, "after all three branches")

}

// TestSettleOnce covers the guard that keeps a miscount from closing done twice,
// which would panic and take the process down.
func TestSettleOnce(t *testing.T) {

	t.Run("release after settling is a no-op", func(t *testing.T) {

		a := New()
		ctx := WithContext(context.Background(), a)

		send(ctx)
		Release(ctx)
		mustSettle(t, a, false, "first release")

		// a buggy task releasing twice must not re-settle
		Release(ctx)
		Release(ctx)

		mustSettle(t, a, false, "after extra releases")

	})

	t.Run("branch registered after settling is ignored", func(t *testing.T) {

		a := New()
		ctx := WithContext(context.Background(), a)

		send(ctx)
		Release(ctx)
		mustSettle(t, a, false, "first release")

		// without the guard this reopens the counter, and the Release below
		// drives it to zero a second time -> close of closed channel
		send(ctx)
		Release(ctx)

		mustSettle(t, a, false, "after late branch")

	})

	t.Run("reject after settling does not flip the outcome", func(t *testing.T) {

		a := New()
		ctx := WithContext(context.Background(), a)

		send(ctx)
		Release(ctx)
		Reject(ctx)

		if a.Failed() {
			t.Fatal("a record that already succeeded must not become failed")
		}

	})

	t.Run("reject settles immediately, before siblings finish", func(t *testing.T) {

		a := New()
		ctx := WithContext(context.Background(), a)

		send(ctx)
		send(ctx)
		send(ctx)
		Release(ctx)

		Reject(ctx)

		// one leaked sibling must not suppress the failure, or the source can
		// never learn why the record needs redelivery
		mustSettle(t, a, true, "on reject with siblings outstanding")

	})

}

// TestJoined covers aggregation: an aggregator does not release its buffered
// inputs, the combined record's outcome settles them.
func TestJoined(t *testing.T) {

	t.Run("combined record settles every input", func(t *testing.T) {

		parents := make([]*Ack, 3)
		ctxs := make([]context.Context, 3)
		for i := range parents {
			parents[i] = New()
			ctxs[i] = WithContext(context.Background(), parents[i])
			send(ctxs[i]) // each source emitted its record
		}

		joined := Joined(ctxs...)
		joinedCtx := WithContext(context.Background(), joined)
		send(joinedCtx) // the aggregator emits the combined record

		for i, p := range parents {
			if settled(p) {
				t.Fatalf("input %d settled before the archive carrying it was handled", i)
			}
		}

		Release(joinedCtx) // the sink handled the combined record

		for i, p := range parents {
			mustSettle(t, p, false, "input %d", i)
		}

	})

	t.Run("failing the combined record fails every input", func(t *testing.T) {

		parents := make([]*Ack, 2)
		ctxs := make([]context.Context, 2)
		for i := range parents {
			parents[i] = New()
			ctxs[i] = WithContext(context.Background(), parents[i])
			send(ctxs[i])
		}

		joined := Joined(ctxs...)
		joinedCtx := WithContext(context.Background(), joined)
		send(joinedCtx)
		Reject(joinedCtx)

		for i, p := range parents {
			mustSettle(t, p, true, "input %d", i)
		}

	})

	t.Run("one source contributing twice is completed twice", func(t *testing.T) {

		// an upstream fan-out sent two records from one source message, and both
		// landed in the same batch. Collapsing them would settle the source
		// while the batch was still in flight.
		a := New()
		ctx := WithContext(context.Background(), a)

		send(ctx) // source emit
		send(ctx) // fan-out: two records from it
		send(ctx)
		Release(ctx) // the fanning task released its input

		joined := Joined(ctx, ctx) // both contributed to one batch
		joinedCtx := WithContext(context.Background(), joined)
		send(joinedCtx)

		if settled(a) {
			t.Fatal("settled before the batch was handled")
		}

		Release(joinedCtx)

		mustSettle(t, a, false, "after the batch was handled")

	})

}

// TestUntracked: a pipeline with no acking source must be completely unaffected.
func TestUntracked(t *testing.T) {

	ctx := context.Background()

	// none of these have an ack to find, and none may panic
	send(ctx)
	Release(ctx)
	Reject(ctx)

	if err := Rejected(ctx, context.Canceled); err != context.Canceled {
		t.Fatalf("Rejected must pass the error through, got %v", err)
	}

	if _, ok := FromContext(ctx); ok {
		t.Fatal("a bare context must not yield an ack")
	}

	// Joined over untracked contexts yields an ack with no children
	j := Joined(ctx, ctx)
	jctx := WithContext(context.Background(), j)
	send(jctx)
	Release(jctx)
	mustSettle(t, j, false, "joined over untracked inputs")

}

// TestWithContextNilParent covers the aggregator that has no input record to
// inherit a context from.
func TestWithContextNilParent(t *testing.T) {

	a := New()
	ctx := WithContext(context.TODO(), a)

	if got, ok := FromContext(ctx); !ok || got != a {
		t.Fatal("ack must be recoverable from a context built on a nil parent")
	}

}

// TestConcurrent runs the emit/release cycle from many goroutines, so -race can
// find unsynchronised access to the counter and the settle guard.
func TestConcurrent(t *testing.T) {

	const (
		workers = 16
		perTask = 200
	)

	a := New()
	ctx := WithContext(context.Background(), a)

	send(ctx) // the source's own emit, so the record is live while workers run

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perTask {
				send(ctx) // fan out
				Release(ctx)
			}
		}()
	}

	wg.Wait()

	if settled(a) {
		t.Fatal("settled while the source's own branch was still outstanding")
	}

	Release(ctx)

	mustSettle(t, a, false, "after the final release")

}
