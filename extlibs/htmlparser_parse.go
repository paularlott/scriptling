package extlibs

import (
	"context"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/paularlott/scriptling/object"
)

// htmlParse is one feed() of an HTMLParser. It follows CPython 3.14's
// html.parser (HTMLParser.goahead and the parse_* methods it calls) for a
// complete chunk, so malformed markup produces the same callbacks Python
// does. The regular expressions CPython uses are written out as scanners;
// where they backtrack, the scanner says so.
//
// Not covered: CPython buffers a construct left incomplete at the end of a
// feed() until the next feed() or close(), and treats the content of script,
// style and the other raw-text elements as text. Here each feed() is parsed
// as if close() followed it.
type htmlParse struct {
	ctx      context.Context
	instance *object.Instance
	data     string
	convert  bool // convert_charrefs
}

// parseHTML parses one feed() of data. A handler's error or raised exception
// stops parsing and is returned, as Python propagates it out of feed().
func parseHTML(ctx context.Context, instance *object.Instance, data string) object.Object {
	convert := true
	if val, ok := instance.GetField("convert_charrefs"); ok {
		if b, ok := val.(*object.Boolean); ok {
			convert = b.BoolValue()
		}
	}
	p := htmlParse{ctx: ctx, instance: instance, data: data, convert: convert}
	if err := p.goahead(); err != nil {
		return err
	}
	return &object.Null{}
}

// handle calls a handler and returns its result only when it must propagate.
func (p *htmlParse) handle(method string, args ...string) object.Object {
	objs := make([]object.Object, len(args))
	for i, a := range args {
		objs[i] = object.NewString(a)
	}
	if r := callHandler(p.ctx, p.instance, method, objs...); object.IsPropagating(r) {
		return r
	}
	return nil
}

func (p *htmlParse) text(s string) object.Object {
	if p.convert {
		s = pyUnescape(s)
	}
	return p.handle("handle_data", s)
}

// goahead is HTMLParser.goahead(end=True).
func (p *htmlParse) goahead() object.Object {
	data := p.data
	n := len(data)
	i := 0
	for i < n {
		var j int
		if p.convert {
			j = strings.IndexByte(data[i:], '<')
		} else {
			j = strings.IndexAny(data[i:], "<&")
		}
		if j < 0 {
			j = n
		} else {
			j += i
		}
		if i < j {
			if err := p.text(data[i:j]); err != nil {
				return err
			}
		}
		i = j
		if i == n {
			break
		}

		if data[i] == '<' {
			var k int
			var err object.Object
			switch rest := data[i:]; {
			case len(rest) > 1 && isASCIILetter(rest[1]):
				k, err = p.parseStarttag(i)
			case strings.HasPrefix(rest, "</"):
				k, err = p.parseEndtag(i)
			case strings.HasPrefix(rest, "<!--"):
				k, err = p.parseComment(i)
			case strings.HasPrefix(rest, "<?"):
				k, err = p.parsePI(i)
			case strings.HasPrefix(rest, "<!"):
				k, err = p.parseHTMLDeclaration(i)
			default:
				err = p.handle("handle_data", "<")
				k = i + 1
			}
			if err == nil && k < 0 {
				k, err = p.incomplete(i)
			}
			if err != nil {
				return err
			}
			i = k
			continue
		}

		if strings.HasPrefix(data[i:], "&#") {
			if name, k, ok := matchCharref(data, i); ok {
				if err := p.handle("handle_charref", name); err != nil {
					return err
				}
				i = k
				continue
			}
			if i+2 < n && (isASCIIDigit(data[i+2]) || (data[i+2]|0x20 == 'x' && i+3 < n && isHexDigit(data[i+3]))) {
				if err := p.handle("handle_charref", data[i+2:]); err != nil {
					return err
				}
				return nil
			}
			// CPython's "i + 3 < n" counts characters: two must follow "&#".
			if utf8.RuneCountInString(data[i+2:min(n, i+2+2*utf8.UTFMax)]) < 2 {
				break
			}
			if err := p.handle("handle_data", "&#"); err != nil {
				return err
			}
			i += 2
			continue
		}

		// data[i] == '&'
		if name, k, ok := matchEntityref(data, i); ok {
			if err := p.handle("handle_entityref", name); err != nil {
				return err
			}
			i = k
			continue
		}
		if i+1 < n && isASCIILetter(data[i+1]) {
			if err := p.handle("handle_entityref", data[i+1:]); err != nil {
				return err
			}
			return nil
		}
		if i+1 >= n {
			break
		}
		if err := p.handle("handle_data", "&"); err != nil {
			return err
		}
		i++
	}
	if i < n {
		return p.text(data[i:])
	}
	return nil
}

