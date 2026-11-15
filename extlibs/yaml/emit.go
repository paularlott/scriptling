package yaml

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/paularlott/scriptling/object"
	"gopkg.in/yaml.v3"
)

// The YAML emitter.
//
// yaml.Marshal works on a Go value, which a script value with shared
// references expands into (every reference converts in full), and its
// emitter keeps every event of the document until it is done: about 1.6 KB
// for each value written. A small script value could so make it allocate
// gigabytes before any limit was checked. This emitter writes script values
// directly, laying them out exactly as yaml.Marshal does (block style, four
// space indents, keys in yaml.v3's order), and holds the text to the value
// limit and the memory left as it is written, stopping as soon as it passes
// them; every value writes at least a byte, so that bounds the work too.
// Strings take yaml.v3's own rendering, worked out once per distinct string.

// maxEmitDepth bounds nesting, as conversion.ToGoChecked does.
const maxEmitDepth = 10000

// maxSimpleKey is the longest key yaml.v3 writes as a simple key; a longer
// or multi-line key needs the complex key form, which this emitter leaves to
// yaml.Marshal.
const maxSimpleKey = 128

type emitState int

const (
	stateRoot emitState = iota
	stateSeqItem
	stateMapValue
)

type yamlEmitter struct {
	buf         bytes.Buffer
	strings     map[string]string // yaml.v3 renderings of strings
	seen        map[object.Object]struct{}
	needMarshal bool // a key or string needs yaml.Marshal's layout: give up
	err         object.Object
}

func newYAMLEmitter() *yamlEmitter {
	return &yamlEmitter{seen: make(map[object.Object]struct{})}
}

// emitDocument writes obj as a YAML document. It returns false when the
// value needs yaml.Marshal (a complex key) or the limits stopped it (err).
func (e *yamlEmitter) emitDocument(obj object.Object) bool {
	if s, ok := e.leafString(obj); ok {
		// A root scalar string is exactly what yaml.Marshal writes.
		r, errObj := e.renderString(s)
		if errObj != nil {
			e.err = errObj
			return false
		}
		e.buf.WriteString(r)
		return e.check()
	}
	if !e.node(obj, -1, stateRoot, 0) {
		return false
	}
	if e.buf.Len() == 0 || e.buf.Bytes()[e.buf.Len()-1] != '\n' {
		e.buf.WriteByte('\n')
	}
	return e.check()
}

// check holds the text written so far to the limits.
func (e *yamlEmitter) check() bool {
	if e.err != nil || e.needMarshal {
		return false
	}
	if errObj := object.CheckAllocFor("YAML", int64(e.buf.Len())); errObj != nil {
		e.err = errObj
		return false
	}
	return true
}

// checkGrowth checks the text will stay within the limits after n more
// bytes, before writing indentation that grows with nesting depth.
func (e *yamlEmitter) checkGrowth(n int) bool {
	if errObj := object.CheckAllocFor("YAML", int64(e.buf.Len())+int64(n)); errObj != nil {
		e.err = errObj
		return false
	}
	return true
}

// increaseIndent is yaml.v3's indentation rule: a block nested in a sequence
// item starts just past the "- ", anything else at the next multiple of four.
func increaseIndent(indent int, state emitState) int {
	switch {
	case indent < 0:
		return 0
	case state == stateSeqItem:
		return indent + 2
	default:
		return 4 * ((indent + 4) / 4)
	}
}

// column is the column the text ends at.
func (e *yamlEmitter) column() int {
	b := e.buf.Bytes()
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		return len(b) - i - 1
	}
	return len(b)
}

// writeIndent starts an entry at indent: on the current line when it is
// the first entry of a block nested in a sequence item (just after its
// "-"), else on a new line.
func (e *yamlEmitter) writeIndent(indent int, sameLine bool) bool {
	if !e.checkGrowth(indent + 1) {
		return false
	}
	col := e.column()
	if !sameLine && col > 0 {
		e.buf.WriteByte('\n')
		col = 0
	}
	for ; col < indent; col++ {
		e.buf.WriteByte(' ')
	}
	return true
}

// leafString returns the string a non-container value converts to, as
// conversion.ToGo renders it, when it converts to a string.
func (e *yamlEmitter) leafString(obj object.Object) (string, bool) {
	switch o := obj.(type) {
	case *object.String:
		return o.StringValue(), true
	case *object.Error:
		return o.Message, true
	case *object.Builtin:
		if o.HelpText != "" {
			return o.HelpText, true
		}
		return "<builtin function>", true
	case *object.Function:
		return o.Name, true
	case *object.Null, *object.Boolean, *object.Integer, *object.Float,
		*object.Bytes, *object.List, *object.Tuple, *object.Dict, *object.FloatArray:
		return "", false
	}
	return obj.Inspect(), true
}

