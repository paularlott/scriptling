package stdlib

import (
	"context"
	"strings"
	"unsafe"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// stringIOData holds the mutable state for a StringIO instance.
type stringIOData struct {
	buf    strings.Builder
	pos    int
	closed bool
}

// stringIOHolder wraps *stringIOData as an object.Object so it can be stored
// in an Instance's Fields map without reflection or unsafe pointers.
type stringIOHolder struct{ data *stringIOData }

func (h *stringIOHolder) Type() object.ObjectType { return object.BUILTIN_OBJ }
func (h *stringIOHolder) Inspect() string         { return "<StringIO data>" }
func (h *stringIOHolder) AsString() (string, object.Object) {
	return "", &object.Error{Message: object.ErrMustBeString}
}
func (h *stringIOHolder) AsInt() (int64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeInteger}
}
func (h *stringIOHolder) AsFloat() (float64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeNumber}
}
func (h *stringIOHolder) AsBool() (bool, object.Object) { return true, nil }
func (h *stringIOHolder) AsList() ([]object.Object, object.Object) {
	return nil, &object.Error{Message: object.ErrMustBeList}
}
func (h *stringIOHolder) AsDict() (map[string]object.Object, object.Object) {
	return nil, &object.Error{Message: object.ErrMustBeDict}
}
func (h *stringIOHolder) CoerceString() (string, object.Object) { return h.Inspect(), nil }
func (h *stringIOHolder) CoerceInt() (int64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeInteger}
}
func (h *stringIOHolder) CoerceFloat() (float64, object.Object) {
	return 0, &object.Error{Message: object.ErrMustBeNumber}
}

const sioKey = "__sio__"

func sioGet(inst *object.Instance) (*stringIOData, bool) {
	h, ok := inst.GetField(sioKey)
	if !ok {
		return nil, false
	}
	holder, ok := h.(*stringIOHolder)
	if !ok {
		return nil, false
	}
	return holder.data, true
}

func sioSelf(args []object.Object, method string) (*object.Instance, *stringIOData, object.Object) {
	if len(args) < 1 {
		return nil, nil, errors.NewError("%s() requires self", method)
	}
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return nil, nil, errors.NewError("%s(): self must be a StringIO instance", method)
	}
	data, ok := sioGet(inst)
	if !ok {
		return nil, nil, errors.NewError("%s(): invalid StringIO instance", method)
	}
	if data.closed {
		return nil, nil, errors.NewError("I/O operation on closed file")
	}
	return inst, data, nil
}

var stringIOClass = &object.Class{
	Name: "StringIO",
	Methods: map[string]object.Object{
		"__init__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("StringIO.__init__ requires self")
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("StringIO.__init__: self must be an instance")
				}
				data := &stringIOData{}
				if len(args) >= 2 {
					s, err := args[1].AsString()
					if err != nil {
						return errors.NewTypeError("STRING", args[1].Type().String())
					}
					if errObj := data.write(s); errObj != nil {
						return errObj
					}
				}
				inst.SetField(sioKey, &stringIOHolder{data: data})
				return &object.Null{}
			},
		},
		"write": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "write")
				if errObj != nil {
					return errObj
				}
				if len(args) < 2 {
					return errors.NewError("write() requires a string argument")
				}
				s, err := args[1].AsString()
				if err != nil {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				if errObj := data.write(s); errObj != nil {
					return errObj
				}
				return object.NewInteger(int64(len(s)))
			},
			HelpText: `write(s) - Write string s to buffer; returns number of characters written`,
		},
		"getvalue": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "getvalue")
				if errObj != nil {
					return errObj
				}
				return object.NewString(data.buf.String())
			},
			HelpText: `getvalue() - Return the entire buffer contents as a string`,
		},
		"read": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "read")
				if errObj != nil {
					return errObj
				}
				content := data.buf.String()
				if data.pos >= len(content) {
					return object.NewString("")
				}
				if len(args) >= 2 {
					n, err := args[1].AsInt()
					if err != nil {
						return errors.NewTypeError("INTEGER", args[1].Type().String())
					}
					end := data.pos + int(n)
					if end > len(content) {
						end = len(content)
					}
					result := content[data.pos:end]
					data.pos = end
					return object.NewString(result)
				}
				result := content[data.pos:]
				data.pos = len(content)
				return object.NewString(result)
			},
			HelpText: `read([n]) - Read up to n characters from current position; reads all if n omitted`,
		},
		"readline": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "readline")
				if errObj != nil {
					return errObj
				}
				content := data.buf.String()
				if data.pos >= len(content) {
					return object.NewString("")
				}
				rest := content[data.pos:]
				idx := strings.Index(rest, "\n")
				if idx == -1 {
					data.pos = len(content)
					return object.NewString(rest)
				}
				line := rest[:idx+1]
				data.pos += idx + 1
				return object.NewString(line)
			},
			HelpText: `readline() - Read one line (including newline) from current position`,
		},
		"readlines": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "readlines")
				if errObj != nil {
					return errObj
				}
				content := data.buf.String()
				if data.pos >= len(content) {
					return &object.List{Elements: []object.Object{}}
				}
				rest := content[data.pos:]
				lines := strings.SplitAfter(rest, "\n")
				if len(lines) > 0 && lines[len(lines)-1] == "" {
					lines = lines[:len(lines)-1]
				}
				elems := make([]object.Object, len(lines))
				for i, ln := range lines {
					elems[i] = object.NewString(ln)
				}
				data.pos = len(content)
				return &object.List{Elements: elems}
			},
			HelpText: `readlines() - Read all remaining lines (including newlines) as a list`,
		},
		"seek": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "seek")
				if errObj != nil {
					return errObj
				}
				if len(args) < 2 {
					return errors.NewError("seek() requires a position argument")
				}
				pos, err := args[1].AsInt()
				if err != nil {
					return errors.NewTypeError("INTEGER", args[1].Type().String())
				}
				content := data.buf.String()
				if pos < 0 {
					pos = 0
				}
				if int(pos) > len(content) {
					pos = int64(len(content))
				}
				data.pos = int(pos)
				return object.NewInteger(pos)
			},
			HelpText: `seek(pos) - Set the read position to pos`,
		},
		"tell": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "tell")
				if errObj != nil {
					return errObj
				}
				return object.NewInteger(int64(data.pos))
			},
			HelpText: `tell() - Return the current read position`,
		},
		"truncate": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "truncate")
				if errObj != nil {
					return errObj
				}
				content := data.buf.String()
				pos := int64(data.pos)
				if len(args) >= 2 {
					var err object.Object
					pos, err = args[1].AsInt()
					if err != nil {
						return errors.NewTypeError("INTEGER", args[1].Type().String())
					}
				}
				if pos < 0 {
					pos = 0
				}
				if int(pos) > len(content) {
					pos = int64(len(content))
				}
				// Reset drops the old backing array (its charge is credited back
				// when it is collected) and the kept prefix gets a new one.
				data.buf.Reset()
				if errObj := data.write(content[:pos]); errObj != nil {
					return errObj
				}
				if data.pos > int(pos) {
					data.pos = int(pos)
				}
				return object.NewInteger(pos)
			},
			HelpText: `truncate([pos]) - Truncate buffer to pos characters (default: current position)`,
		},
		"close": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("close() requires self")
				}
				inst, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("close(): self must be a StringIO instance")
				}
				data, ok := sioGet(inst)
				if !ok {
					return errors.NewError("close(): invalid StringIO instance")
				}
				data.closed = true
				return &object.Null{}
			},
			HelpText: `close() - Close the buffer`,
		},
		"__enter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewError("__enter__() requires self")
				}
				return args[0]
			},
		},
		"__exit__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 {
					return object.NewBoolean(false)
				}
				inst, ok := args[0].(*object.Instance)
				if ok {
					if data, ok := sioGet(inst); ok {
						data.closed = true
					}
				}
				return object.NewBoolean(false)
			},
		},
	},
}

