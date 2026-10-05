package extlibs

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/object"
)

// csvWriterClass / csvReaderClass implement Python's csv.writer and
// csv.reader over any object with write()/readline() methods (StringIO,
// files). The default line terminator matches Python: \r\n.

var csvWriterClass = &object.Class{
	Name: "csv.writer",
	Methods: map[string]object.Object{
		"writerow": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return csvSimpleError("writerow() requires a row argument")
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return csvSimpleError("writerow() must be called on a csv.writer")
				}
				return csvWriteRows(ctx, self, args[1:2])
			},
			HelpText: "writerow(row) - Write one row to the underlying file",
		},
		"writerows": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return csvSimpleError("writerows() requires a rows argument")
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return csvSimpleError("writerows() must be called on a csv.writer")
				}
				rows, errObj := args[1].AsList()
				if errObj != nil {
					return errObj
				}
				return csvWriteRows(ctx, self, rows)
			},
			HelpText: "writerows(rows) - Write many rows to the underlying file",
		},
	},
}

var csvReaderClass = &object.Class{
	Name: "csv.reader",
	Methods: map[string]object.Object{
		"__iter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return args[0]
			},
			HelpText: "__iter__() - Readers are their own iterator",
		},
		"__next__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, ok := args[0].(*object.Instance)
				if !ok {
					return csvSimpleError("__next__() must be called on a csv.reader")
				}
				return csvReadNext(self)
			},
			HelpText: "__next__() - Return the next row or raise StopIteration",
		},
	},
}

func csvSimpleError(format string, a ...any) object.Object {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

// csvCellString stringifies one cell as Python's csv writer does: strings
// pass through, None becomes the empty string, and anything else is str()'d
// (ints, floats, bools, containers).
func csvCellString(cell object.Object) (string, object.Object) {
	if s, ok := cell.(*object.String); ok {
		return s.StringValue(), nil
	}
	if _, isNull := cell.(*object.Null); isNull {
		return "", nil
	}
	return cell.Inspect(), nil
}

// csvWriteRows encodes rows and writes them through the target's write
// method, so StringIO.getvalue() and real file objects both observe them.
func csvWriteRows(ctx context.Context, self *object.Instance, rows []object.Object) object.Object {
	target, _ := self.Field("_target").(*object.Instance)
	if target == nil {
		return csvSimpleError("csv writer has no target file")
	}
	writeFn, ok := target.Class.LookupMember("write")
	if !ok {
		return csvSimpleError("csv target has no write method")
	}
	writeBuiltin, isBuiltin := writeFn.(*object.Builtin)

	delimiter, _ := self.Field("_delimiter").(*object.String)
	comma := ','
	if delimiter != nil && delimiter.StringValue() != "" {
		comma = []rune(delimiter.StringValue())[0]
	}

	var buf strings.Builder
	w := csv.NewWriter(&buf)
	w.Comma = comma
	for _, rowObj := range rows {
		row, ok := rowObj.(*object.List)
		if !ok {
			return csvSimpleError("csv row must be a list")
		}
		strs := make([]string, len(row.Elements))
		for i, cell := range row.Elements {
			s, cellErr := csvCellString(cell)
			if cellErr != nil {
				return cellErr
			}
			strs[i] = s
		}
		if e := w.Write(strs); e != nil {
			return csvSimpleError("csv write: %s", e.Error())
		}
	}
	w.Flush()
	// Go's csv.Writer ends records with \n; Python's default is \r\n.
	out := strings.ReplaceAll(buf.String(), "\n", "\r\n")

	if isBuiltin && writeBuiltin.Fn != nil {
		return writeBuiltin.Fn(ctx, object.Kwargs{}, target, object.NewString(out))
	}
	return csvSimpleError("cannot call write on target")
}

// csvReadNext pulls the next line from the source's readline() and parses
// it as one CSV record. An empty line ends iteration.
func csvReadNext(self *object.Instance) object.Object {
	record, errObj := csvNextRecord(self)
	if errObj != nil {
		return errObj
	}
	elems := make([]object.Object, len(record))
	for i, cell := range record {
		elems[i] = object.NewString(cell)
	}
	return &object.List{Elements: elems}
}

// csvNextRecord reads and parses the next CSV record from the instance's
// _lines/_pos fields. It returns the StopIteration exception at end.
func csvNextRecord(self *object.Instance) ([]string, object.Object) {
	lines, _ := self.Field("_lines").(*object.List)
	pos, _ := self.Field("_pos").(*object.Integer)
	if lines == nil || pos == nil {
		return nil, csvSimpleError("csv reader has no source")
	}
	idx := int(pos.IntValue())
	if idx >= len(lines.Elements) {
		return nil, &object.Exception{Message: "StopIteration", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
	}
	line := lines.Elements[idx].Inspect()
	self.SetField("_pos", object.NewInteger(int64(idx+1)))
	if strings.TrimRight(line, "\r\n") == "" {
		return nil, &object.Exception{Message: "StopIteration", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
	}
	r := csv.NewReader(strings.NewReader(line))
	record, err := r.Read()
	if err != nil {
		return nil, &object.Exception{Message: "StopIteration", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
	}
	return record, nil
}

// csvDictReaderClass implements csv.DictReader: like csv.reader but yielding
// dicts keyed by the header row (or an explicit fieldnames list).
var csvDictReaderClass = &object.Class{
	Name: "csv.DictReader",
	Methods: map[string]object.Object{
		"__iter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return args[0]
			},
			HelpText: "__iter__() - Readers are their own iterator",
		},
		"fieldnames": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, ok := args[0].(*object.Instance)
				if !ok {
					return csvSimpleError("fieldnames must be accessed on a csv.DictReader")
				}
				// The public field (set at construction / by the header
				// read) shadows this; reaching here means direct call.
				if v, has := self.GetField("fieldnames"); has {
					return v
				}
				return &object.Null{}
			},
			HelpText: "fieldnames - The header field names",
		},
		"__next__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, ok := args[0].(*object.Instance)
				if !ok {
					return csvSimpleError("__next__() must be called on a csv.DictReader")
				}
				if err := csvDictReaderHeader(self); err != nil {
					return err
				}
				record, errObj := csvNextRecord(self)
				if errObj != nil {
					return errObj
				}
				names, _ := self.Field("_fieldnames").(*object.List)
				restval, hasRestval := self.GetField("_restval")
				d := &object.Dict{Pairs: make(map[string]object.DictPair)}
				for i, cell := range record {
					var key object.Object
					if names != nil && i < len(names.Elements) {
						key = names.Elements[i]
					} else {
						// Extra cells collect under the None key, as Python's
						// restkey=None does.
						key = &object.Null{}
					}
					ks, kerr := key.AsString()
					if kerr != nil {
						ks = key.Inspect()
					}
					d.SetByString(ks, object.NewString(cell))
				}
				if names != nil {
					for i := len(record); i < len(names.Elements); i++ {
						ks, kerr := names.Elements[i].AsString()
						if kerr != nil {
							ks = names.Elements[i].Inspect()
						}
						if hasRestval {
							d.SetByString(ks, restval)
						} else {
							d.SetByString(ks, &object.Null{})
						}
					}
				}
				return d
			},
			HelpText: "__next__() - Return the next row as a dict or raise StopIteration",
		},
	},
}

