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
			s, e := cell.AsString()
			if e != nil {
				return csvSimpleError("csv cell %d is not a string", i)
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
	lines, _ := self.Field("_lines").(*object.List)
	pos, _ := self.Field("_pos").(*object.Integer)
	if lines == nil || pos == nil {
		return csvSimpleError("csv reader has no source")
	}
	idx := int(pos.IntValue())
	if idx >= len(lines.Elements) {
		return &object.Exception{Message: "StopIteration", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
	}
	line := lines.Elements[idx].Inspect()
	self.SetField("_pos", object.NewInteger(int64(idx+1)))
	if strings.TrimRight(line, "\r\n") == "" {
		return &object.Exception{Message: "StopIteration", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
	}
	r := csv.NewReader(strings.NewReader(line))
	record, err := r.Read()
	if err != nil {
		return &object.Exception{Message: "StopIteration", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
	}
	elems := make([]object.Object, len(record))
	for i, cell := range record {
		elems[i] = object.NewString(cell)
	}
	return &object.List{Elements: elems}
}
