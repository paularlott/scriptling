package stdlib

import (
	"context"
	"strconv"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Python's csv.reader, csv.writer, csv.DictReader and csv.DictWriter: the
// reader is a state machine over the lines of any iterable of strings (a
// list, a file or StringIO), so quoted fields may span lines and a blank
// line is an empty row; the writer quotes as Python's QUOTE_* modes say.
// Both take Python's dialect parameters.

const (
	csvQuoteMinimal = iota
	csvQuoteAll
	csvQuoteNonnumeric
	csvQuoteNone
	csvQuoteStrings
	csvQuoteNotnull
)

// csvConstants are the csv module's constants.
var csvConstants = map[string]object.Object{
	"QUOTE_MINIMAL":    object.NewInteger(csvQuoteMinimal),
	"QUOTE_ALL":        object.NewInteger(csvQuoteAll),
	"QUOTE_NONNUMERIC": object.NewInteger(csvQuoteNonnumeric),
	"QUOTE_NONE":       object.NewInteger(csvQuoteNone),
	"QUOTE_STRINGS":    object.NewInteger(csvQuoteStrings),
	"QUOTE_NOTNULL":    object.NewInteger(csvQuoteNotnull),
}

type csvDialect struct {
	delimiter        rune
	quotechar        rune // 0 for None
	escapechar       rune // 0 for None
	doublequote      bool
	skipinitialspace bool
	lineterminator   string
	quoting          int
	strict           bool
}

// csvDialectParams are the keyword arguments every reader and writer takes.
var csvDialectParams = []string{"dialect", "delimiter", "quotechar", "escapechar", "doublequote", "skipinitialspace", "lineterminator", "quoting", "strict"}

func csvError(format string, a ...any) object.Object {
	return errors.NewValueError(format, a...)
}

// csvDialectFrom reads a dialect: a named one ('excel', 'excel-tab',
// 'unix') overridden by individual parameters.
func csvDialectFrom(kwargs object.Kwargs, dialect object.Object) (*csvDialect, object.Object) {
	d := &csvDialect{delimiter: ',', quotechar: '"', doublequote: true, lineterminator: "\r\n"}
	if dialect == nil {
		dialect = kwargs.Get("dialect")
	}
	if dialect != nil && dialect.Type() != object.NULL_OBJ {
		name, err := dialect.AsString()
		if err != nil {
			return nil, errors.NewTypeErrorTagged("dialect must be a name ('excel', 'excel-tab' or 'unix')")
		}
		switch name {
		case "excel":
		case "excel-tab":
			d.delimiter = '\t'
		case "unix":
			d.lineterminator = "\n"
			d.quoting = csvQuoteAll
		default:
			return nil, csvError("unknown dialect")
		}
	}
	char := func(name string, v object.Object, allowNone bool) (rune, object.Object) {
		if v.Type() == object.NULL_OBJ && allowNone {
			return 0, nil
		}
		s, ok := v.(*object.String)
		if !ok || len([]rune(s.StringValue())) != 1 {
			return 0, errors.NewTypeErrorTagged("\"%s\" must be a 1-character string", name)
		}
		return []rune(s.StringValue())[0], nil
	}
	for key, v := range kwargs.All() {
		var errObj object.Object
		switch key {
		case "delimiter":
			d.delimiter, errObj = char(key, v, false)
		case "quotechar":
			d.quotechar, errObj = char(key, v, true)
		case "escapechar":
			d.escapechar, errObj = char(key, v, true)
		case "doublequote":
			d.doublequote = isTruthy(v)
		case "skipinitialspace":
			d.skipinitialspace = isTruthy(v)
		case "strict":
			d.strict = isTruthy(v)
		case "lineterminator":
			s, ok := v.(*object.String)
			if !ok {
				return nil, errors.NewTypeErrorTagged("\"lineterminator\" must be a string")
			}
			d.lineterminator = s.StringValue()
		case "quoting":
			n, ok := v.(*object.Integer)
			if !ok || n.IntValue() < 0 || n.IntValue() > csvQuoteNotnull {
				return nil, errors.NewTypeErrorTagged("bad \"quoting\" value")
			}
			d.quoting = int(n.IntValue())
		}
		if errObj != nil {
			return nil, errObj
		}
	}
	if d.quoting != csvQuoteNone && d.quotechar == 0 {
		return nil, errors.NewTypeErrorTagged("quotechar must be set if quoting enabled")
	}
	return d, nil
}

// csvLineSource returns the lines of a reader's input: a file-like object's
// whole text split after each newline, or the strings any other iterable
// yields.
func csvLineSource(ctx context.Context, src object.Object) (func() (object.Object, bool), object.Object) {
	if inst, ok := src.(*object.Instance); ok {
		if _, has := inst.Class.LookupMember("getvalue"); has {
			if lines := readAllLinesFromSource(ctx, inst); lines != nil {
				if n := len(lines); n > 0 && lines[n-1].Inspect() == "" {
					lines = lines[:n-1]
				}
				next, _ := object.IterSource(&object.List{Elements: lines})
				return next, nil
			}
		}
	}
	if s, ok := src.(*object.String); ok {
		// A str iterates by character, as in Python.
		chars := make([]object.Object, 0, len(s.StringValue()))
		for _, r := range s.StringValue() {
			chars = append(chars, object.NewString(string(r)))
		}
		next, _ := object.IterSource(&object.List{Elements: chars})
		return next, nil
	}
	next, errObj := lazySource(ctx, src)
	if errObj != nil {
		return nil, errObj
	}
	return next, nil
}

// csvParser reads records from a line source.
type csvParser struct {
	next    func() (object.Object, bool)
	d       *csvDialect
	lineNum int64
}

const csvEOL = -1

// readRecord returns the next record (nil at the end of the input), with
// each field's quoted flag.
func (p *csvParser) readRecord() ([]string, []bool, object.Object) {
	const (
		startRecord = iota
		startField
		escapedChar
		inField
		inQuotedField
		escapeInQuotedField
		quoteInQuotedField
		eatCRNL
	)
	d := p.d
	var fields []string
	var quoted []bool
	var field strings.Builder
	fieldQuoted := false
	state := startRecord
	save := func() {
		fields = append(fields, field.String())
		quoted = append(quoted, fieldQuoted)
		field.Reset()
		fieldQuoted = false
	}
	for {
		lineObj, ok := p.next()
		if !ok {
			if state == startRecord {
				return nil, nil, nil
			}
			if state == inQuotedField || state == escapeInQuotedField {
				if d.strict {
					return nil, nil, csvError("unexpected end of data")
				}
			}
			if state != eatCRNL {
				save()
			}
			return fields, quoted, nil
		}
		if object.IsPropagating(lineObj) {
			return nil, nil, lineObj
		}
		line, isStr := lineObj.(*object.String)
		if !isStr {
			return nil, nil, csvError("iterator should return strings, not %s (the file should be opened in text mode)", object.PythonTypeName(lineObj))
		}
		p.lineNum++
		chars := []rune(line.StringValue())
		for i := 0; i <= len(chars); i++ {
			c := rune(csvEOL)
			if i < len(chars) {
				c = chars[i]
			}
			if c == 0 && i < len(chars) {
				return nil, nil, csvError("line contains NUL")
			}
			isNL := c == '\n' || c == '\r'
		again:
			switch state {
			case startRecord:
				if c == csvEOL {
					// A blank line is an empty row.
					return []string{}, []bool{}, nil
				}
				if isNL {
					state = eatCRNL
					continue
				}
				state = startField
				goto again
			case startField:
				switch {
				case isNL || c == csvEOL:
					save()
					if c == csvEOL {
						return fields, quoted, nil
					}
					state = eatCRNL
				case c == d.quotechar && d.quoting != csvQuoteNone:
					fieldQuoted = true
					state = inQuotedField
				case c == d.escapechar && d.escapechar != 0:
					state = escapedChar
				case c == ' ' && d.skipinitialspace:
				case c == d.delimiter:
					save()
				default:
					field.WriteRune(c)
					state = inField
				}
			case escapedChar:
				if c == csvEOL {
					field.WriteRune('\n')
				} else {
					field.WriteRune(c)
				}
				state = inField
			case inField:
				switch {
				case isNL || c == csvEOL:
					save()
					if c == csvEOL {
						return fields, quoted, nil
					}
					state = eatCRNL
				case c == d.escapechar && d.escapechar != 0:
					state = escapedChar
				case c == d.delimiter:
					save()
					state = startField
				default:
					field.WriteRune(c)
				}
			case inQuotedField:
				switch {
				case c == csvEOL:
					// The quoted field continues on the next line.
				case c == d.escapechar && d.escapechar != 0:
					state = escapeInQuotedField
				case c == d.quotechar:
					if d.doublequote {
						state = quoteInQuotedField
					} else {
						state = inField
					}
				default:
					field.WriteRune(c)
				}
			case escapeInQuotedField:
				if c == csvEOL {
					field.WriteRune('\n')
				} else {
					field.WriteRune(c)
				}
				state = inQuotedField
			case quoteInQuotedField:
				switch {
				case c == d.quotechar:
					field.WriteRune(c)
					state = inQuotedField
				case c == d.delimiter:
					save()
					state = startField
				case isNL || c == csvEOL:
					save()
					if c == csvEOL {
						return fields, quoted, nil
					}
					state = eatCRNL
				case !d.strict:
					field.WriteRune(c)
					state = inField
				default:
					return nil, nil, csvError("'%c' expected after '%c'", d.delimiter, d.quotechar)
				}
			case eatCRNL:
				switch {
				case isNL:
				case c == csvEOL:
					return fields, quoted, nil
				default:
					return nil, nil, csvError("new-line character seen in unquoted field - do you need to open the file with newline=''?")
				}
			}
		}
	}
}

// csvRowObject converts a record to a list, numbers for unquoted fields
// under QUOTE_NONNUMERIC.
func csvRowObject(fields []string, quoted []bool, d *csvDialect) object.Object {
	elems := make([]object.Object, len(fields))
	for i, f := range fields {
		if d.quoting == csvQuoteNonnumeric && !quoted[i] {
			v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
			if err != nil {
				return csvError("could not convert string to float: %s", object.ReprString(f))
			}
			elems[i] = object.NewFloat(v)
			continue
		}
		elems[i] = object.NewString(f)
	}
	object.TrackSlice(elems)
	return &object.List{Elements: elems}
}

var stopIteration = func() object.Object {
	return &object.Exception{Message: "StopIteration", ExceptionType: object.ExceptionTypeStopIteration, Raised: true}
}

func selfIter() *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("__iter__", kwargs); err != nil {
				return err
			}
			return args[0]
		},
		HelpText: "__iter__() - Readers are their own iterator",
	}
}

