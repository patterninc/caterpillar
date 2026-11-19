package kafka

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOffsetTrackerCommitsContiguousPrefix(t *testing.T) {

	tr := newOffsetTracker()
	for _, o := range []int64{10, 11, 12} {
		tr.observe(0, o)
	}

	commit, store, pause := tr.settle(0, 11, false)
	assert.Equal(t, int64(10), commit, "10 is still in flight")
	assert.True(t, store)
	assert.False(t, pause)

	commit, _, _ = tr.settle(0, 10, false)
	assert.Equal(t, int64(12), commit)

	commit, _, _ = tr.settle(0, 12, false)
	assert.Equal(t, int64(13), commit, "everything settled: commit past the last observed offset")

}

func TestOffsetTrackerSkipsLogHoles(t *testing.T) {

	tr := newOffsetTracker()
	tr.observe(0, 5)
	tr.observe(0, 9)

	tr.settle(0, 5, false)
	commit, _, _ := tr.settle(0, 9, false)
	assert.Equal(t, int64(10), commit)

}

func TestOffsetTrackerFailureCapsCommitAndPauses(t *testing.T) {

	tr := newOffsetTracker()
	for _, o := range []int64{1, 2, 3} {
		tr.observe(0, o)
	}

	commit, store, pause := tr.settle(0, 2, true)
	assert.Equal(t, int64(1), commit)
	assert.True(t, store)
	assert.True(t, pause)

	tr.settle(0, 1, false)
	commit, _, pause = tr.settle(0, 3, false)
	assert.Equal(t, int64(2), commit, "never commit past the failed offset")
	assert.True(t, pause)

	assert.Equal(t, []int32{0}, tr.cappedPartitions())
	assert.Equal(t, map[int32]int64{0: 2}, tr.commitPositions())

}

func TestOffsetTrackerSettleBeforeObserve(t *testing.T) {

	tr := newOffsetTracker()

	commit, store, pause := tr.settle(3, 7, false)
	assert.Equal(t, int64(-1), commit)
	assert.False(t, store)
	assert.False(t, pause)

	_, store, pause = tr.settle(3, 7, true)
	assert.False(t, store)
	assert.True(t, pause)

	assert.Empty(t, tr.commitPositions())
	assert.Equal(t, []int32{3}, tr.cappedPartitions())

}

func TestOffsetTrackerPartitionsAreIndependent(t *testing.T) {

	tr := newOffsetTracker()
	tr.observe(0, 1)
	tr.observe(1, 100)
	tr.settle(1, 100, true)
	tr.settle(0, 1, false)

	assert.Equal(t, map[int32]int64{0: 2, 1: 100}, tr.commitPositions())

	capped := tr.cappedPartitions()
	sort.Slice(capped, func(i, j int) bool { return capped[i] < capped[j] })
	assert.Equal(t, []int32{1}, capped)

}

func TestOffsetTrackerAllAssignedPaused(t *testing.T) {

	tr := newOffsetTracker()
	assert.False(t, tr.allAssignedPaused(nil))

	tr.observe(0, 1)
	tr.observe(1, 1)
	tr.settle(0, 1, true)
	assert.False(t, tr.allAssignedPaused([]int32{0, 1}))
	assert.False(t, tr.allAssignedPaused([]int32{0, 2}))

	tr.settle(1, 1, true)
	assert.True(t, tr.allAssignedPaused([]int32{0, 1}))

}
