// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"bytes"
	"context"
	"encoding/csv"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// RegisterCsvLibrary registers the csv library. It is importable as both
// "csv" (Python-compatible) and "scriptling.csv".
func RegisterCsvLibrary(registrar object.LibraryRegistrar) {
	registrar.RegisterLibrary(NewCsvLibrary())
	registrar.RegisterLibrary(newAliasedLibrary(NewCsvLibrary(), "csv"))
}

func NewCsvLibrary() *object.Library {
	return object.NewLibrary(CsvLibraryName, map[string]*object.Builtin{
		"writer": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := minOneInstanceArg(args, "writer"); err != nil {
					return err
				}
				delimiter, _ := csvDelimiter(kwargs)
				inst := object.NewInstanceWithFields(csvWriterClass, map[string]object.Object{
					"_target":    args[0],
					"_delimiter": object.NewString(string(delimiter)),
				})
				return inst
			},
			HelpText: `writer(fileobj, delimiter=",") - CSV writer over a write()-able object

writerow(row) writes one row; writerows(rows) writes many.`,
		},
		"reader": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := minOneInstanceArg(args, "reader"); err != nil {
					return err
				}
				lines := readAllLinesFromSource(ctx, args[0].(*object.Instance))
				if lines == nil {
					return csvSimpleError("csv.reader source must have read(), getvalue() or readline()")
				}
				inst := object.NewInstanceWithFields(csvReaderClass, map[string]object.Object{
					"_lines": &object.List{Elements: lines},
					"_pos":   object.NewInteger(0),
				})
				return inst
			},
			HelpText: `reader(fileobj, delimiter=",") - CSV reader over line content

Iterating yields rows (lists of strings).`,
		},
		"DictReader": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := minOneInstanceArg(args, "DictReader"); err != nil {
					return err
				}
				lines := readAllLinesFromSource(ctx, args[0].(*object.Instance))
				if lines == nil {
					return csvSimpleError("csv.DictReader source must have read(), getvalue() or readline()")
				}
				fields := map[string]object.Object{
					"_lines":     &object.List{Elements: lines},
					"_pos":       object.NewInteger(0),
					"fieldnames": &object.Null{},
				}
				// Positional or keyword fieldnames; restval fills short rows.
				if len(args) > 1 {
					names, errObj := args[1].AsList()
					if errObj != nil {
						return errObj
					}
					list := &object.List{Elements: append([]object.Object{}, names...)}
					fields["_fieldnames"] = list
					fields["fieldnames"] = list
				}
				if names, ok := kwargs.Kwargs["fieldnames"]; ok {
					nl, errObj := names.AsList()
					if errObj != nil {
						return errObj
					}
					list := &object.List{Elements: append([]object.Object{}, nl...)}
					fields["_fieldnames"] = list
					fields["fieldnames"] = list
				}
				if rv, ok := kwargs.Kwargs["restval"]; ok {
					fields["_restval"] = rv
				}
				return object.NewInstanceWithFields(csvDictReaderClass, fields)
			},
			HelpText: `DictReader(fileobj, fieldnames=None, restval=None) - CSV reader yielding dicts

The first row is the header unless fieldnames is given.`,
		},
		"DictWriter": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := minOneInstanceArg(args, "DictWriter"); err != nil {
					return err
				}
				if len(args) < 2 {
					return csvSimpleError("csv.DictWriter requires fieldnames")
				}
				names, errObj := args[1].AsList()
				if errObj != nil {
					return errObj
				}
				delimiter, _ := csvDelimiter(kwargs)
				fields := map[string]object.Object{
					"_target":     args[0],
					"_delimiter":  object.NewString(string(delimiter)),
					"_fieldnames": &object.List{Elements: append([]object.Object{}, names...)},
				}
				if rv, ok := kwargs.Kwargs["restval"]; ok {
					fields["_restval"] = rv
				}
				return object.NewInstanceWithFields(csvDictWriterClass, fields)
			},
			HelpText: `DictWriter(fileobj, fieldnames, restval="", delimiter=",")

writerow(dict) writes rows ordered by fieldnames; writeheader() writes the header.`,
		},
		"loads": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				content, err := args[0].AsString()
				if err != nil {
					return err
				}
				delimiter, _ := csvDelimiter(kwargs)

				reader := csv.NewReader(strings.NewReader(content))
				reader.Comma = delimiter
				reader.FieldsPerRecord = -1 // allow variable-length rows

				records, e := reader.ReadAll()
				if e != nil {
					return errors.NewError("csv.loads: %s", e.Error())
				}

				elements := make([]object.Object, len(records))
				for i, row := range records {
					rowElems := make([]object.Object, len(row))
					for j, val := range row {
						rowElems[j] = object.NewString(val)
					}
					elements[i] = &object.List{Elements: rowElems}
				}
				return &object.List{Elements: elements}
			},
			HelpText: `loads(content, delimiter=",") - Parse a CSV string into a list of rows

Returns a list of lists, where each inner list is a row of string values.
Handles quoting, embedded commas, and embedded newlines per RFC 4180.

Parameters:
  content   CSV text to parse
  delimiter Field delimiter character (default ",")

Returns:
  list[list[str]] - Rows of string values`,
		},
		"loads_dict": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				content, err := args[0].AsString()
				if err != nil {
					return err
				}
				delimiter, _ := csvDelimiter(kwargs)

				reader := csv.NewReader(strings.NewReader(content))
				reader.Comma = delimiter
				reader.FieldsPerRecord = -1

				records, e := reader.ReadAll()
				if e != nil {
					return errors.NewError("csv.loads_dict: %s", e.Error())
				}
				if len(records) == 0 {
					return &object.List{Elements: []object.Object{}}
				}

				headers := records[0]
				elements := make([]object.Object, 0, len(records)-1)
				for _, row := range records[1:] {
					d := &object.Dict{Pairs: make(map[string]object.DictPair)}
					for i, header := range headers {
						val := ""
						if i < len(row) {
							val = row[i]
						}
						d.SetByString(header, object.NewString(val))
					}
					elements = append(elements, d)
				}
				return &object.List{Elements: elements}
			},
			HelpText: `loads_dict(content, delimiter=",") - Parse CSV into a list of dicts

Treats the first row as column headers. Each subsequent row becomes a dict
mapping header names to cell values.

Parameters:
  content   CSV text to parse
  delimiter Field delimiter character (default ",")

Returns:
  list[dict] - List of dicts keyed by header names`,
		},
		"dumps": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				list, err := args[0].AsList()
				if err != nil {
					return err
				}
				delimiter, _ := csvDelimiter(kwargs)

				records := make([][]string, len(list))
				for i, row := range list {
					rowList, ok := row.(*object.List)
					if !ok {
						return errors.NewError("csv.dumps: row %d is not a list", i)
					}
					strs := make([]string, len(rowList.Elements))
					for j, cell := range rowList.Elements {
						s, e := cell.AsString()
						if e != nil {
							return errors.NewError("csv.dumps: cell [%d][%d] is not a string", i, j)
						}
						strs[j] = s
					}
					records[i] = strs
				}

				var buf bytes.Buffer
				w := csv.NewWriter(&buf)
				w.Comma = delimiter
				for _, record := range records {
					if e := w.Write(record); e != nil {
						return errors.NewError("csv.dumps: %s", e.Error())
					}
				}
				w.Flush()
				return object.NewString(buf.String())
			},
			HelpText: `dumps(rows, delimiter=",") - Format rows into a CSV string

Parameters:
  rows      List of lists (each inner list is a row of string values)
  delimiter Field delimiter character (default ",")

Returns:
  str - CSV-formatted text`,
		},
		"dumps_dict": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				list, err := args[0].AsList()
				if err != nil {
					return err
				}
				delimiter, _ := csvDelimiter(kwargs)

				if len(list) == 0 {
					return object.NewString("")
				}

				// Collect header order from the first dict.
				first, ok := list[0].(*object.Dict)
				if !ok {
					return errors.NewError("csv.dumps_dict: expected a list of dicts")
				}
				// Collect header order: explicit columns kwarg, or sorted keys.
				var headers []string
				if v := kwargs.Get("columns"); v != nil {
					colList, ok := v.(*object.List)
					if !ok {
						return errors.NewError("csv.dumps_dict: columns must be a list of strings")
					}
					for _, c := range colList.Elements {
						s, e := c.AsString()
						if e != nil {
							return errors.NewError("csv.dumps_dict: column name is not a string")
						}
						headers = append(headers, s)
					}
				} else {
					headers = make([]string, 0, len(first.Pairs))
					for _, pair := range first.Pairs {
						s, _ := pair.Key.AsString()
						headers = append(headers, s)
					}
					sort.Strings(headers)
				}

				var buf bytes.Buffer
				w := csv.NewWriter(&buf)
				w.Comma = delimiter
				w.Write(headers)

				for _, item := range list {
					d, ok := item.(*object.Dict)
					if !ok {
						return errors.NewError("csv.dumps_dict: row is not a dict")
					}
					row := make([]string, len(headers))
					for i, h := range headers {
						if val, ok := d.GetByString(h); ok {
							s, _ := val.Value.AsString()
							row[i] = s
						}
					}
					w.Write(row)
				}
				w.Flush()
				return object.NewString(buf.String())
			},
			HelpText: `dumps_dict(rows, delimiter=",") - Format dicts into a CSV string

Column headers are taken from the keys of the first dict. Each dict becomes
a row, with values written in header order.

Parameters:
  rows      List of dicts
  delimiter Field delimiter character (default ",")

Returns:
  str - CSV-formatted text with a header row`,
		},
	}, nil, "CSV parsing and formatting (string-based)")
}

