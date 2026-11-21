package object

import (
	"iter"
	"sort"
	"unsafe"
)

// Dict is an insertion-ordered dictionary, as Python's dict.
//
// Entries live in one slice in insertion order. A small dict is searched
// linearly, which is faster than hashing for a handful of keys and needs no
// second allocation; once it outgrows dictSmallMax an index map from
// canonical key to position takes over. Deleting leaves a hole that is
// skipped on reads and swept when holes outnumber live entries, so delete is
// O(1) amortized whichever end (or middle) it happens at.
//
// The first entry of entries[head:] and the last entry of entries are always
// live: Delete and MoveToEnd trim holes at both ends as they make them.
type Dict struct {
	entries []dictEntry
	// index maps a canonical key to its position in entries; nil while the
	// dict is small.
	index map[string]int32
	head  int32 // entries[:head] is unused (deleted from the front, or a gap for move-to-front)
	live  int32 // number of live entries
	// extra holds the rarely used attachments (default factory, owning
	// instance) behind one pointer, so an ordinary dict pays 8 bytes for them.
	extra *dictExtra
	// Module is the import name when this dict is a library module
	// ("math", "scriptling.runtime.kv"), empty for an ordinary dict.
	// Modules display as <module 'math'> and expose only their members.
	Module string
}

// DictPair is one key and its value.
type DictPair struct {
	Key   Object
	Value Object
}

// dictEntry is one slot of a dict: the canonical (hash) key and the pair. A
// nil Key marks a deleted entry.
type dictEntry struct {
	canonical string
	DictPair
}

// dictSmallMax is the largest number of slots searched linearly before the
// index map is built.
const dictSmallMax = 8

// dictEntryWeight is the estimated memory per dict entry charged against an
// interpreter's memory limit: the entry plus its share of the index map.
const dictEntryWeight = int64(unsafe.Sizeof(dictEntry{})) + 40

// DictEntryWeight is the estimated memory per dict entry, for checking a
// dict's size before it is presized.
const DictEntryWeight = dictEntryWeight

// NewDict returns an empty, insertion-ordered dict.
func NewDict() *Dict {
	return &Dict{}
}

// NewDictSized returns an empty, insertion-ordered dict with room for n pairs.
func NewDictSized(n int) *Dict {
	d := &Dict{}
	if n <= 0 {
		return d
	}
	d.entries = make([]dictEntry, 0, n)
	if n > dictSmallMax {
		d.index = make(map[string]int32, n)
	}
	if accountingOn.Load() {
		// Presized storage is held from the start; Store only charges when
		// the entries grow past it.
		trackAlloc(&d.entries[:1][0], int64(n)*dictEntryWeight)
	}
	return d
}

// Len returns the number of pairs.
func (d *Dict) Len() int { return int(d.live) }

// find returns the position in entries of the live entry for a canonical
// key, or -1.
func (d *Dict) find(canonical string) int {
	if d.index != nil {
		if pos, ok := d.index[canonical]; ok {
			return int(pos)
		}
		return -1
	}
	for i := int(d.head); i < len(d.entries); i++ {
		if e := &d.entries[i]; e.Key != nil && e.canonical == canonical {
			return i
		}
	}
	return -1
}

// Lookup returns the pair stored under a canonical key (DictKey, or
// DictStringKey for a string).
func (d *Dict) Lookup(canonical string) (DictPair, bool) {
	if pos := d.find(canonical); pos >= 0 {
		return d.entries[pos].DictPair, true
	}
	return DictPair{}, false
}

// Has reports whether a canonical key is present.
func (d *Dict) Has(canonical string) bool { return d.find(canonical) >= 0 }

// Store inserts or updates key → value, extending the insertion order for
// new keys (Python dict semantics: an existing key keeps its position and its
// original key object). canonical must be the dict's hash key for key
// (DictKey, or evalHashKeyChecked's result).
func (d *Dict) Store(canonical string, key Object, value Object) {
	if pos := d.find(canonical); pos >= 0 {
		e := &d.entries[pos]
		e.Value = value
		d.writeThrough(e.Key, value)
		return
	}
	d.push(dictEntry{canonical: canonical, DictPair: DictPair{Key: key, Value: value}})
	d.writeThrough(key, value)
}

