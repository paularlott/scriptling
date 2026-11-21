package object

import (
	"sync"
	"sync/atomic"
)

// An instShape lists the field names of an Instance in assignment order; the
// instance holds only the values, by position. Instances of a class that assign
// the same fields in the same order share one shape, reached from the class's
// root shape through transitions (one per added name), so a class's instances
// pay for their field names once rather than per instance.
//
// Shared shapes are immutable apart from their transition list, which is
// copy-on-write and published atomically, so they may be used from any
// goroutine: a Class (and so its shapes) can be shared between interpreters.
// An instance whose layout cannot be shared — too many fields, a class with
// too many layouts, or no class — gets a private shape that only it uses and
// changes in place, under the interpreter lock that guards the instance.
type instShape struct {
	names []string

	// Private shapes only: name to position.
	private bool
	index   map[string]int

	// Shared shapes only. The transitions are the shapes that extend this one
	// by one name, which is each one's last: first is the first made and rest
	// the others, replaced, never changed, under mu.
	first atomic.Pointer[instShape]
	rest  atomic.Pointer[[]*instShape]
	mu    sync.Mutex
	root  *instShape // the class's root shape; count is used only there
	count atomic.Int32
}

const (
	maxSharedShapeLen = 32 // longer layouts are private
	maxShapeEdges     = 8  // transitions per shape
	maxClassShapes    = 64 // shared shapes per class
)

// find returns the position of name, or -1.
func (s *instShape) find(name string) int {
	if s.private {
		if n, ok := s.index[name]; ok {
			return n
		}
		return -1
	}
	for n, k := range s.names {
		if k == name {
			return n
		}
	}
	return -1
}

// rootShape returns the class's empty shape, or nil for a nil class.
func (c *Class) rootShape() *instShape {
	if c == nil {
		return nil
	}
	if r := c.shapeRoot.Load(); r != nil {
		return r
	}
	r := &instShape{}
	r.root = r
	c.shapeRoot.CompareAndSwap(nil, r)
	return c.shapeRoot.Load()
}

// next returns the shared shape that extends the shared shape s by name, or
// nil when the layout is not to be shared.
func (s *instShape) next(name string) *instShape {
	n := len(s.names)
	if t := s.first.Load(); t != nil && t.names[n] == name {
		return t
	}
	if r := s.rest.Load(); r != nil {
		for _, t := range *r {
			if t.names[n] == name {
				return t
			}
		}
	}
	return s.nextSlow(name)
}

func (s *instShape) nextSlow(name string) *instShape {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.names)
	first := s.first.Load()
	if first != nil && first.names[n] == name {
		return first
	}
	var rest []*instShape
	if r := s.rest.Load(); r != nil {
		rest = *r
		for _, t := range rest {
			if t.names[n] == name {
				return t
			}
		}
	}
	if len(rest) >= maxShapeEdges-1 || n >= maxSharedShapeLen {
		return nil
	}
	if s.root.count.Add(1) > maxClassShapes {
		s.root.count.Add(-1)
		return nil
	}
	names := make([]string, n+1)
	copy(names, s.names)
	names[n] = name
	t := &instShape{names: names, root: s.root}
	if first == nil {
		s.first.Store(t)
		return t
	}
	grown := make([]*instShape, len(rest)+1)
	copy(grown, rest)
	grown[len(rest)] = t
	s.rest.Store(&grown)
	return t
}

// newPrivateShape returns a private shape holding a copy of names.
func newPrivateShape(names []string) *instShape {
	p := &instShape{
		private: true,
		names:   make([]string, len(names), len(names)+4),
		index:   make(map[string]int, len(names)+1),
	}
	copy(p.names, names)
	for n, k := range p.names {
		p.index[k] = n
	}
	return p
}

// add appends name to a private shape.
func (s *instShape) add(name string) {
	s.index[name] = len(s.names)
	s.names = append(s.names, name)
}

// remove deletes the name at position n from a private shape.
func (s *instShape) remove(n int) {
	delete(s.index, s.names[n])
	copy(s.names[n:], s.names[n+1:])
	s.names[len(s.names)-1] = ""
	s.names = s.names[:len(s.names)-1]
	for k := n; k < len(s.names); k++ {
		s.index[s.names[k]] = k
	}
}