// incomplete handles a construct at i that runs to the end of the data
// without being closed (the k < 0 branch of goahead with end=True). It
// returns the end of the data.
func (p *htmlParse) incomplete(i int) (int, object.Object) {
	data := p.data
	n := len(data)
	rest := data[i:]
	var err object.Object
	switch {
	case len(rest) > 1 && isASCIILetter(rest[1]):
		// An unclosed start tag is dropped.
	case strings.HasPrefix(rest, "</"):
		if i+2 == n {
			err = p.handle("handle_data", "</")
		} else if !isASCIILetter(data[i+2]) {
			err = p.handle("handle_comment", data[i+2:])
		}
	case strings.HasPrefix(rest, "<!--"):
		body := data[i+4:]
		for _, suffix := range []string{"--!", "--", "-"} {
			if strings.HasSuffix(body, suffix) {
				body = body[:len(body)-len(suffix)]
				break
			}
		}
		err = p.handle("handle_comment", body)
	case strings.HasPrefix(rest, "<![CDATA["):
		err = p.handle("unknown_decl", data[i+3:])
	case hasDoctypePrefix(rest):
		err = p.handle("handle_decl", data[i+2:])
	case strings.HasPrefix(rest, "<!"):
		err = p.handle("handle_comment", data[i+2:])
	case strings.HasPrefix(rest, "<?"):
		err = p.handle("handle_pi", data[i+2:])
	}
	return n, err
}

// parseHTMLDeclaration handles "<!" other than a comment.
func (p *htmlParse) parseHTMLDeclaration(i int) (int, object.Object) {
	data := p.data
	rest := data[i:]
	switch {
	case strings.HasPrefix(rest, "<![CDATA["):
		j := strings.Index(data[i+9:], "]]>")
		if j < 0 {
			return -1, nil
		}
		j += i + 9
		return j + 3, p.handle("unknown_decl", data[i+3:j])
	case hasDoctypePrefix(rest):
		gt := strings.IndexByte(data[i+9:], '>')
		if gt < 0 {
			return -1, nil
		}
		gt += i + 9
		return gt + 1, p.handle("handle_decl", data[i+2:gt])
	}
	return p.parseBogusComment(i)
}

// parseComment handles "<!--": closed by "-->" or "--!>", or abruptly by ">"
// or "->" straight after the opener.
func (p *htmlParse) parseComment(i int) (int, object.Object) {
	data := p.data
	n := len(data)
	start, end := -1, -1
	for s := i + 4; ; {
		idx := strings.Index(data[s:], "--")
		if idx < 0 {
			break
		}
		m := s + idx
		if m+2 < n && data[m+2] == '>' {
			start, end = m, m+3
			break
		}
		if m+3 < n && data[m+2] == '!' && data[m+3] == '>' {
			start, end = m, m+4
			break
		}
		s = m + 1
	}
	if start < 0 {
		switch {
		case i+4 < n && data[i+4] == '>':
			start, end = i+4, i+5
		case i+5 < n && data[i+4] == '-' && data[i+5] == '>':
			start, end = i+4, i+6
		default:
			return -1, nil
		}
	}
	return end, p.handle("handle_comment", data[i+4:start])
}