// pyCSVReaderClass is csv.reader's class; its parser is the instance's
// native data.
var pyCSVReaderClass = &object.Class{
	Name: "_csv.reader",
	Methods: map[string]object.Object{
		"__iter__": selfIter(),
		"__next__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__next__", kwargs); err != nil {
					return err
				}
				self, _ := args[0].(*object.Instance)
				p, _ := self.NativeData.(*csvParser)
				if p == nil {
					return csvSimpleError("__next__() must be called on a csv.reader")
				}
				fields, quoted, errObj := p.readRecord()
				if errObj != nil {
					return errObj
				}
				self.SetField("line_num", object.NewInteger(p.lineNum))
				if fields == nil {
					return stopIteration()
				}
				return csvRowObject(fields, quoted, p.d)
			},
			HelpText: "__next__() - The next row, as a list of strings",
		},
	},
}

func newCSVReader(ctx context.Context, src, dialect object.Object, kwargs object.Kwargs) (*object.Instance, *csvParser, object.Object) {
	d, errObj := csvDialectFrom(kwargs, dialect)
	if errObj != nil {
		return nil, nil, errObj
	}
	next, errObj := csvLineSource(ctx, src)
	if errObj != nil {
		return nil, nil, errObj
	}
	p := &csvParser{next: next, d: d}
	inst := object.NewInstanceWithData(pyCSVReaderClass, map[string]object.Object{
		"line_num": object.NewInteger(0),
		"dialect":  &object.Null{},
	}, p)
	return inst, p, nil
}