// push appends a new live entry, building the index when the dict stops
// being small.
func (d *Dict) push(e dictEntry) {
	if d.index == nil && len(d.entries)-int(d.head) >= dictSmallMax {
		if d.holes() > 0 {
			d.compact() // make room without leaving linear search
		}
		if len(d.entries)-int(d.head) >= dictSmallMax {
			d.buildIndex(2 * dictSmallMax)
		}
	}
	oldCap := cap(d.entries)
	d.entries = append(d.entries, e)
	d.live++
	if d.index != nil {
		d.index[e.canonical] = int32(len(d.entries) - 1)
	}
	if cap(d.entries) != oldCap && accountingOn.Load() {
		// The index map grows with the entries, so their slots also stand in
		// for its buckets, which have no pointer to attach a charge to.
		trackAlloc(&d.entries[0], int64(cap(d.entries))*dictEntryWeight)
	}
}

// buildIndex (re)creates the index map from the live entries.
func (d *Dict) buildIndex(sizeHint int) {
	d.index = make(map[string]int32, max(sizeHint, int(d.live)))
	for i := int(d.head); i < len(d.entries); i++ {
		if e := &d.entries[i]; e.Key != nil {
			d.index[e.canonical] = int32(i)
		}
	}
}

// holes is the number of deleted entries inside entries[head:].
func (d *Dict) holes() int { return len(d.entries) - int(d.head) - int(d.live) }

// writeThrough applies a Store to the owning instance (an obj.__dict__ view).
// Only string keys name attributes; others stay in the view alone.
func (d *Dict) writeThrough(key, value Object) {
	if d.extra == nil || d.extra.owner == nil {
		return
	}
	if name, ok := key.(*String); ok {
		d.extra.owner.SetField(name.value, value)
		d.extra.owner.InvalidateBoundMethod(name.value)
	}
}

// StoreFrom copies every pair of other into d (dict.update), in other's
// insertion order; later stores override earlier keys as in Python.
func (d *Dict) StoreFrom(other *Dict) {
	if other == d {
		return
	}
	for _, e := range other.entries[other.head:] {
		if e.Key != nil {
			d.Store(e.canonical, e.Key, e.Value)
		}
	}
}

// Delete removes a key. O(1) amortized: the entry becomes a hole that reads
// skip (a delete at either end trims it at once), and compact() sweeps holes
// once they outnumber live entries. Deleting an absent key is a no-op.
func (d *Dict) Delete(canonical string) {
	pos := d.find(canonical)
	if pos < 0 {
		return
	}
	key := d.entries[pos].Key
	d.remove(pos)
	if d.extra != nil && d.extra.owner != nil {
		if name, ok := key.(*String); ok {
			d.extra.owner.DeleteField(name.value)
			d.extra.owner.InvalidateBoundMethod(name.value)
		}
	}
}

// remove turns the entry at pos into a hole and restores the invariant that
// both ends of entries[head:] are live.
func (d *Dict) remove(pos int) {
	if d.index != nil {
		delete(d.index, d.entries[pos].canonical)
	}
	d.entries[pos] = dictEntry{}
	d.live--
	if d.live == 0 {
		d.entries, d.head = d.entries[:0], 0 // emptied: reuse the slice
		return
	}
	// Newest key (popitem, LIFO): drop it and any holes it hid.
	for len(d.entries) > int(d.head) && d.entries[len(d.entries)-1].Key == nil {
		d.entries = d.entries[:len(d.entries)-1]
	}
	// Oldest key (FIFO / LRU eviction): advance past it and any holes behind it.
	for d.entries[d.head].Key == nil {
		d.head++
	}
	if holes := d.holes(); (holes > 32 && holes > int(d.live)) || (d.head > 32 && int(d.head) > len(d.entries)/2) {
		d.compact()
	}
}

// compact rebuilds entries from the live ones, dropping the unused prefix and
// every hole.
func (d *Dict) compact() {
	kept := make([]dictEntry, 0, int(d.live)+int(d.live)/4+1)
	for _, e := range d.entries[d.head:] {
		if e.Key != nil {
			kept = append(kept, e)
		}
	}
	d.entries, d.head = kept, 0
	if d.index != nil {
		d.buildIndex(0)
	}
	if accountingOn.Load() && cap(kept) > 0 {
		trackAlloc(&kept[:1][0], int64(cap(kept))*dictEntryWeight)
	}
}