// parseBogusComment handles "<!..." and "</..." that are not markup: the text
// up to the next ">" is a comment.
func (p *htmlParse) parseBogusComment(i int) (int, object.Object) {
	pos := strings.IndexByte(p.data[i+2:], '>')
	if pos < 0 {
		return -1, nil
	}
	pos += i + 2
	return pos + 1, p.handle("handle_comment", p.data[i+2:pos])
}

// parsePI handles "<?": a processing instruction ends at the first ">".
func (p *htmlParse) parsePI(i int) (int, object.Object) {
	j := strings.IndexByte(p.data[i+2:], '>')
	if j < 0 {
		return -1, nil
	}
	j += i + 2
	return j + 1, p.handle("handle_pi", p.data[i+2:j])
}

// parseEndtag handles "</".
func (p *htmlParse) parseEndtag(i int) (int, object.Object) {
	data := p.data
	if strings.IndexByte(data[i+2:], '>') < 0 {
		return -1, nil
	}
	if !isASCIILetter(data[i+2]) {
		if data[i+2] == '>' { // "</>" is ignored
			return i + 3, nil
		}
		return p.parseBogusComment(i)
	}
	j := locateTagEnd(data, i+2)
	if data[j-1] != '>' {
		return -1, nil
	}
	name, _ := tagFind(data, i+2)
	return j, p.handle("handle_endtag", strings.ToLower(name))
}

// parseStarttag handles "<" followed by a letter.
func (p *htmlParse) parseStarttag(i int) (int, object.Object) {
	data := p.data
	p.instance.SetField("_lasttag", &object.Null{})
	endpos := locateTagEnd(data, i+1)
	if data[endpos-1] != '>' {
		return -1, nil
	}
	p.instance.SetField("_lasttag", object.NewString(data[i:endpos]))

	name, k := tagFind(data, i+1)
	tag := strings.ToLower(name)
	var attrs []object.Object
	for k < endpos {
		attrName, value, hasValue, end, ok := attrFind(data, k)
		if !ok {
			break
		}
		var v object.Object = &object.Null{}
		if hasValue {
			if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
				value = value[1 : len(value)-1]
			}
			if value != "" {
				value = unescapeAttrValue(value)
			}
			v = object.NewString(value)
		}
		attrs = append(attrs, &object.Tuple{Elements: []object.Object{object.NewString(strings.ToLower(attrName)), v}})
		k = end
	}

	closing := ""
	if k < endpos {
		closing = strings.TrimFunc(data[k:endpos], pyIsSpace)
	}
	if closing != ">" && closing != "/>" {
		return endpos, p.handle("handle_data", data[i:endpos])
	}
	attrList := &object.List{Elements: attrs}
	if attrList.Elements == nil {
		attrList.Elements = []object.Object{}
	}
	object.TrackSlice(attrList.Elements)
	method := "handle_starttag"
	if closing == "/>" {
		method = "handle_startendtag"
	}
	r := callHandler(p.ctx, p.instance, method, object.NewString(tag), attrList)
	if object.IsPropagating(r) {
		return endpos, r
	}
	return endpos, nil
}

// isTagSpace is the whitespace HTML tag syntax recognises: [\t\n\r\f ].
func isTagSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func isASCIILetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }
func isASCIIDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isHexDigit(c byte) bool    { return isASCIIDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'f') }
func isASCIIAlnum(c byte) bool  { return isASCIILetter(c) || isASCIIDigit(c) }

// pyIsSpace is Python's str.isspace for one character.
func pyIsSpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

// hasDoctypePrefix reports whether s starts with "<!doctype" in any case.
func hasDoctypePrefix(s string) bool {
	return len(s) >= 9 && strings.EqualFold(s[:9], "<!doctype")
}

// tagFind is CPython's tagfind_tolerant at p (a letter): the tag name, and
// the position after the whitespace and lone slashes that follow it.
func tagFind(data string, p int) (string, int) {
	n := len(data)
	j := p + 1
	for j < n && !isTagSpace(data[j]) && data[j] != '/' && data[j] != '>' {
		j++
	}
	name := data[p:j]
	for j < n && (isTagSpace(data[j]) || (data[j] == '/' && !(j+1 < n && data[j+1] == '>'))) {
		j++
	}
	return name, j
}