// csvFormatRow renders one row as the writer's dialect says.
func csvFormatRow(ctx context.Context, row []object.Object, d *csvDialect) (string, object.Object) {
	var b strings.Builder
	special := string(d.delimiter) + "\r\n" + d.lineterminator
	if d.quotechar != 0 {
		special += string(d.quotechar)
	}
	if d.escapechar != 0 {
		special += string(d.escapechar)
	}
	for i, cell := range row {
		if i > 0 {
			b.WriteRune(d.delimiter)
		}
		var s string
		numeric, isNone := false, false
		switch v := cell.(type) {
		case *object.String:
			s = v.StringValue()
		case *object.Null:
			isNone = true
		case *object.Integer, *object.Boolean:
			s, numeric = cell.Inspect(), true
		case *object.Float:
			s, numeric = object.FloatStr(v.FloatValue()), true
		default:
			rendered, errObj := renderStr(ctx, cell)
			if errObj != nil {
				return "", errObj
			}
			s = rendered
		}
		var quote bool
		switch d.quoting {
		case csvQuoteAll:
			quote = true
		case csvQuoteNonnumeric:
			quote = !numeric
		case csvQuoteStrings:
			quote = !numeric && !isNone
		case csvQuoteNotnull:
			quote = !isNone
		case csvQuoteMinimal:
			quote = strings.ContainsAny(s, special) || (s == "" && len(row) == 1)
			if d.escapechar != 0 && !d.doublequote {
				quote = strings.ContainsAny(s, string(d.delimiter)+"\r\n"+d.lineterminator) || (s == "" && len(row) == 1)
			}
		}
		if d.quoting == csvQuoteNone {
			for _, r := range s {
				if strings.ContainsRune(special, r) {
					if d.escapechar == 0 {
						return "", csvError("need to escape, but no escapechar set")
					}
					b.WriteRune(d.escapechar)
				}
				b.WriteRune(r)
			}
			continue
		}
		if quote {
			b.WriteRune(d.quotechar)
		}
		for _, r := range s {
			switch {
			case r == d.quotechar && d.quotechar != 0:
				if d.doublequote {
					b.WriteRune(r)
				} else if d.escapechar != 0 {
					b.WriteRune(d.escapechar)
				} else {
					return "", csvError("need to escape, but no escapechar set")
				}
			case r == d.escapechar && d.escapechar != 0:
				b.WriteRune(d.escapechar)
			case !quote && d.escapechar != 0 && strings.ContainsRune(string(d.delimiter)+"\r\n", r):
				b.WriteRune(d.escapechar)
			}
			b.WriteRune(r)
		}
		if quote {
			b.WriteRune(d.quotechar)
		}
	}
	b.WriteString(d.lineterminator)
	if errObj := object.CheckAllocFor("CSV text", int64(b.Len())); errObj != nil {
		return "", errObj
	}
	return b.String(), nil
}

