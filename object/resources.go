package object

import (
	"slices"
	"sync"
)

// resourceSet is what scripts in one interpreter opened and have not closed
// themselves: clients, connections, child processes, file handles. It hangs
// off the interpreter's lock domain, so every environment of one interpreter
// shares it, as do the script libraries it imports (ShareResources); a
// background task, which runs in an environment of its own, has its own.
type resourceSet struct {
	mu   sync.Mutex
	next uint64
	open map[uint64]func()
}

// TrackResource registers close as something a script running in e's
// interpreter opened. CloseResources runs it, unless untrack is called first,
// as a library does when the script closes the resource itself. close must be
// safe to call more than once and from any goroutine.
//
// The interpreter keeps close until then: it must not hold on to the script
// object that owns the resource, or to an environment, or that object could
// never be collected.
func (e *Environment) TrackResource(close func()) (untrack func()) {
	rs := e.resourceSet()
	if rs == nil {
		return func() {}
	}
	rs.mu.Lock()
	if rs.open == nil {
		rs.open = make(map[uint64]func())
	}
	rs.next++
	id := rs.next
	rs.open[id] = close
	rs.mu.Unlock()
	return func() {
		rs.mu.Lock()
		delete(rs.open, id)
		rs.mu.Unlock()
	}
}

// CloseResources closes everything scripts in e's interpreter opened and have
// not closed, newest first. A close that panics does not stop the rest.
func (e *Environment) CloseResources() {
	rs := e.resourceSet()
	if rs == nil {
		return
	}
	rs.mu.Lock()
	open := rs.open
	rs.open = nil
	rs.mu.Unlock()
	ids := make([]uint64, 0, len(open))
	for id := range open {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range slices.Backward(ids) {
		func() {
			defer func() { _ = recover() }()
			open[id]()
		}()
	}
}

// ShareResources makes what scripts in e's lock domain open belong to from's
// interpreter, as a script library's environment does with the interpreter
// that imports it. Call it before anything runs in e.
func (e *Environment) ShareResources(from *Environment) {
	if rs := from.resourceSet(); rs != nil && e != nil && e.root != nil && e.root.gil != nil {
		e.root.gil.resources = rs
	}
}

func (e *Environment) resourceSet() *resourceSet {
	if e == nil || e.root == nil || e.root.gil == nil {
		return nil
	}
	return e.root.gil.resources
}