// attrValue matches an attribute's value indicator and value at j:
// [\t\n\r\f ]*=[\t\n\r\f ]* followed by a quoted or bare value. ok is false
// when there is none.
func attrValue(data string, j int) (value string, end int, ok bool) {
	n := len(data)
	k := j
	for k < n && isTagSpace(data[k]) {
		k++
	}
	if k >= n || data[k] != '=' {
		return "", 0, false
	}
	k++
	afterEq := k
	for k < n && isTagSpace(data[k]) {
		k++
	}
	if k < n && (data[k] == '\'' || data[k] == '"') {
		if e := strings.IndexByte(data[k+1:], data[k]); e >= 0 {
			return data[k : k+e+2], k + e + 2, true
		}
		// An unclosed quote matches no value. The regex then backtracks
		// into the whitespace before it, where an empty bare value matches.
		if k > afterEq {
			return "", k - 1, true
		}
		return "", 0, false
	}
	e := k
	for e < n && data[e] != '>' && !isTagSpace(data[e]) {
		e++
	}
	return data[k:e], e, true
}

// attrNameAt reports whether an attribute name starts at j: it must follow a
// quote, whitespace or slash, and not itself be one of [\t\n\r\f />].
func attrNameAt(data string, j int) bool {
	if j >= len(data) {
		return false
	}
	switch data[j-1] {
	case '\'', '"', '\t', '\n', '\r', '\f', ' ', '/':
	default:
		return false
	}
	c := data[j]
	return !isTagSpace(c) && c != '/' && c != '>'
}

// attrNameEnd returns the end of the attribute name starting at j.
func attrNameEnd(data string, j int) int {
	n := len(data)
	j++
	for j < n && !isTagSpace(data[j]) && data[j] != '/' && data[j] != '=' && data[j] != '>' {
		j++
	}
	return j
}

// attrFind is CPython's attrfind_tolerant at k.
func attrFind(data string, k int) (name, value string, hasValue bool, end int, ok bool) {
	if !attrNameAt(data, k) {
		return "", "", false, 0, false
	}
	j := attrNameEnd(data, k)
	name = data[k:j]
	if v, e, found := attrValue(data, j); found {
		value, hasValue, j = v, true, e
	}
	n := len(data)
	for j < n && (isTagSpace(data[j]) || (data[j] == '/' && !(j+1 < n && data[j+1] == '>'))) {
		j++
	}
	return name, value, hasValue, j, true
}

// locateTagEnd is CPython's locatetagend at p (the letter after "<" or
// "</"). The tag is complete when data[end-1] == '>'.
func locateTagEnd(data string, p int) int {
	n := len(data)
	j := p + 1
	for j < n && !isTagSpace(data[j]) && data[j] != '/' && data[j] != '>' {
		j++
	}
	for j < n && (isTagSpace(data[j]) || data[j] == '/') {
		j++
	}
	for attrNameAt(data, j) {
		j = attrNameEnd(data, j)
		if _, e, found := attrValue(data, j); found {
			j = e
		}
		for j < n && (isTagSpace(data[j]) || data[j] == '/') {
			j++
		}
	}
	if j < n && data[j] == '>' {
		j++
	}
	return j
}

// matchCharref is CPython's charref regex at i ("&#"): a decimal or hex
// reference followed by a character that is not a hex digit. It returns the
// name passed to handle_charref and where parsing resumes (after a ";", or at
// any other terminator).
func matchCharref(data string, i int) (string, int, bool) {
	n := len(data)
	j := i + 2
	e := j
	switch {
	case j < n && isASCIIDigit(data[j]):
		for e < n && isASCIIDigit(data[e]) {
			e++
		}
	case j < n && data[j]|0x20 == 'x':
		e = j + 1
		for e < n && isHexDigit(data[e]) {
			e++
		}
		if e == j+1 {
			return "", 0, false
		}
	default:
		return "", 0, false
	}
	if e >= n || isHexDigit(data[e]) {
		return "", 0, false
	}
	if data[e] == ';' {
		return data[j:e], e + 1, true
	}
	return data[j:e], e, true
}