// renderStr is str(v) for a cell.
func renderStr(ctx context.Context, v object.Object) (string, object.Object) {
	return object.Str(ctx, v)
}

// csvWriteText writes text through the target's write().
func csvWriteText(ctx context.Context, target object.Object, text string) object.Object {
	inst, ok := target.(*object.Instance)
	if !ok {
		return csvSimpleError("argument 1 must have a \"write\" method")
	}
	writeFn, ok := inst.Class.LookupMember("write")
	if !ok {
		return csvSimpleError("argument 1 must have a \"write\" method")
	}
	if b, ok := writeFn.(*object.Builtin); ok && b.Fn != nil {
		return b.Fn(ctx, object.Kwargs{}, inst, object.NewString(text))
	}
	return callCallable(ctx, writeFn, inst, object.NewString(text))
}

func csvRowElements(ctx context.Context, row object.Object) ([]object.Object, object.Object) {
	switch r := row.(type) {
	case *object.List:
		return r.Elements, nil
	case *object.Tuple:
		return r.Elements, nil
	case *object.String:
		return nil, csvError("iterable expected, not str")
	}
	elems, errObj := collectIterable(ctx, row)
	if errObj != nil {
		return nil, csvError("iterable expected, not %s", object.PythonTypeName(row))
	}
	return elems, nil
}

