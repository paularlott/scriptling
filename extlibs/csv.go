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

// RegisterCsvLibrary registers the scriptling.csv library.
func RegisterCsvLibrary(registrar object.LibraryRegistrar) {
	registrar.RegisterLibrary(NewCsvLibrary())
}

func NewCsvLibrary() *object.Library {
	return object.NewLibrary(CsvLibraryName, map[string]*object.Builtin{
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
			HelpText: `parse(content, delimiter=",") - Parse a CSV string into a list of rows

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
			HelpText: `parse_dict(content, delimiter=",") - Parse CSV into a list of dicts

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
			HelpText: `format(rows, delimiter=",") - Format rows into a CSV string

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
			HelpText: `format_dict(rows, delimiter=",") - Format dicts into a CSV string

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