// matchEntityref is CPython's entityref regex at i ("&"): a name followed by
// a character that is not alphanumeric.
func matchEntityref(data string, i int) (string, int, bool) {
	n := len(data)
	if i+1 >= n || !isASCIILetter(data[i+1]) {
		return "", 0, false
	}
	e := i + 2
	for e < n && (isASCIIAlnum(data[e]) || data[e] == '-' || data[e] == '.') {
		e++
	}
	if e == n {
		// No terminator after the whole name: the regex backtracks to the
		// last "-" or "." inside it, which then terminates a shorter name.
		for e = n - 1; e > i+1 && data[e] != '-' && data[e] != '.'; e-- {
		}
		if e == i+1 {
			return "", 0, false
		}
	}
	if data[e] == ';' {
		return data[i+1 : e], e + 1, true
	}
	return data[i+1 : e], e, true
}

// pyInvalidCharrefs maps the numeric references HTML5 remaps (code points,
// so U+0081 is "\u0081": "\x81" would be a lone byte, not UTF-8).
var pyInvalidCharrefs = map[uint64]string{
	0x00: "\ufffd", 0x0d: "\r", 0x80: "\u20ac", 0x81: "\u0081", 0x82: "\u201a",
	0x83: "\u0192", 0x84: "\u201e", 0x85: "\u2026", 0x86: "\u2020", 0x87: "\u2021",
	0x88: "\u02c6", 0x89: "\u2030", 0x8a: "\u0160", 0x8b: "\u2039", 0x8c: "\u0152",
	0x8d: "\u008d", 0x8e: "\u017d", 0x8f: "\u008f", 0x90: "\u0090", 0x91: "\u2018",
	0x92: "\u2019", 0x93: "\u201c", 0x94: "\u201d", 0x95: "\u2022", 0x96: "\u2013",
	0x97: "\u2014", 0x98: "\u02dc", 0x99: "\u2122", 0x9a: "\u0161", 0x9b: "\u203a",
	0x9c: "\u0153", 0x9d: "\u009d", 0x9e: "\u017e", 0x9f: "\u0178",
}

// pyInvalidCodepoint reports the code points a numeric reference may not
// produce (they unescape to nothing).
func pyInvalidCodepoint(c uint64) bool {
	switch {
	case c >= 0x1 && c <= 0x8, c == 0xb, c >= 0xe && c <= 0x1f, c >= 0x7f && c <= 0x9f,
		c >= 0xfdd0 && c <= 0xfdef:
		return true
	}
	return c&0xfffe == 0xfffe && c <= 0x10ffff
}

// numericCharref is the text of numeric reference digits (base 10 or 16).
func numericCharref(digits string, base int) string {
	num, err := strconv.ParseUint(digits, base, 64)
	if err != nil {
		num = 0x110000 // too large for any code point
	}
	if s, ok := pyInvalidCharrefs[num]; ok {
		return s
	}
	if (num >= 0xd800 && num <= 0xdfff) || num > 0x10ffff {
		return "\ufffd"
	}
	if pyInvalidCodepoint(num) {
		return ""
	}
	return string(rune(num))
}