type csvWriterState struct {
	target object.Object
	d      *csvDialect
}

var pyCSVWriterClass = &object.Class{Name: "_csv.writer", Methods: map[string]object.Object{}}

func csvWriterOf(args []object.Object, name string) (*csvWriterState, object.Object) {
	if len(args) < 1 {
		return nil, csvSimpleError("%s() must be called on a csv writer", name)
	}
	self, _ := args[0].(*object.Instance)
	if self == nil {
		return nil, csvSimpleError("%s() must be called on a csv writer", name)
	}
	w, _ := self.NativeData.(*csvWriterState)
	if w == nil {
		return nil, csvSimpleError("%s() must be called on a csv writer", name)
	}
	return w, nil
}

func init() {
	pyCSVWriterClass.Methods["writerow"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("writerow", kwargs); err != nil {
				return err
			}
			w, errObj := csvWriterOf(args, "writerow")
			if errObj != nil {
				return errObj
			}
			if len(args) != 2 {
				return errors.NewTypeErrorTagged("writerow() takes exactly one argument (%d given)", len(args)-1)
			}
			row, errObj := csvRowElements(ctx, args[1])
			if errObj != nil {
				return errObj
			}
			text, errObj := csvFormatRow(ctx, row, w.d)
			if errObj != nil {
				return errObj
			}
			return csvWriteText(ctx, w.target, text)
		},
		HelpText: "writerow(row) - Write one row; returns what write() returned",
	}
	pyCSVWriterClass.Methods["writerows"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("writerows", kwargs); err != nil {
				return err
			}
			w, errObj := csvWriterOf(args, "writerows")
			if errObj != nil {
				return errObj
			}
			if len(args) != 2 {
				return errors.NewTypeErrorTagged("writerows() takes exactly one argument (%d given)", len(args)-1)
			}
			rows, errObj := collectIterable(ctx, args[1])
			if errObj != nil {
				return errObj
			}
			var b strings.Builder
			for _, r := range rows {
				row, errObj := csvRowElements(ctx, r)
				if errObj != nil {
					return errObj
				}
				text, errObj := csvFormatRow(ctx, row, w.d)
				if errObj != nil {
					return errObj
				}
				b.WriteString(text)
				if errObj := object.CheckAllocFor("CSV text", int64(b.Len())); errObj != nil {
					return errObj
				}
			}
			if b.Len() == 0 {
				return &object.Null{}
			}
			if r := csvWriteText(ctx, w.target, b.String()); object.IsPropagating(r) {
				return r
			}
			return &object.Null{}
		},
		HelpText: "writerows(rows) - Write many rows",
	}
	pyCSVDictWriterClass.Methods["writeheader"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("writeheader", kwargs); err != nil {
				return err
			}
			w, errObj := csvWriterOf(args, "writeheader")
			if errObj != nil {
				return errObj
			}
			names, errObj := csvRowElements(ctx, args[0].(*object.Instance).Field("fieldnames"))
			if errObj != nil {
				return errObj
			}
			text, errObj := csvFormatRow(ctx, names, w.d)
			if errObj != nil {
				return errObj
			}
			return csvWriteText(ctx, w.target, text)
		},
		HelpText: "writeheader() - Write the field names as a row",
	}
	pyCSVDictWriterClass.Methods["writerow"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("writerow", kwargs); err != nil {
				return err
			}
			w, errObj := csvWriterOf(args, "writerow")
			if errObj != nil {
				return errObj
			}
			if len(args) != 2 {
				return errors.NewTypeErrorTagged("writerow() takes exactly one argument (%d given)", len(args)-1)
			}
			row, errObj := csvDictRow(ctx, args[0].(*object.Instance), args[1])
			if errObj != nil {
				return errObj
			}
			text, errObj := csvFormatRow(ctx, row, w.d)
			if errObj != nil {
				return errObj
			}
			return csvWriteText(ctx, w.target, text)
		},
		HelpText: "writerow(rowdict) - Write one dict as a row in fieldnames order",
	}
	pyCSVDictWriterClass.Methods["writerows"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("writerows", kwargs); err != nil {
				return err
			}
			w, errObj := csvWriterOf(args, "writerows")
			if errObj != nil {
				return errObj
			}
			if len(args) != 2 {
				return errors.NewTypeErrorTagged("writerows() takes exactly one argument (%d given)", len(args)-1)
			}
			rows, errObj := collectIterable(ctx, args[1])
			if errObj != nil {
				return errObj
			}
			var b strings.Builder
			for _, r := range rows {
				row, errObj := csvDictRow(ctx, args[0].(*object.Instance), r)
				if errObj != nil {
					return errObj
				}
				text, errObj := csvFormatRow(ctx, row, w.d)
				if errObj != nil {
					return errObj
				}
				b.WriteString(text)
				if errObj := object.CheckAllocFor("CSV text", int64(b.Len())); errObj != nil {
					return errObj
				}
			}
			if b.Len() > 0 {
				if r := csvWriteText(ctx, w.target, b.String()); object.IsPropagating(r) {
					return r
				}
			}
			return &object.Null{}
		},
		HelpText: "writerows(rowdicts) - Write many dicts as rows",
	}
}

