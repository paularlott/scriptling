package object

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"weak"
)

// Dict and set keys are strings, so an instance that hashes by a __hash__
// method cannot be keyed by its hash alone: two unequal instances may share
// a hash. Python finds a key by hash and then by ==; here equal instances
// are gathered into groups, one per run of mutually equal instances sharing
// a hash, and an instance's key names its group. Each class that defines
// __hash__ keeps the groups of the instances it hashes; an instance joins a
// group the first time it is keyed (comparing it with one member of each
// group of its hash, as Python compares a probe with the stored keys that
// share its hash) and leaves once it has been collected (swept out as the
// membership grows). Ordinary keys (str, int, ...) never come here.

// hashGroups is the grouping of one class's hashed instances.
type hashGroups struct {
	mu      sync.Mutex
	buckets map[int64][]*hashGroup
	members map[weak.Pointer[Instance]]memberRef
	version uint64
	// sweepAt is the member count at which collected instances are next
	// swept out, so the bookkeeping stays proportional to the live ones.
	sweepAt int
}

// memberRef is where an instance sits: its group, and its position in the
// group's members, so leaving is O(1) however large the group grows.
type memberRef struct {
	group *hashGroup
	idx   int
}

// hashGroup is a set of equal instances that share a hash.
type hashGroup struct {
	hash    int64
	key     string
	members []weak.Pointer[Instance]
}

var hashGroupIDs atomic.Uint64

// groups returns c's hash grouping, made on first use.
func (c *Class) groups() *hashGroups {
	if g := c.hashGroups.Load(); g != nil {
		return g
	}
	g := &hashGroups{buckets: make(map[int64][]*hashGroup), members: make(map[weak.Pointer[Instance]]memberRef)}
	if c.hashGroups.CompareAndSwap(nil, g) {
		return g
	}
	return c.hashGroups.Load()
}

// InstanceHashMethod returns the __hash__ an instance of c hashes by.
// hashable is false when c's instances are unhashable: __hash__ is None, or
// the class defines __eq__ without __hash__ (Python sets __hash__ to None
// for it). fn is nil for an instance that hashes by identity.
func InstanceHashMethod(c *Class) (fn Object, hashable bool) {
	if h, ok := c.Methods["__hash__"]; ok {
		if _, none := h.(*Null); none {
			return nil, false
		}
		return h, true
	}
	if _, hasEq := c.Methods["__eq__"]; hasEq {
		return nil, false
	}
	return nil, true
}

// hashOwner is the class that first defines fn as its __hash__ along c's
// bases: instances of subclasses that inherit one __hash__ are grouped
// together, so Sub(1) and Base(1) find each other as keys.
func hashOwner(c *Class, fn Object) *Class {
	for c.BaseClass != nil && c.BaseClass.Methods["__hash__"] == fn {
		c = c.BaseClass
	}
	return c
}

// InstanceKey returns the dict key of inst, whose class's __hash__ method fn
// returned n. eq reports whether an instance already keyed under n equals
// inst (Python compares the stored key with the probe); it is called with no
// lock held and its error is returned. Equal instances with the same hash
// get the same key, unequal ones different keys.
func InstanceKey(inst *Instance, fn Object, n int64, eq func(stored *Instance) (bool, Object)) (string, Object) {
	g := hashOwner(inst.Class, fn).groups()
	wp := weak.Make(inst)
	type candidate struct {
		group *hashGroup
		rep   *Instance
	}
	var cands []candidate
	for attempt := 0; ; attempt++ {
		g.mu.Lock()
		prev, known := g.members[wp]
		if known && prev.group.hash == n {
			key := prev.group.key
			g.mu.Unlock()
			return key, nil
		}
		cands = cands[:0]
		for _, grp := range g.buckets[n] {
			for _, m := range grp.members {
				if rep := m.Value(); rep != nil {
					cands = append(cands, candidate{grp, rep})
					break
				}
			}
		}
		version := g.version
		g.mu.Unlock()

		var found *hashGroup
		for _, c := range cands {
			if c.rep == inst {
				found = c.group
				break
			}
			same, errObj := eq(c.rep)
			if errObj != nil {
				return "", errObj
			}
			if same {
				found = c.group
				break
			}
		}

		g.mu.Lock()
		if g.version != version && attempt < 4 {
			// The groups changed while the comparisons ran: look again.
			g.mu.Unlock()
			continue
		}
		g.version++
		if found != nil && !g.inBucket(found) {
			// Its last member was collected while the comparisons ran.
			g.buckets[n] = append(g.buckets[n], found)
		}
		if found == nil {
			found = &hashGroup{hash: n, key: "h:" + strconv.FormatInt(n, 10) + "#" + strconv.FormatUint(hashGroupIDs.Add(1), 10)}
			g.buckets[n] = append(g.buckets[n], found)
		}
		prev, known = g.members[wp]
		if known {
			g.leave(prev, wp)
		} else if len(g.members) >= g.sweepAt {
			g.sweep()
		}
		found.members = append(found.members, wp)
		g.members[wp] = memberRef{found, len(found.members) - 1}
		g.mu.Unlock()
		return found.key, nil
	}
}

// inBucket reports whether grp is still one of its hash's groups. g.mu is
// held.
func (g *hashGroups) inBucket(grp *hashGroup) bool {
	for _, other := range g.buckets[grp.hash] {
		if other == grp {
			return true
		}
	}
	return false
}

// sweep drops the members that have been collected, and sets when to
// sweep next. g.mu is held.
func (g *hashGroups) sweep() {
	for wp, ref := range g.members {
		if wp.Value() == nil {
			g.leave(ref, wp)
			delete(g.members, wp)
		}
	}
	g.sweepAt = max(2*len(g.members), 1024)
}

// leave takes wp out of its group, dropping the group once it is empty.
// The last member moves into wp's place. g.mu is held.
func (g *hashGroups) leave(ref memberRef, wp weak.Pointer[Instance]) {
	grp := ref.group
	last := len(grp.members) - 1
	if ref.idx != last {
		moved := grp.members[last]
		grp.members[ref.idx] = moved
		g.members[moved] = memberRef{grp, ref.idx}
	}
	grp.members[last] = weak.Pointer[Instance]{}
	grp.members = grp.members[:last]
	if len(grp.members) > 0 {
		return
	}
	bucket := g.buckets[grp.hash]
	for i, other := range bucket {
		if other == grp {
			bucket = append(bucket[:i], bucket[i+1:]...)
			break
		}
	}
	if len(bucket) == 0 {
		delete(g.buckets, grp.hash)
	} else {
		g.buckets[grp.hash] = bucket
	}
}

// hashKeyHook is the evaluator's dict key function, installed for libraries
// that cannot import it.
var hashKeyHook func(ctx context.Context, obj Object) (string, Object)

// SetHashKeyHook registers the evaluator's dict key function.
func SetHashKeyHook(fn func(ctx context.Context, obj Object) (string, Object)) {
	hashKeyHook = fn
}

// HashKey returns obj's dict and set key as a script sees it: an instance
// (alone or inside a tuple) keyed through its __hash__ and __eq__, so equal
// values share a key. The second return is the error to propagate: a
// TypeError for an unhashable value, or what __hash__ or __eq__ raised.
func HashKey(ctx context.Context, obj Object) (string, Object) {
	switch obj.(type) {
	case *String, *Integer:
		return DictKey(obj), nil
	}
	if hashKeyHook != nil && ctx != nil {
		return hashKeyHook(ctx, obj)
	}
	if !IsHashable(obj) {
		return "", unhashableError(obj)
	}
	return DictKey(obj), nil
}