// Clear removes every entry.
func (d *Dict) Clear() {
	if d.extra != nil && d.extra.owner != nil {
		for _, e := range d.entries[d.head:] {
			if name, ok := e.Key.(*String); ok {
				d.extra.owner.DeleteField(name.value)
				d.extra.owner.InvalidateBoundMethod(name.value)
			}
		}
	}
	d.entries, d.index, d.head, d.live = nil, nil, 0, 0
}

// All iterates over the pairs in insertion order, with each one's canonical
// key. Storing to or deleting from the dict inside the loop is safe: a pair
// deleted before the loop reaches it is not produced, and pairs added during
// the loop may or may not be.
func (d *Dict) All() iter.Seq2[string, DictPair] {
	return func(yield func(string, DictPair) bool) {
		entries := d.entries
		for i := int(d.head); i < len(entries); i++ {
			e := entries[i]
			if e.Key == nil {
				continue
			}
			// The loop body may have compacted or cleared the dict, leaving
			// this snapshot behind: check such an entry is still present.
			if unsafe.SliceData(entries) != unsafe.SliceData(d.entries) && !d.Has(e.canonical) {
				continue
			}
			if !yield(e.canonical, e.DictPair) {
				return
			}
		}
	}
}

// OrderedKeys returns the canonical keys in insertion order. Mutating the
// dict while iterating the result is safe: it is a snapshot.
func (d *Dict) OrderedKeys() []string {
	keys := make([]string, 0, d.live)
	for _, e := range d.entries[d.head:] {
		if e.Key != nil {
			keys = append(keys, e.canonical)
		}
	}
	return keys
}

// OrderedPairs returns a snapshot of the pairs in insertion order.
func (d *Dict) OrderedPairs() []DictPair {
	pairs := make([]DictPair, 0, d.live)
	for _, e := range d.entries[d.head:] {
		if e.Key != nil {
			pairs = append(pairs, e.DictPair)
		}
	}
	return pairs
}

// SortKeys reorders the dict by canonical key.
func (d *Dict) SortKeys() {
	if d.holes() > 0 || d.head > 0 {
		d.compact()
	}
	sort.Slice(d.entries, func(i, j int) bool { return d.entries[i].canonical < d.entries[j].canonical })
	if d.index != nil {
		d.buildIndex(0)
	}
}

// MoveToEnd repositions an existing key at the end (last) or the front of the
// insertion order, as OrderedDict.move_to_end does, and reports whether the
// key exists. O(1) amortized.
func (d *Dict) MoveToEnd(canonical string, last bool) bool {
	pos := d.find(canonical)
	if pos < 0 {
		return false
	}
	if (last && pos == len(d.entries)-1) || (!last && pos == int(d.head)) {
		return true // already there
	}
	e := d.entries[pos]
	d.remove(pos)
	if last {
		d.push(e)
		return true
	}
	if d.head == 0 {
		// Open a gap in front so repeated moves-to-front stay O(1) amortized.
		gap := len(d.entries)/2 + 8
		grown := make([]dictEntry, gap+len(d.entries), gap+len(d.entries)+len(d.entries)/2)
		copy(grown[gap:], d.entries)
		d.entries, d.head = grown, int32(gap)
		if d.index != nil {
			d.buildIndex(0)
		}
		if accountingOn.Load() {
			trackAlloc(&grown[0], int64(cap(grown))*dictEntryWeight)
		}
	}
	d.head--
	d.entries[d.head] = e
	d.live++
	if d.index != nil {
		d.index[canonical] = d.head
	} else if len(d.entries)-int(d.head) > dictSmallMax {
		d.buildIndex(2 * dictSmallMax)
	}
	return true
}

// FirstInserted returns the oldest pair, with its canonical key — what
// popitem(last=False) removes.
func (d *Dict) FirstInserted() (string, DictPair, bool) {
	if d.live == 0 {
		return "", DictPair{}, false
	}
	e := d.entries[d.head]
	return e.canonical, e.DictPair, true
}

// LastInserted returns the most recently inserted pair, with its canonical
// key — the pair Python's popitem() removes.
func (d *Dict) LastInserted() (string, DictPair, bool) {
	if d.live == 0 {
		return "", DictPair{}, false
	}
	e := d.entries[len(d.entries)-1]
	return e.canonical, e.DictPair, true
}