var pyCSVDictWriterClass = &object.Class{Name: "DictWriter", Methods: map[string]object.Object{}}

// csvDictRow projects a dict onto the writer's fieldnames.
func csvDictRow(ctx context.Context, self *object.Instance, rowObj object.Object) ([]object.Object, object.Object) {
	names, errObj := csvRowElements(ctx, self.Field("fieldnames"))
	if errObj != nil {
		return nil, errObj
	}
	d, ok := rowObj.(*object.Dict)
	if !ok {
		return nil, errors.NewTypeErrorTagged("DictWriter rows must be dicts, not %s", object.PythonTypeName(rowObj))
	}
	if s, _ := self.Field("extrasaction").(*object.String); s == nil || s.StringValue() == "raise" {
		var extra []string
		for _, pair := range d.All() {
			found := false
			for _, n := range names {
				eq, errObj := object.Equal(ctx, n, pair.Key)
				if errObj != nil {
					return nil, errObj
				}
				if eq {
					found = true
					break
				}
			}
			if !found {
				extra = append(extra, object.ReprString(pair.Key.Inspect()))
				if _, isStr := pair.Key.(*object.String); !isStr {
					extra[len(extra)-1] = pair.Key.Inspect()
				}
			}
		}
		if len(extra) > 0 {
			return nil, csvError("dict contains fields not in fieldnames: %s", strings.Join(extra, ", "))
		}
	}
	restval := self.Field("restval")
	row := make([]object.Object, len(names))
	for i, n := range names {
		if pair, ok := d.Lookup(object.DictKey(n)); ok {
			row[i] = pair.Value
		} else {
			row[i] = restval
		}
	}
	return row, nil
}