// csvDictReaderHeader consumes the header row on first use, when no explicit
// fieldnames were given, and publishes the public fieldnames field.
func csvDictReaderHeader(self *object.Instance) object.Object {
	if _, has := self.GetField("_fieldnames"); has {
		return nil
	}
	record, errObj := csvNextRecord(self)
	if errObj != nil {
		return errObj
	}
	elems := make([]object.Object, len(record))
	for i, name := range record {
		elems[i] = object.NewString(name)
	}
	names := &object.List{Elements: elems}
	self.SetField("_fieldnames", names)
	self.SetField("fieldnames", names)
	return nil
}

// csvDictWriterClass implements csv.DictWriter: writes dicts as rows ordered
// by the given fieldnames; missing keys take restval.
var csvDictWriterClass = &object.Class{
	Name: "csv.DictWriter",
	Methods: map[string]object.Object{
		"writeheader": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				self, ok := args[0].(*object.Instance)
				if !ok {
					return csvSimpleError("writeheader() must be called on a csv.DictWriter")
				}
				names, _ := self.Field("_fieldnames").(*object.List)
				if names == nil {
					return csvSimpleError("csv.DictWriter has no fieldnames")
				}
				row := &object.List{Elements: names.Elements}
				return csvWriteRows(ctx, self, []object.Object{row})
			},
			HelpText: "writeheader() - Write the header row",
		},
		"writerow": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return csvSimpleError("writerow() requires a row argument")
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return csvSimpleError("writerow() must be called on a csv.DictWriter")
				}
				row, errObj := csvDictWriterRow(self, args[1])
				if errObj != nil {
					return errObj
				}
				return csvWriteRows(ctx, self, []object.Object{row})
			},
			HelpText: "writerow(row) - Write one dict row to the underlying file",
		},
		"writerows": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return csvSimpleError("writerows() requires a rows argument")
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return csvSimpleError("writerows() must be called on a csv.DictWriter")
				}
				rows, errObj := args[1].AsList()
				if errObj != nil {
					return errObj
				}
				out := make([]object.Object, 0, len(rows))
				for _, r := range rows {
					row, errObj := csvDictWriterRow(self, r)
					if errObj != nil {
						return errObj
					}
					out = append(out, row)
				}
				return csvWriteRows(ctx, self, out)
			},
			HelpText: "writerows(rows) - Write many dict rows to the underlying file",
		},
	},
}

// csvDictWriterRow projects a dict through the writer's fieldnames into a
// list row, filling restval for missing keys.
func csvDictWriterRow(self *object.Instance, rowObj object.Object) (object.Object, object.Object) {
	names, _ := self.Field("_fieldnames").(*object.List)
	if names == nil {
		return nil, csvSimpleError("csv.DictWriter has no fieldnames")
	}
	d, ok := rowObj.(*object.Dict)
	if !ok {
		return nil, csvSimpleError("csv.DictWriter rows must be dicts")
	}
	restval, hasRestval := self.GetField("_restval")
	elems := make([]object.Object, len(names.Elements))
	for i, nameObj := range names.Elements {
		name, kerr := nameObj.AsString()
		if kerr != nil {
			name = nameObj.Inspect()
		}
		if pair, exists := d.GetByString(name); exists {
			elems[i] = pair.Value
		} else if hasRestval {
			elems[i] = restval
		} else {
			elems[i] = &object.Null{}
		}
	}
	return &object.List{Elements: elems}, nil
}
