package stdlib

import (
	"context"
	"strings"

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

func (h *stringIOHolder) Type() object.ObjectType                        { return object.BUILTIN_OBJ }
func (h *stringIOHolder) Inspect() string                                { return "<StringIO data>" }
func (h *stringIOHolder) AsString() (string, object.Object)              { return "", &object.Error{Message: object.ErrMustBeString} }
func (h *stringIOHolder) AsInt() (int64, object.Object)                  { return 0, &object.Error{Message: object.ErrMustBeInteger} }
func (h *stringIOHolder) AsFloat() (float64, object.Object)              { return 0, &object.Error{Message: object.ErrMustBeNumber} }
func (h *stringIOHolder) AsBool() (bool, object.Object)                  { return true, nil }
func (h *stringIOHolder) AsList() ([]object.Object, object.Object)       { return nil, &object.Error{Message: object.ErrMustBeList} }
func (h *stringIOHolder) AsDict() (map[string]object.Object, object.Object) { return nil, &object.Error{Message: object.ErrMustBeDict} }
func (h *stringIOHolder) CoerceString() (string, object.Object)          { return h.Inspect(), nil }
func (h *stringIOHolder) CoerceInt() (int64, object.Object)              { return 0, &object.Error{Message: object.ErrMustBeInteger} }
func (h *stringIOHolder) CoerceFloat() (float64, object.Object)          { return 0, &object.Error{Message: object.ErrMustBeNumber} }

const sioKey = "__sio__"

func sioGet(inst *object.Instance) (*stringIOData, bool) {
	h, ok := inst.Fields[sioKey]
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
					data.buf.WriteString(s)
				}
				inst.Fields[sioKey] = &stringIOHolder{data: data}
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
				n, _ := data.buf.WriteString(s)
				return object.NewInteger(int64(n))
			},
			HelpText: `write(s) - Write string s to buffer; returns number of characters written`,
		},
		"getvalue": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, data, errObj := sioSelf(args, "getvalue")
				if errObj != nil {
					return errObj
				}
				return &object.String{Value: data.buf.String()}
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
					return &object.String{Value: ""}
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
					return &object.String{Value: result}
				}
				result := content[data.pos:]
				data.pos = len(content)
				return &object.String{Value: result}
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
					return &object.String{Value: ""}
				}
				rest := content[data.pos:]
				idx := strings.Index(rest, "\n")
				if idx == -1 {
					data.pos = len(content)
					return &object.String{Value: rest}
				}
				line := rest[:idx+1]
				data.pos += idx + 1
				return &object.String{Value: line}
			},
			HelpText: `readline() - Read one line (including newline) from current position`,
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
				data.buf.Reset()
				data.buf.WriteString(content[:pos])
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
					return &object.Boolean{Value: false}
				}
				inst, ok := args[0].(*object.Instance)
				if ok {
					if data, ok := sioGet(inst); ok {
						data.closed = true
					}
				}
				return &object.Boolean{Value: false}
			},
		},
	},
}

// stringIONew creates a new StringIO instance (called as io.StringIO(...)).
func stringIONew(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	inst := &object.Instance{
		Class:  stringIOClass,
		Fields: map[string]object.Object{},
	}
	data := &stringIOData{}
	if len(args) >= 1 {
		s, err := args[0].AsString()
		if err != nil {
			return errors.NewTypeError("STRING", args[0].Type().String())
		}
		data.buf.WriteString(s)
	}
	inst.Fields[sioKey] = &stringIOHolder{data: data}
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
	return w.data.buf.Write(p)
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