// csvDelimiter reads the optional delimiter kwarg (default ',').
func csvDelimiter(kwargs object.Kwargs) (rune, object.Object) {
	if v := kwargs.Get("delimiter"); v != nil {
		s, err := v.AsString()
		if err != nil {
			return ',', err
		}
		if len(s) > 0 {
			return []rune(s)[0], nil
		}
	}
	return ',', nil
}

// newAliasedLibrary re-names a library so it can be imported under more than
// one module name. The functions and constants are shared by reference.
func newAliasedLibrary(lib *object.Library, name string) *object.Library {
	functions := map[string]*object.Builtin{}
	for fnName, fn := range lib.Functions() {
		functions[fnName] = fn
	}
	constants := lib.Constants()
	if constants == nil {
		constants = map[string]object.Object{}
	}
	return object.NewLibrary(name, functions, constants, lib.Description())
}

func minOneInstanceArg(args []object.Object, what string) object.Object {
	if len(args) < 1 {
		return csvSimpleError("%s() requires a file object", what)
	}
	if _, ok := args[0].(*object.Instance); !ok {
		return csvSimpleError("%s() requires a file-like object", what)
	}
	return nil
}

// readAllLinesFromSource pulls the full text from an object exposing one of
// read(), getvalue() or readline().
func readAllLinesFromSource(ctx context.Context, src *object.Instance) []object.Object {
	callNoArg := func(name string) object.Object {
		fn, ok := src.Class.LookupMember(name)
		if !ok {
			return nil
		}
		if b, ok := fn.(*object.Builtin); ok && b.Fn != nil {
			return b.Fn(ctx, object.Kwargs{}, src)
		}
		return nil
	}
	var text string
	if res := callNoArg("getvalue"); res != nil {
		if s, ok := res.(*object.String); ok {
			text = s.StringValue()
		}
	}
	if text == "" {
		if res := callNoArg("read"); res != nil {
			if s, ok := res.(*object.String); ok {
				text = s.StringValue()
			}
		}
	}
	if text == "" {
		return nil
	}
	raw := strings.SplitAfter(text, "\n")
	elems := make([]object.Object, 0, len(raw))
	for _, ln := range raw {
		elems = append(elems, object.NewString(ln))
	}
	return elems
}