var pyCSVDictReaderClass = &object.Class{
	Name: "DictReader",
	Methods: map[string]object.Object{
		"__iter__": selfIter(),
		"__next__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__next__", kwargs); err != nil {
					return err
				}
				self, _ := args[0].(*object.Instance)
				p, _ := self.NativeData.(*csvParser)
				if p == nil {
					return csvSimpleError("__next__() must be called on a csv.DictReader")
				}
				names, _ := self.Field("fieldnames").(*object.List)
				if names == nil {
					return stopIteration()
				}
				// Blank rows are skipped, as in Python.
				var fields []string
				var quoted []bool
				for {
					var errObj object.Object
					fields, quoted, errObj = p.readRecord()
					if errObj != nil {
						return errObj
					}
					self.SetField("line_num", object.NewInteger(p.lineNum))
					if fields == nil {
						return stopIteration()
					}
					if len(fields) > 0 {
						break
					}
				}
				rowObj := csvRowObject(fields, quoted, p.d)
				row, ok := rowObj.(*object.List)
				if !ok {
					return rowObj
				}
				d := object.NewDict()
				n := min(len(names.Elements), len(row.Elements))
				for i := 0; i < n; i++ {
					d.Store(object.DictKey(names.Elements[i]), names.Elements[i], row.Elements[i])
				}
				if len(row.Elements) > len(names.Elements) {
					restkey := self.Field("restkey")
					rest := append([]object.Object(nil), row.Elements[len(names.Elements):]...)
					d.Store(object.DictKey(restkey), restkey, &object.List{Elements: rest})
				}
				for i := len(row.Elements); i < len(names.Elements); i++ {
					d.Store(object.DictKey(names.Elements[i]), names.Elements[i], self.Field("restval"))
				}
				return d
			},
			HelpText: "__next__() - The next row, as a dict keyed by fieldnames",
		},
	},
}