// stringIONew creates a new StringIO instance (called as io.StringIO(...)).
func stringIONew(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	inst := object.NewInstanceWithFields(stringIOClass, nil)
	data := &stringIOData{}
	if len(args) >= 1 {
		s, err := args[0].AsString()
		if err != nil {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		if errObj := data.write(s); errObj != nil {
			return errObj
		}
	}
	inst.SetField(sioKey, &stringIOHolder{data: data})
	return inst
}

// GetStringIOWriter returns an io.Writer backed by a StringIO instance,
// for use with print(file=buf) in the evaluator.
func GetStringIOWriter(inst *object.Instance) (*StringIOWriter, bool) {
	data, ok := sioGet(inst)
	if !ok {
		return nil, false
	}
	return &StringIOWriter{data: data}, true
}

// StringIOWriter implements io.Writer for a StringIO instance.
type StringIOWriter struct{ data *stringIOData }

func (w *StringIOWriter) Write(p []byte) (int, error) {
	if w.data.closed {
		return 0, nil
	}
	if errObj := w.data.writeBytes(p); errObj != nil {
		if e, ok := errObj.(*object.Error); ok && strings.HasPrefix(e.Message, object.ErrMemoryLimitExceeded.Error()) {
			return 0, object.ErrOutputNoRoom
		}
		return 0, object.ErrOutputLimitExceeded
	}
	return len(p), nil
}

// write appends s to the buffer within the interpreter's limits.
func (d *stringIOData) write(s string) object.Object {
	return d.grow(len(s), func() { d.buf.WriteString(s) })
}

// writeBytes appends p to the buffer within the interpreter's limits.
func (d *stringIOData) writeBytes(p []byte) object.Object {
	return d.grow(len(p), func() { d.buf.Write(p) })
}

// grow checks that n more bytes keep the buffer within the interpreter's
// limits, runs the append, and reports a new backing array to the memory
// accounting. The charge belongs to the backing array itself, so it is
// credited back once a reset or a later growth lets the old array go.
func (d *stringIOData) grow(n int, appendFn func()) object.Object {
	size := d.buf.Len() + n
	if errObj := object.CheckAllocFor("StringIO buffer", int64(size)); errObj != nil {
		return errObj
	}
	oldCap := d.buf.Cap()
	if size > oldCap {
		if errObj := object.CheckRoom("StringIO buffer", object.GrowthRoom(oldCap, size)); errObj != nil {
			return errObj
		}
	}
	appendFn()
	if d.buf.Cap() != oldCap && d.buf.Len() > 0 {
		// strings.Builder.String shares the builder's array (no copy), so the
		// charge is attached to the new backing array itself.
		object.TrackAlloc(unsafe.StringData(d.buf.String()), int64(d.buf.Cap()))
	}
	return nil
}

var IOLibrary = object.NewLibrary(IOLibraryName, map[string]*object.Builtin{
	"StringIO": {
		Fn: stringIONew,
		HelpText: `StringIO([initial_value=""]) - In-memory string buffer

Creates a StringIO object that behaves like a file opened for text I/O.
Optional initial_value pre-fills the buffer.

Methods: write(s), getvalue(), read([n]), readline(), seek(pos), tell(), truncate([pos]), close()
Supports the with statement: with io.StringIO() as buf: ...`,
	},
}, nil, "In-memory I/O streams")
