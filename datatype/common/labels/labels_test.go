package labels

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/janelia-flyem/dvid/dvid"
)

func TestMapping(t *testing.T) {
	var m Mapping
	m.set(1, 4)
	m.set(2, 5)
	m.set(20, 6)
	m.set(6, 32)
	m.set(32, 21)
	if v, ok := m.Get(1); v != 4 || !ok {
		t.Errorf("Incorrect mapping on Get.  Got %d, %t\n", v, ok)
	}
	if v, ok := m.Get(2); v != 5 || !ok {
		t.Errorf("Incorrect mapping on Get.  Got %d, %t\n", v, ok)
	}
	if v, ok := m.Get(20); v != 6 || !ok {
		t.Errorf("Incorrect mapping on Get.  Got %d, %t\n", v, ok)
	}
	if v, ok := m.Get(32); v != 21 || !ok {
		t.Errorf("Incorrect mapping on Get.  Got %d, %t\n", v, ok)
	}
	if v, ok := m.Get(10); ok {
		t.Errorf("Got mapping for 10 when none was inserted.  Received %d, %t\n", v, ok)
	}
	if v, ok := m.FinalLabel(20); v != 21 || !ok {
		t.Errorf("Couldn't get final mapping label from 20->6->32->21.  Got %d, %t\n", v, ok)
	}
}

func TestCounts(t *testing.T) {
	var c Counts
	if !c.Empty() {
		t.Errorf("Expected Counts to be empty")
	}
	c.Incr(7)
	c.Incr(21)
	c.Decr(21)
	c.Decr(7)
	if !c.Empty() {
		t.Errorf("Expected Counts to be empty after incr/decr cycles")
	}

	c.Incr(9)
	if v := c.Value(9); v != 1 {
		t.Errorf("Bad count.  Expected 1 got %d\n", v)
	}
	// Test thread-safety
	var wg sync.WaitGroup
	wg.Add(500)
	expected := 1
	for i := 0; i < 500; i++ {
		r := rand.Intn(3)
		if r == 0 {
			expected--
			go func() {
				c.Decr(9)
				wg.Done()
			}()
		} else {
			expected++
			go func() {
				c.Incr(9)
				wg.Done()
			}()
		}
	}
	wg.Wait()
	if v := c.Value(9); v != expected {
		t.Errorf("After concurrent, random incr/decr, got %d, expected %d\n", v, expected)
	}

}

func TestMergeCache(t *testing.T) {
	merges := []MergeTuple{
		MergeTuple{4, 1, 2, 3},
		MergeTuple{9, 10, 11, 12},
		MergeTuple{21, 100, 18, 85, 97, 45},
	}
	merges2 := []MergeTuple{
		MergeTuple{4, 9, 3, 22},
		MergeTuple{9, 5, 11, 6},
		MergeTuple{21, 44, 55, 66, 77, 88},
	}
	expectmap := map[uint64]uint64{
		1:   4,
		2:   4,
		3:   4,
		10:  9,
		11:  9,
		12:  9,
		100: 21,
		18:  21,
		85:  21,
		97:  21,
		45:  21,
	}

	iv := dvid.InstanceVersion{"foobar", 23}
	iv2 := dvid.InstanceVersion{"foobar", 24}
	for _, tuple := range merges {
		op, err := tuple.Op()
		if err != nil {
			t.Errorf("Error converting tuple %v to MergeOp: %v\n", tuple, err)
		}
		MergeCache.Add(iv, op)
	}
	for _, tuple := range merges2 {
		op, err := tuple.Op()
		if err != nil {
			t.Errorf("Error converting tuple %v to MergeOp: %v\n", tuple, err)
		}
		MergeCache.Add(iv2, op)
	}
	mapping := MergeCache.LabelMap(iv)
	for a, b := range expectmap {
		c, ok := mapping.Get(a)
		if !ok || c != b {
			t.Errorf("Expected mapping of %d -> %d, got %d (%t) instead\n", a, b, c, ok)
		}
	}
	if _, ok := mapping.Get(66); ok {
		t.Errorf("Got mapping even though none existed for this version.")
	}
}

func TestDirtyCache(t *testing.T) {
	var c DirtyCache
	iv := dvid.InstanceVersion{"foobar", 23}
	iv2 := dvid.InstanceVersion{"foobar", 24}
	if !c.Empty(iv) && !c.Empty(iv2) {
		t.Errorf("DirtyCache should be considered empty but it's not.")
	}
	c.Incr(iv, 390)
	c.Incr(iv, 84)
	c.Incr(iv, 390)
	if c.Empty(iv) {
		t.Errorf("DirtyCache should be non-empty.")
	}
	if !c.Empty(iv2) {
		t.Errorf("DirtyCache should be empty")
	}
	c.Incr(iv2, 24)
	c.Incr(iv2, 390)
	if c.IsDirty(iv, 1) {
		t.Errorf("Label is marked dirty when it's not.")
	}
	if !c.IsDirty(iv, 84) {
		t.Errorf("Label is not marked dirty when it is.")
	}
	if c.IsDirty(iv, 24) {
		t.Errorf("Label is marked dirty when it's not.")
	}
	if !c.IsDirty(iv2, 24) {
		t.Errorf("Label is not marked dirty when it is.")
	}
	if !c.IsDirty(iv, 390) {
		t.Errorf("Label is not marked dirty when it is.")
	}
	c.Decr(iv, 390)
	if !c.IsDirty(iv, 390) {
		t.Errorf("Label is not marked dirty when it is.")
	}
	c.Decr(iv, 390)
	if c.IsDirty(iv, 390) {
		t.Errorf("Label is marked dirty when it's not.")
	}
	if !c.IsDirty(iv2, 390) {
		t.Errorf("Label is not marked dirty when it is.")
	}
}