// csvPythonFunctions are csv.reader, csv.writer, csv.DictReader and
// csv.DictWriter.
func csvPythonFunctions() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"reader": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("reader", kwargs, csvDialectParams...); err != nil {
						return err
					}
				}
				if len(args) < 1 || len(args) > 2 {
					return errors.NewTypeErrorTagged("reader() takes 1 or 2 positional arguments (%d given)", len(args))
				}
				var dialect object.Object
				if len(args) == 2 {
					dialect = args[1]
				}
				inst, _, errObj := newCSVReader(ctx, args[0], dialect, kwargs)
				if errObj != nil {
					return errObj
				}
				return inst
			},
			HelpText: `reader(csvfile, dialect='excel', **fmtparams) - Rows of a CSV source

csvfile is any iterable of lines: a file, io.StringIO or a list of strings.
Each row is a list of strings; quoted fields may span lines. fmtparams:
delimiter, quotechar, escapechar, doublequote, skipinitialspace, quoting,
strict.`,
		},
		"writer": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("writer", kwargs, csvDialectParams...); err != nil {
						return err
					}
				}
				if len(args) < 1 || len(args) > 2 {
					return errors.NewTypeErrorTagged("writer() takes 1 or 2 positional arguments (%d given)", len(args))
				}
				var dialect object.Object
				if len(args) == 2 {
					dialect = args[1]
				}
				d, errObj := csvDialectFrom(kwargs, dialect)
				if errObj != nil {
					return errObj
				}
				if inst, ok := args[0].(*object.Instance); !ok {
					return csvSimpleError("argument 1 must have a \"write\" method")
				} else if _, has := inst.Class.LookupMember("write"); !has {
					return csvSimpleError("argument 1 must have a \"write\" method")
				}
				return object.NewInstanceWithData(pyCSVWriterClass, map[string]object.Object{"dialect": &object.Null{}}, &csvWriterState{target: args[0], d: d})
			},
			HelpText: `writer(csvfile, dialect='excel', **fmtparams) - A CSV writer over a write()-able object

writerow(row) writes one row, writerows(rows) many. Rows end with \r\n
unless lineterminator says otherwise; quoting is csv.QUOTE_MINIMAL,
QUOTE_ALL, QUOTE_NONNUMERIC or QUOTE_NONE.`,
		},
		"DictReader": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				own := []string{"fieldnames", "restkey", "restval"}
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("DictReader", kwargs, append(own, csvDialectParams...)...); err != nil {
						return err
					}
				}
				if len(args) < 1 || len(args) > 5 {
					return errors.NewTypeErrorTagged("DictReader() takes 1 to 5 positional arguments (%d given)", len(args))
				}
				vals := map[string]object.Object{}
				for i, a := range args[1:] {
					vals[[]string{"fieldnames", "restkey", "restval", "dialect"}[i]] = a
				}
				fmtMap := map[string]object.Object{}
				for k, v := range kwargs.All() {
					if k == "fieldnames" || k == "restkey" || k == "restval" {
						vals[k] = v
					} else {
						fmtMap[k] = v
					}
				}
				inst, p, errObj := newCSVReader(ctx, args[0], vals["dialect"], object.NewKwargs(fmtMap))
				if errObj != nil {
					return errObj
				}
				inst.Class = pyCSVDictReaderClass
				get := func(k string) object.Object {
					if v, ok := vals[k]; ok && v != nil {
						return v
					}
					return &object.Null{}
				}
				inst.SetField("restkey", get("restkey"))
				inst.SetField("restval", get("restval"))
				names := get("fieldnames")
				if names.Type() == object.NULL_OBJ {
					// The header row, read now so .fieldnames is ready.
					for {
						fields, _, errObj := p.readRecord()
						if errObj != nil {
							return errObj
						}
						if fields == nil {
							break
						}
						if len(fields) == 0 {
							continue
						}
						elems := make([]object.Object, len(fields))
						for i, f := range fields {
							elems[i] = object.NewString(f)
						}
						names = &object.List{Elements: elems}
						break
					}
				} else {
					elems, errObj := collectIterable(ctx, names)
					if errObj != nil {
						return errObj
					}
					names = &object.List{Elements: elems}
				}
				inst.SetField("fieldnames", names)
				inst.SetField("line_num", object.NewInteger(p.lineNum))
				return inst
			},
			HelpText: `DictReader(f, fieldnames=None, restkey=None, restval=None, dialect='excel', **fmtparams)

Rows as dicts keyed by fieldnames (the first row when not given). Extra
values collect in a list under restkey; missing ones are restval.`,
		},
		"DictWriter": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				own := []string{"fieldnames", "restval", "extrasaction"}
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("DictWriter", kwargs, append(own, csvDialectParams...)...); err != nil {
						return err
					}
				}
				if len(args) < 1 || len(args) > 5 {
					return errors.NewTypeErrorTagged("DictWriter() takes 2 to 5 positional arguments (%d given)", len(args))
				}
				vals := map[string]object.Object{}
				for i, a := range args[1:] {
					vals[[]string{"fieldnames", "restval", "extrasaction", "dialect"}[i]] = a
				}
				fmtMap := map[string]object.Object{}
				for k, v := range kwargs.All() {
					if k == "fieldnames" || k == "restval" || k == "extrasaction" {
						vals[k] = v
					} else {
						fmtMap[k] = v
					}
				}
				names, ok := vals["fieldnames"]
				if !ok {
					return errors.NewTypeErrorTagged("DictWriter() missing required argument: 'fieldnames'")
				}
				elems, errObj := collectIterable(ctx, names)
				if errObj != nil {
					return errObj
				}
				d, errObj := csvDialectFrom(object.NewKwargs(fmtMap), vals["dialect"])
				if errObj != nil {
					return errObj
				}
				extras := object.Object(object.NewString("raise"))
				if v, ok := vals["extrasaction"]; ok {
					s, isStr := v.(*object.String)
					if !isStr || (s.StringValue() != "raise" && s.StringValue() != "ignore") {
						return csvError("extrasaction (%s) must be 'raise' or 'ignore'", v.Inspect())
					}
					extras = s
				}
				restval := object.Object(object.NewString(""))
				if v, ok := vals["restval"]; ok {
					restval = v
				}
				if inst, ok := args[0].(*object.Instance); !ok {
					return csvSimpleError("argument 1 must have a \"write\" method")
				} else if _, has := inst.Class.LookupMember("write"); !has {
					return csvSimpleError("argument 1 must have a \"write\" method")
				}
				return object.NewInstanceWithData(pyCSVDictWriterClass, map[string]object.Object{
					"fieldnames":   &object.List{Elements: elems},
					"restval":      restval,
					"extrasaction": extras,
				}, &csvWriterState{target: args[0], d: d})
			},
			HelpText: `DictWriter(f, fieldnames, restval='', extrasaction='raise', dialect='excel', **fmtparams)

writeheader() writes the field names; writerow(dict) writes a dict's values
in fieldnames order (restval for missing keys; extra keys raise ValueError
unless extrasaction='ignore').`,
		},
	}
}