// scalar returns the plain rendering of a non-string scalar.
func scalarText(obj object.Object) (string, bool) {
	switch o := obj.(type) {
	case *object.Null:
		return "null", true
	case *object.Boolean:
		if o.BoolValue() {
			return "true", true
		}
		return "false", true
	case *object.Integer:
		return strconv.FormatInt(o.IntValue(), 10), true
	case *object.Float:
		return floatText(o.FloatValue()), true
	}
	return "", false
}

func floatText(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	switch s {
	case "+Inf":
		return ".inf"
	case "-Inf":
		return "-.inf"
	case "NaN":
		return ".nan"
	}
	return s
}

// node writes obj, a value in the given state under a parent at indent.
// The text so far ends where the value starts: after "- ", after "key:", or
// at the start of the document.
func (e *yamlEmitter) node(obj object.Object, indent int, state emitState, depth int) bool {
	depth++
	if depth > maxEmitDepth+1 { // as toGo: the root is depth 0 there
		e.err = &object.Error{
			Message:       "value too deeply nested to serialize (more than 10000 levels)",
			ExceptionType: "RecursionError",
		}
		return false
	}
	if text, ok := scalarText(obj); ok {
		e.inlineScalar(text, state)
		return e.check()
	}
	switch o := obj.(type) {
	case *object.List:
		return e.sequence(o, len(o.Elements), func(i int) object.Object { return o.Elements[i] }, indent, state, depth)
	case *object.Tuple:
		return e.sequence(o, len(o.Elements), func(i int) object.Object { return o.Elements[i] }, indent, state, depth)
	case *object.Bytes:
		// yaml.v3 writes a []byte as a sequence of numbers.
		data := o.BytesValue()
		return e.sequence(nil, len(data), func(i int) object.Object { return object.NewInteger(int64(data[i])) }, indent, state, depth)
	case *object.FloatArray:
		if o.Is2D() {
			rows, cols := o.Rows(), o.Cols()
			return e.sequence(nil, rows, func(i int) object.Object {
				return &object.FloatArray{Data: o.Data[i*cols : (i+1)*cols], Shape: []int{cols}}
			}, indent, state, depth)
		}
		return e.sequence(nil, len(o.Data), func(i int) object.Object { return object.NewFloat(o.Data[i]) }, indent, state, depth)
	case *object.Dict:
		return e.mapping(o, indent, state, depth)
	}
	s, _ := e.leafString(obj)
	return e.stringValue(s, indent, state)
}

// inlineScalar writes a one-line scalar after its "- " or "key:".
func (e *yamlEmitter) inlineScalar(text string, state emitState) {
	if state != stateRoot {
		e.buf.WriteByte(' ')
	}
	e.buf.WriteString(text)
}

const cyclicPlaceholder = "<cyclic reference>"

func (e *yamlEmitter) sequence(container object.Object, n int, at func(int) object.Object, indent int, state emitState, depth int) bool {
	if container != nil {
		if _, cyclic := e.seen[container]; cyclic {
			return e.stringValue(cyclicPlaceholder, indent, state)
		}
		e.seen[container] = struct{}{}
		defer delete(e.seen, container)
	}
	if n == 0 {
		e.inlineScalar("[]", state)
		return e.check()
	}
	inner := increaseIndent(indent, state)
	for i := 0; i < n; i++ {
		// The first item of a sequence that is itself an item starts on the
		// parent's line, just after its "- ".
		if !e.writeIndent(inner, i == 0 && state == stateSeqItem) {
			return false
		}
		e.buf.WriteByte('-')
		if !e.node(at(i), inner, stateSeqItem, depth) {
			return false
		}
		if !e.check() {
			return false
		}
	}
	return true
}

func (e *yamlEmitter) mapping(d *object.Dict, indent int, state emitState, depth int) bool {
	if _, cyclic := e.seen[d]; cyclic {
		return e.stringValue(cyclicPlaceholder, indent, state)
	}
	e.seen[d] = struct{}{}
	defer delete(e.seen, d)
	// conversion.ToGo makes a map keyed by each pair's string key, so a
	// later pair with the same key replaces an earlier one; yaml.v3 then
	// writes the keys in its own order.
	type entry struct {
		key   string
		value object.Object
	}
	index := make(map[string]int, len(d.Pairs))
	entries := make([]entry, 0, len(d.Pairs))
	for _, pair := range d.OrderedPairs() {
		key := pair.StringKey()
		if i, dup := index[key]; dup {
			entries[i].value = pair.Value
			continue
		}
		index[key] = len(entries)
		entries = append(entries, entry{key, pair.Value})
	}
	if len(entries) == 0 {
		e.inlineScalar("{}", state)
		return e.check()
	}
	sort.SliceStable(entries, func(i, j int) bool { return keyLess(entries[i].key, entries[j].key) })
	inner := increaseIndent(indent, state)
	for i, ent := range entries {
		if len(ent.key) > maxSimpleKey || strings.ContainsAny(ent.key, "\n\r\u0085  ") {
			e.needMarshal = true
			return false
		}
		key, errObj := e.renderString(ent.key)
		if errObj != nil {
			e.err = errObj
			return false
		}
		key = strings.TrimSuffix(key, "\n")
		if strings.Contains(key, "\n") {
			e.needMarshal = true
			return false
		}
		if !e.writeIndent(inner, i == 0 && state == stateSeqItem) {
			return false
		}
		e.buf.WriteString(key)
		e.buf.WriteByte(':')
		if !e.node(ent.value, inner, stateMapValue, depth) {
			return false
		}
		if !e.check() {
			return false
		}
	}
	return true
}

