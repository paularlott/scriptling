package stdlib

import "github.com/paularlott/scriptling/object"

// The value types below are immutable, as in Python: assigning an
// attribute of one (d.year = 2021, or a new attribute) is an
// AttributeError rather than a change that the value's own state would not
// follow. The evaluator reads the "readonly" __frozen__ marker. This file
// sorts after the files whose init functions build the classes.
func init() {
	for _, cls := range []*object.Class{DateClass, DatetimeClass, TimeClass, TimedeltaClass, DecimalClass} {
		if cls != nil {
			cls.Methods["__frozen__"] = object.NewString("readonly")
		}
	}
}
