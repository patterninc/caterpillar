package kafka

import "sync"

// offsetTracker tracks a contiguous completed prefix per partition so stored
// offsets match Kafka's next-to-read convention.
type offsetTracker struct {
	mu         sync.Mutex
	partitions map[int32]*partitionState
}

type partitionState struct {
	nextCommit   int64
	haveNext     bool
	pending      map[int64]struct{}
	lowestFailed int64
	paused       bool
}

func newOffsetTracker() *offsetTracker {
	return &offsetTracker{
		partitions: make(map[int32]*partitionState),
	}
}

func (t *offsetTracker) partition(partition int32) *partitionState {
	ps, ok := t.partitions[partition]
	if !ok {
		ps = &partitionState{
			pending:      make(map[int64]struct{}),
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

	if !ps.haveNext {
		ps.nextCommit = offset
		ps.haveNext = true
	}

	if failed {
		if ps.lowestFailed < 0 || offset < ps.lowestFailed {
			ps.lowestFailed = offset
		}
		ps.paused = true
		commitTo = ps.cappedCommit()
		return commitTo, ps.haveNext, true
	}

	if offset < ps.nextCommit {
		return -1, false, ps.paused
	}

	if offset == ps.nextCommit {
		ps.nextCommit++
	} else {
		ps.pending[offset] = struct{}{}
	}

	t.advance(ps)

	commitTo = ps.cappedCommit()
	if !ps.haveNext {
		return -1, false, ps.paused
	}

	return commitTo, true, ps.paused
}

func (t *offsetTracker) advance(ps *partitionState) {
	for {
		if ps.lowestFailed >= 0 && ps.nextCommit >= ps.lowestFailed {
			return
		}
		if _, ok := ps.pending[ps.nextCommit]; !ok {
			return
		}
		delete(ps.pending, ps.nextCommit)
		ps.nextCommit++
	}
}

func (ps *partitionState) cappedCommit() int64 {
	if ps.lowestFailed >= 0 && ps.nextCommit > ps.lowestFailed {
		return ps.lowestFailed
	}
	return ps.nextCommit
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
		if ps.haveNext {
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
