package kafka

import "sync"

// offsetTracker tracks the next-to-read store position per partition from
// received offsets that have not settled, so log holes are not treated as gaps.
type offsetTracker struct {
	mu         sync.Mutex
	partitions map[int32]*partitionState
}

type partitionState struct {
	inFlight     map[int64]struct{}
	maxObserved  int64
	haveObserved bool
	lowestFailed int64
	paused       bool
}

func newOffsetTracker() *offsetTracker {
	return &offsetTracker{
		partitions: make(map[int32]*partitionState),
	}
}

// observe records a received offset; every read must be observed so holes in
// the log are not mistaken for unfinished records.
func (t *offsetTracker) observe(partition int32, offset int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ps := t.partition(partition)
	ps.inFlight[offset] = struct{}{}
	if !ps.haveObserved || offset > ps.maxObserved {
		ps.maxObserved = offset
		ps.haveObserved = true
	}
}

func (t *offsetTracker) partition(partition int32) *partitionState {
	ps, ok := t.partitions[partition]
	if !ok {
		ps = &partitionState{
			inFlight:     make(map[int64]struct{}),
			lowestFailed: -1,
		}
		t.partitions[partition] = ps
	}
	return ps
}

// commitTo is the next offset Kafka should read, not the last completed one.
func (t *offsetTracker) settle(partition int32, offset int64, failed bool) (commitTo int64, shouldStore bool, pause bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ps := t.partition(partition)

	if !ps.haveObserved {
		if failed {
			if ps.lowestFailed < 0 || offset < ps.lowestFailed {
				ps.lowestFailed = offset
			}
			ps.paused = true
		}
		return -1, false, ps.paused
	}

	delete(ps.inFlight, offset)

	if failed {
		if ps.lowestFailed < 0 || offset < ps.lowestFailed {
			ps.lowestFailed = offset
		}
		ps.paused = true
		return ps.cappedCommit(), true, true
	}

	return ps.cappedCommit(), true, ps.paused
}

func (ps *partitionState) cappedCommit() int64 {
	commitTo := ps.maxObserved + 1
	if len(ps.inFlight) > 0 {
		commitTo = -1
		for offset := range ps.inFlight {
			if commitTo < 0 || offset < commitTo {
				commitTo = offset
			}
		}
	}
	if ps.lowestFailed >= 0 && commitTo > ps.lowestFailed {
		return ps.lowestFailed
	}
	return commitTo
}

func (t *offsetTracker) cappedPartitions() []int32 {
	t.mu.Lock()
	defer t.mu.Unlock()

	var out []int32
	for partition, ps := range t.partitions {
		if ps.lowestFailed >= 0 {
			out = append(out, partition)
		}
	}
	return out
}

func (t *offsetTracker) commitPositions() map[int32]int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make(map[int32]int64, len(t.partitions))
	for partition, ps := range t.partitions {
		if ps.haveObserved {
			out[partition] = ps.cappedCommit()
		}
	}
	return out
}

func (t *offsetTracker) allAssignedPaused(assignment []int32) bool {
	if len(assignment) == 0 {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, partition := range assignment {
		ps := t.partitions[partition]
		if ps == nil || !ps.paused {
			return false
		}
	}
	return true
}