// stringValue writes a string in a sequence item or mapping value.
func (e *yamlEmitter) stringValue(s string, indent int, state emitState) bool {
	r, errObj := e.renderString(s)
	if errObj != nil {
		e.err = errObj
		return false
	}
	r = strings.TrimSuffix(r, "\n")
	if strings.ContainsAny(r, "\r\u0085\u2028\u2029") {
		// yaml.v3 indents what follows these line breaks to the depth the
		// string is written at: rare enough to leave to yaml.Marshal.
		e.needMarshal = true
		return false
	}
	nl := strings.IndexByte(r, '\n')
	if nl < 0 {
		e.inlineScalar(r, state)
		return e.check()
	}
	// A block scalar: yaml.Marshal wrote it at the top level, its lines
	// indented by four; here they are indented as yaml.v3 indents a scalar
	// nested in this state. Its last line break, if it keeps one, ends the
	// line the way the document's own would.
	full, _ := e.renderString(s) // cached
	header, rest := full[:nl], full[nl+1:]
	e.buf.WriteByte(' ')
	e.buf.WriteString(header)
	e.buf.WriteByte('\n')
	lineIndent := increaseIndent(indent, state)
	for len(rest) > 0 {
		line := rest
		if i := strings.IndexByte(rest, '\n'); i >= 0 {
			line = rest[:i+1]
		}
		rest = rest[len(line):]
		if line != "\n" {
			if !e.checkGrowth(lineIndent + len(line)) {
				return false
			}
			for range lineIndent {
				e.buf.WriteByte(' ')
			}
			line = strings.TrimPrefix(line, "    ")
		}
		e.buf.WriteString(line)
	}
	return e.check()
}

// renderString returns yaml.Marshal's rendering of s as a document, cached
// so a string referenced many times is rendered once. Strings yaml.v3
// writes plain are recognised without calling it.
func (e *yamlEmitter) renderString(s string) (string, object.Object) {
	if plainSafe(s) {
		return s + "\n", nil
	}
	if r, ok := e.strings[s]; ok {
		return r, nil
	}
	if errObj := object.CheckAllocFor("YAML", int64(len(s))*4); errObj != nil {
		return "", errObj
	}
	out, err := yaml.Marshal(s)
	if err != nil {
		return "", &object.Error{Message: "yaml dump error: " + err.Error()}
	}
	if e.strings == nil {
		e.strings = make(map[string]string)
	}
	r := string(out)
	e.strings[s] = r
	return r, nil
}

// plainSafe reports whether yaml.v3 certainly writes s plain, as is: words
// of letters, digits and underscores starting with a letter, separated by
// single spaces, that no YAML resolver reads as anything but a string.
func plainSafe(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	c := s[0]
	if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
		case c == ' ' && s[i-1] != ' ' && i < len(s)-1:
		default:
			return false
		}
	}
	switch strings.ToLower(s) {
	case "y", "yes", "n", "no", "on", "off", "true", "false", "null":
		return false
	}
	return true
}

// keyLess is yaml.v3's order for string map keys: letters before digits
// after a common digit run, numbers by value.
func keyLess(a, b string) bool {
	ar, br := []rune(a), []rune(b)
	digits := false
	for i := 0; i < len(ar) && i < len(br); i++ {
		if ar[i] == br[i] {
			digits = unicode.IsDigit(ar[i])
			continue
		}
		al := unicode.IsLetter(ar[i])
		bl := unicode.IsLetter(br[i])
		if al && bl {
			return ar[i] < br[i]
		}
		if al || bl {
			if digits {
				return al
			}
			return bl
		}
		var ai, bi int
		var an, bn int64
		if ar[i] == '0' || br[i] == '0' {
			for j := i - 1; j >= 0 && unicode.IsDigit(ar[j]); j-- {
				if ar[j] != '0' {
					an = 1
					bn = 1
					break
				}
			}
		}
		for ai = i; ai < len(ar) && unicode.IsDigit(ar[ai]); ai++ {
			an = an*10 + int64(ar[ai]-'0')
		}
		for bi = i; bi < len(br) && unicode.IsDigit(br[bi]); bi++ {
			bn = bn*10 + int64(br[bi]-'0')
		}
		if an != bn {
			return an < bn
		}
		if ai != bi {
			return ai < bi
		}
		return ar[i] < br[i]
	}
	return len(ar) < len(br)
}