// pyUnescape is Python's html.unescape: numeric and named character
// references by the HTML5 rules, including named references without their
// semicolon and the longest-prefix match for unknown names.
// html5Entity returns the replacement for an HTML5 named character reference
// (with or without its semicolon, as Python's html.entities.html5 keys them)
// by binary search over the generated, bytewise-sorted name table in
// htmlparser_entities.go.
func html5Entity(name string) (string, bool) {
	lo, hi := 0, len(html5EntityNameEnds)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if html5EntityName(mid) < name {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == len(html5EntityNameEnds) || html5EntityName(lo) != name {
		return "", false
	}
	start := 0
	if lo > 0 {
		start = int(html5EntityValueEnds[lo-1])
	}
	return html5EntityValues[start:html5EntityValueEnds[lo]], true
}

// html5EntityName is entry i's name.
func html5EntityName(i int) string {
	start := 0
	if i > 0 {
		start = int(html5EntityNameEnds[i-1])
	}
	return html5EntityNames[start:html5EntityNameEnds[i]]
}

func pyUnescape(s string) string {
	amp := strings.IndexByte(s, '&')
	if amp < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	n := len(s)
	i := 0
	for amp >= 0 {
		a := i + amp
		b.WriteString(s[i:a])
		repl, end, ok := unescapeAt(s, a)
		if !ok {
			b.WriteByte('&')
			i = a + 1
		} else {
			b.WriteString(repl)
			i = end
		}
		if i >= n {
			break
		}
		amp = strings.IndexByte(s[i:], '&')
	}
	if i < n {
		b.WriteString(s[i:])
	}
	return b.String()
}

// unescapeAt replaces the character reference at s[a] == '&', if any.
func unescapeAt(s string, a int) (string, int, bool) {
	n := len(s)
	j := a + 1
	if j < n && s[j] == '#' {
		e := j + 1
		base := 10
		digits := e
		if e < n && s[e]|0x20 == 'x' {
			base, digits = 16, e+1
			e = digits
			for e < n && isHexDigit(s[e]) {
				e++
			}
		} else {
			for e < n && isASCIIDigit(s[e]) {
				e++
			}
		}
		if e == digits {
			return "", 0, false
		}
		repl := numericCharref(s[digits:e], base)
		if e < n && s[e] == ';' {
			e++
		}
		return repl, e, true
	}
	// A name of 1-32 characters other than [\t\n\f <&#;], then an optional ";".
	e := j
	for count := 0; e < n && count < 32; count++ {
		c := s[e]
		if c == '\t' || c == '\n' || c == '\f' || c == ' ' || c == '<' || c == '&' || c == '#' || c == ';' {
			break
		}
		_, size := utf8.DecodeRuneInString(s[e:])
		e += size
	}
	if e == j {
		return "", 0, false
	}
	if e < n && s[e] == ';' {
		e++
	}
	name := s[j:e]
	if v, ok := html5Entity(name); ok {
		return v, e, true
	}
	// The longest known prefix of at least two characters.
	for x := len(name); x > 0; {
		_, size := utf8.DecodeLastRuneInString(name[:x])
		x -= size
		if utf8.RuneCountInString(name[:x]) < 2 {
			break
		}
		if v, ok := html5Entity(name[:x]); ok {
			return v + name[x:], e, true
		}
	}
	return "&" + name, e, true
}

// unescapeAttrValue is CPython's _unescape_attrvalue: numeric references are
// always replaced, named ones only when the whole name is known and not
// followed by "=".
func unescapeAttrValue(s string) string {
	if strings.IndexByte(s, '&') < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	n := len(s)
	i := 0
	for i < n {
		if s[i] != '&' {
			b.WriteByte(s[i])
			i++
			continue
		}
		e := i + 1
		numeric := false
		switch {
		case e+1 < n && s[e] == '#' && isASCIIDigit(s[e+1]):
			numeric = true
			e += 2
			for e < n && isASCIIDigit(s[e]) {
				e++
			}
		case e+2 < n && s[e] == '#' && s[e+1]|0x20 == 'x' && isHexDigit(s[e+2]):
			numeric = true
			e += 3
			for e < n && isHexDigit(s[e]) {
				e++
			}
		case e < n && isASCIILetter(s[e]):
			for e++; e < n && isASCIIAlnum(s[e]); e++ {
			}
		default:
			b.WriteByte('&')
			i++
			continue
		}
		if e < n && (s[e] == ';' || s[e] == '=') {
			e++
		}
		ref := s[i:e]
		switch {
		case numeric:
			b.WriteString(pyUnescape(ref))
		case ref[len(ref)-1] != '=':
			if _, known := html5Entity(ref[1:]); known {
				b.WriteString(pyUnescape(ref))
				break
			}
			b.WriteString(ref)
		default:
			b.WriteString(ref)
		}
		i = e
	}
	return b.String()
}
