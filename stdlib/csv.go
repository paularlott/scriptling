package stdlib

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/libutil"
	"github.com/paularlott/scriptling/object"
)

// CSVLibrary provides the scriptling.csv library. RegisterAll also registers
// it as "csv" (Python-compatible) through CSVAliasLibrary.
var CSVLibrary = newCSVLibrary()

// CSVAliasLibrary is CSVLibrary importable under the Python name "csv".
var CSVAliasLibrary = libutil.NewAliasedLibrary(CSVLibrary, CSVAliasLibraryName)

func newCSVLibrary() *object.Library {
	fns := csvLibraryFunctions()
	for name, fn := range csvPythonFunctions() {
		fns[name] = fn
	}
	return object.NewLibrary(CSVLibraryName, fns, csvConstants, "CSV parsing and formatting (string-based)")
}

func csvLibraryFunctions() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"loads": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("loads", kwargs, "delimiter"); err != nil {
						return err
					}
				}
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

				reader.ReuseRecord = true

				// Rows are converted as they are read, so the parsed rows are
				// never all held natively on top of the values they become.
				out, errObj := libutil.NewResultList(ctx, 0)
				if errObj != nil {
					return errObj
				}
				for {
					row, e := reader.Read()
					if e == io.EOF {
						break
					}
					if e != nil {
						return errors.NewError("csv.loads: %s", e.Error())
					}
					if errObj := object.CheckElements(int64(len(row))); errObj != nil {
						return errObj
					}
					rowElems := make([]object.Object, len(row))
					object.TrackSlice(rowElems)
					for j, val := range row {
						rowElems[j] = object.NewString(val)
					}
					if errObj := out.Add(&object.List{Elements: rowElems}); errObj != nil {
						return errObj
					}
				}
				return out.List()
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
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("loads_dict", kwargs, "delimiter"); err != nil {
						return err
					}
				}
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
					d := object.NewDict()
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
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("dumps", kwargs, "delimiter"); err != nil {
						return err
					}
				}
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
						return errors.NewTypeErrorTagged("csv.dumps: row %d is not a list", i)
					}
					strs := make([]string, len(rowList.Elements))
					for j, cell := range rowList.Elements {
						s, e := cell.AsString()
						if e != nil {
							return errors.NewTypeErrorTagged("csv.dumps: cell [%d][%d] is not a string", i, j)
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
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("dumps_dict", kwargs, "delimiter", "columns"); err != nil {
						return err
					}
				}
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
					return errors.NewTypeErrorTagged("csv.dumps_dict: expected a list of dicts")
				}
				// Collect header order: explicit columns kwarg, or sorted keys.
				var headers []string
				if v := kwargs.Get("columns"); v != nil {
					colList, ok := v.(*object.List)
					if !ok {
						return errors.NewTypeErrorTagged("csv.dumps_dict: columns must be a list of strings")
					}
					for _, c := range colList.Elements {
						s, e := c.AsString()
						if e != nil {
							return errors.NewTypeErrorTagged("csv.dumps_dict: column name is not a string")
						}
						headers = append(headers, s)
					}
				} else {
					headers = make([]string, 0, first.Len())
					for _, pair := range first.All() {
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
						return errors.NewTypeErrorTagged("csv.dumps_dict: row is not a dict")
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
	}
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
