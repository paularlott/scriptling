package object

// Iterator type names: what type() reports for an iterator ("range", "map").
// An iterator that does not set its own reports "iterator".
var (
	IteratorTypeName  = NewString("iterator")
	rangeTypeName     = NewString("range")
	zipTypeName       = NewString("zip")
	enumerateTypeName = NewString("enumerate")
)

// TypeName returns the iterator's type name.
func (it *Iterator) TypeName() *String {
	if it.typeName != nil {
		return it.typeName
	}
	return IteratorTypeName
}

// SetTypeName sets the iterator's type name, Python's name for it ("map",
// "list_iterator"), and returns the iterator. Share one string per name
// rather than allocating one per iterator.
func (it *Iterator) SetTypeName(name *String) *Iterator {
	it.typeName = name
	return it
}
